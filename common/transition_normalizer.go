/*
 *   Copyright (c) 2025 Arcology Network

 *   This program is free software: you can redistribute it and/or modify
 *   it under the terms of the GNU General Public License as published by
 *   the Free Software Foundation, either version 3 of the License, or
 *   (at your option) any later version.

 *   This program is distributed in the hope that it will be useful,
 *   but WITHOUT ANY WARRANTY; without even the implied warranty of
 *   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *   GNU General Public License for more details.

 *   You should have received a copy of the GNU General Public License
 *   along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

/*
 *   Copyright (c) 2025 Arcology Network

 *   This program is free software: you can redistribute it and/or modify
 *   it under the terms of the GNU General Public License as published by
 *   the Free Software Foundation, either version 3 of the License, or
 *   (at your option) any later version.

 *   This program is distributed in the hope that it will be useful,
 *   but WITHOUT ANY WARRANTY; without even the implied warranty of
 *   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *   GNU General Public License for more details.

 *   You should have received a copy of the GNU General Public License
 *   along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package common

import (
	"encoding/hex"
	"strings"

	statecommon "github.com/arcology-network/common-lib/crdt/common"
	statecell "github.com/arcology-network/common-lib/crdt/statecell"
	"github.com/arcology-network/common-lib/exp/slice"
	commontypes "github.com/arcology-network/common-lib/types"
	"github.com/holiman/uint256"
)

// TransactionNormalizer generates the mandatory system transitions for a
// transaction—gas debit/credit and nonce increment—and marks them as
// conflict-immune so they always commit regardless of execution outcome.
//
// It extracts the sender/coinbase balance updates associated with gas payment
// and move 3 transitions to the immune list, which are immune to execution failures:
//
//  1. A debit transition on the sender's balance to pay the gas fee.
//  2. A credit transition on the coinbase's balance for the same amount from the sender.
//  3. Nonce increment transition for the sender.
//
// These will be committed regardless of whether the transaction execution succeeds or fails.

type TransactionNormalizer struct {
	gasUsed  uint64
	Coinbase [20]byte
	msgView  *commontypes.MessageView
}

func NewTransactionNormalizer(gasUsed uint64, coinbase [20]byte, msgView *commontypes.MessageView) *TransactionNormalizer {
	return &TransactionNormalizer{
		gasUsed:  gasUsed,
		Coinbase: coinbase,
		msgView:  msgView,
	}
}

// insertGasTransition isolates the gas component of a balance update. If the
// existing transition’s delta already equals the gas fee, it is marked as
// conflict-immune and reused. Otherwise, a new transition is cloned with its
// delta set to the exact gas amount. The returned transition always has
// SkipConflictCheck enabled so it commits unconditionally.
func (this *TransactionNormalizer) insertGasTransition(balanceTransition *statecell.StateCell, gasDelta *uint256.Int, isCredit bool) *statecell.StateCell {
	v, _ := balanceTransition.Value().(statecommon.CRDT).Delta()
	totalDelta := v.(uint256.Int)

	if totalDelta.Cmp(gasDelta) == 0 { // Balance change == gas fee paid.
		balanceTransition.Property.SkipConflictCheck(true) // Won't be affect by conflicts
		return balanceTransition
	}

	// Separate the gas fee from the balance change and generate a new transition for that.
	gasTransition := balanceTransition.Clone().(*statecell.StateCell)
	gasTransition.Value().(statecommon.CRDT).SetDelta(*gasDelta, isCredit) // Set the gas fee.
	// gasTransition.Value().(statecommon.CRDT).SetDeltaSign(isCredit) // Negative for the sender, positive for the coinbase.
	gasTransition.Property.SkipConflictCheck(true)
	return gasTransition
}

func (this *TransactionNormalizer) Normalize(RawStateRecords []*statecell.StateCell) []*statecell.StateCell {
	if len(RawStateRecords) == 0 {
		return RawStateRecords
	}

	// The sender isn't the coinbase.
	this.UnapplyNonceOffset(RawStateRecords)                       // Remove the nonce offset first.
	gasTransitions := this.SeparateGasTransitions(RawStateRecords) // Post-process gas transitions.
	nonceTransitions := this.MarkNonceConflictImmune(RawStateRecords)

	return append(gasTransitions, nonceTransitions...)
}

// SeparateGasTransitions extracts unconditional gas fee transfers(for execution) from from balance transitions.
func (this *TransactionNormalizer) SeparateGasTransitions(RawStateRecords []*statecell.StateCell) []*statecell.StateCell {
	if this.msgView.From == this.Coinbase {
		return nil
	}

	gasTransitions := []*statecell.StateCell{}
	senderString := hex.EncodeToString(this.msgView.From[:])
	_, senderBalance := slice.FindFirstIf(RawStateRecords, func(_ int, v *statecell.StateCell) bool { //It includes the gas fee and possible transfers.
		return v != nil &&
			strings.HasSuffix(*v.GetPath(), "/balance") &&
			strings.Contains(*v.GetPath(), senderString)
	})

	coinbaseString := hex.EncodeToString(this.Coinbase[:])
	_, coinbaseBalance := slice.FindFirstIf(RawStateRecords, func(_ int, v *statecell.StateCell) bool {
		return v != nil &&
			strings.HasSuffix(*v.GetPath(), "/balance") &&
			strings.Contains(*v.GetPath(), coinbaseString)
	})

	// Usually, neither the sender balance nor the coinbase balance can be nil unless the transaction
	// is a L1->L2 transaction derived from a transaction receipt and the network is in a L2 setup.
	if senderBalance != nil && coinbaseBalance != nil {
		// Separate the gas fee from the balance change and generate a new transition for that. It will be immune to the execution status.
		gasPrice := &uint256.Int{}
		gasPrice.SetFromBig(this.msgView.GasPrice)
		gasUsedInWei := new(uint256.Int).Mul(uint256.NewInt(this.gasUsed), gasPrice)
		if debit := this.insertGasTransition(*senderBalance, gasUsedInWei, false); debit != nil {
			gasTransitions = append(gasTransitions, debit)
		}

		if credit := this.insertGasTransition(*coinbaseBalance, gasUsedInWei, true); credit != nil {
			gasTransitions = append(gasTransitions, credit)
		}
	}
	return gasTransitions
}

// MarkNonceConflictImmune locates the nonce update for the transaction sender and marks it as
// conflict-immune. A sender's nonce must always be incremented and committed regardless
// of whether the transaction succeeds or reverts.
//
// In Ethereum semantics, nonce incrementation is unconditional once a transaction enters
// the execution pipeline. To preserve this behavior under Arcology's optimistic
// concurrency control, the nonce transition is flagged with SkipConflictCheck = true so
// that it bypasses conflict validation and is always included in the final commit set.
//
// If the sender's nonce update is not present in RawStateRecords (e.g., non-standard
// system transactions or partial receipts), this function returns an empty slice.
func (this *TransactionNormalizer) MarkNonceConflictImmune(RawStateRecords []*statecell.StateCell) []*statecell.StateCell {
	nonceTransitions := []*statecell.StateCell{}
	_, senderNonce := slice.FindFirstIf(RawStateRecords, func(_ int, v *statecell.StateCell) bool {
		return strings.Contains(*v.GetPath(), "/nonce") &&
			strings.Contains(*v.GetPath(), hex.EncodeToString(this.msgView.From[:]))
	})

	if senderNonce != nil {
		(*senderNonce).Property.SkipConflictCheck(true)           // Won't be affect by conflicts either
		nonceTransitions = append(nonceTransitions, *senderNonce) // Add the nonce transition to the immune list even if the execution is unsuccessful.
	}
	return nonceTransitions
}

// When processing multiple transactions from the same sender in a single generation,all the parallel transactions
// are executed based on the same initial state, so they all see the same nonce value for the sender. To prevent
// nonce conflicts, we need to add an offset to the nonce for each transaction based on its order in the batch.
//
// The offsets need to be removed before committing the transitions to the state store, otherwise the nonce values
// will be incorrect.
func (*TransactionNormalizer) UnapplyNonceOffset(RawStateRecords []*statecell.StateCell) {
	for _, record := range RawStateRecords {
		if strings.HasSuffix(*record.GetPath(), "/nonce") {
			nonceDelta, _ := record.Value().(statecommon.CRDT).Delta() // Get the total nonce delta
			if nonceDelta.(uint64) > 1 {
				// Nonce only increases by 1 for each transaction, so if the delta is greater than the reads, it means
				// there is an offset applied. We can reverse the offset by subtracting the offset from the delta.
				negativeOffset := nonceDelta.(uint64) - 1
				record.Value().(statecommon.CRDT).SetDelta(negativeOffset, false) // Remove the offset by applying a negative delta.
			}
		}
	}
}
