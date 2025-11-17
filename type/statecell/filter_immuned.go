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

package statecell

import (
	"encoding/hex"
	"strings"

	"github.com/arcology-network/common-lib/exp/slice"
	libcommon "github.com/arcology-network/common-lib/types"
	stgcommon "github.com/arcology-network/storage-committer/common"
	"github.com/holiman/uint256"
)

type TransactionNormalizer struct {
	gasUsed  uint64
	gasPrice uint64
	Coinbase [20]byte
	msg      *libcommon.StandardMessage
}

func NewTransactionNormalizer(gasUsed uint64, gasPrice uint64, coinbase [20]byte, msg *libcommon.StandardMessage) *TransactionNormalizer {
	return &TransactionNormalizer{
		gasUsed:  gasUsed,
		gasPrice: gasPrice,
		Coinbase: coinbase,
		msg:      msg,
	}
}

func (this *TransactionNormalizer) insertGasTransition(balanceTransition *StateCell, gasDelta *uint256.Int, isCredit bool) *StateCell {
	v, _ := balanceTransition.Value().(stgcommon.Type).Delta()
	totalDelta := v.(uint256.Int)

	if totalDelta.Cmp(gasDelta) == 0 { // Balance change == gas fee paid.
		balanceTransition.Property.SkipConflictCheck(true) // Won't be affect by conflicts
		return balanceTransition
	}

	// Separate the gas fee from the balance change and generate a new transition for that.
	gasTransition := balanceTransition.Clone().(*StateCell)
	gasTransition.Value().(stgcommon.Type).SetDelta(*gasDelta, isCredit) // Set the gas fee.
	// gasTransition.Value().(stgcommon.Type).SetDeltaSign(isCredit) // Negative for the sender, positive for the coinbase.
	gasTransition.Property.SkipConflictCheck(true)
	return gasTransition
}

func (this *TransactionNormalizer) Normalize(rawStateAccesses []*StateCell) []*StateCell {
	if len(rawStateAccesses) == 0 {
		return rawStateAccesses
	}

	// The sender isn't the coinbase.
	ImmunedGas := this.NormalizeGas(rawStateAccesses)
	ImmunedNonce := this.NormalizeNonce(rawStateAccesses)

	return append(ImmunedGas, ImmunedNonce...)
}

// NormalizeGas extracts the sender/coinbase balance updates associated with gas payment
// and produces two canonical system transitions:
//
//  1. A debit transition on the sender's balance for gasUsed * gasPrice.
//  2. A credit transition on the coinbase's balance for the same amount.
//
// These transitions do NOT exist in the raw state accesses; they are reconstructed from the
// transaction metadata and derived gas cost. Both generated transitions are marked as
// conflict-immune (SkipConflictCheck = true) because gas payments must always commit
// regardless of whether the transaction succeeds or reverts.
//
// If the sender is the coinbase, or if the corresponding balance transitions cannot be found,
// this function returns an empty slice.
func (this *TransactionNormalizer) NormalizeGas(rawStateAccesses []*StateCell) []*StateCell {
	if this.msg.Native.From == this.Coinbase {
		return nil
	}

	Immuned := []*StateCell{}

	_, senderBalance := slice.FindFirstIf(rawStateAccesses, func(_ int, v *StateCell) bool { //It includes the gas fee and possible transfers.
		return v != nil &&
			strings.HasSuffix(*v.GetPath(), "/balance") &&
			strings.Contains(*v.GetPath(), hex.EncodeToString(this.msg.Native.From[:]))
	})

	_, coinbaseBalance := slice.FindFirstIf(rawStateAccesses, func(_ int, v *StateCell) bool {
		return v != nil &&
			strings.HasSuffix(*v.GetPath(), "/balance") &&
			strings.Contains(*v.GetPath(), hex.EncodeToString(this.Coinbase[:]))
	})

	// Usually, neither the sender balance nor the coinbase balance can't be nil unless the transaction
	// is a L1->L2 transaction derived from a transaction receipt and the network is in a L2 setup.
	if senderBalance != nil && coinbaseBalance != nil {
		// Separate the gas fee from the balance change and generate a new transition for that. It will be immune to the execution status.
		gasUsedInWei := new(uint256.Int).Mul(uint256.NewInt(this.gasUsed), uint256.NewInt(this.gasPrice))
		if debit := this.insertGasTransition(*senderBalance, gasUsedInWei, false); debit != nil {
			Immuned = append(Immuned, debit)
		}

		if credit := this.insertGasTransition(*coinbaseBalance, gasUsedInWei, true); credit != nil {
			Immuned = append(Immuned, credit)
		}
	}
	return Immuned
}

// NormalizeNonce locates the nonce update for the transaction sender and marks it as
// conflict-immune. A sender's nonce must always be incremented and committed regardless
// of whether the transaction succeeds or reverts.
//
// In Ethereum semantics, nonce incrementation is unconditional once a transaction enters
// the execution pipeline. To preserve this behavior under Arcology's optimistic
// concurrency control, the nonce transition is flagged with SkipConflictCheck = true so
// that it bypasses conflict validation and is always included in the final commit set.
//
// If the sender's nonce update is not present in rawStateAccesses (e.g., non-standard
// system transactions or partial receipts), this function returns an empty slice.
func (this *TransactionNormalizer) NormalizeNonce(rawStateAccesses []*StateCell) []*StateCell {
	Immuned := []*StateCell{}
	_, senderNonce := slice.FindFirstIf(rawStateAccesses, func(_ int, v *StateCell) bool {
		return strings.HasSuffix(*v.GetPath(), "/nonce") && strings.Contains(*v.GetPath(), hex.EncodeToString(this.msg.Native.From[:]))
	})

	if senderNonce != nil {
		(*senderNonce).Property.SkipConflictCheck(true) // Won't be affect by conflicts either
		Immuned = append(Immuned, *senderNonce)         // Add the nonce transition to the immune list even if the execution is unsuccessful.
	}
	return Immuned
}
