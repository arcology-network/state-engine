/*
 *   Copyright (c) 2024 Arcology Network

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

package ethstorage

import (
	statecell "github.com/arcology-network/common-lib/crdt/statecell"
	"github.com/arcology-network/common-lib/exp/associative"
	"github.com/arcology-network/common-lib/exp/slice"
	"github.com/arcology-network/common-lib/storage/indexer"
	statecommon "github.com/arcology-network/state-engine/common"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// An index by account address, transitions have the same Eth account address will be put together in a list
// This is for ETH storage, concurrent container related sub-paths won't be put into this index.
type EthIndexer struct {
	filter  func(tran *statecell.StateCell) bool // Post processing function to filter transitions.
	Version int64

	// Transitions indexed by account address. All the transitions belong to the same account are grouped together.
	*indexer.UnorderedIndexer[[20]byte, *statecell.StateCell, *associative.Pair[*Account, []*statecell.StateCell]]
	dirtyAccounts []*Account
	// err           error
}

func NewEthIndexer(store *EthWorldState, Version int64, filter func(tran *statecell.StateCell) bool) *EthIndexer {
	idxer := (indexer.NewUnorderedIndexer(
		nil,
		func(v *statecell.StateCell) ([20]byte, bool) {
			addr, _ := hexutil.Decode(statecommon.GetAccountAddr(*v.GetPath()))
			return ethcommon.BytesToAddress(addr), true
		},

		func(addr [20]byte, v *statecell.StateCell) *associative.Pair[*Account, []*statecell.StateCell] {
			return &associative.Pair[*Account, []*statecell.StateCell]{
				First:  store.Preload(addr[:]).(*Account),
				Second: []*statecell.StateCell{v},
			}
		},

		func(_ [20]byte, v *statecell.StateCell, pair **associative.Pair[*Account, []*statecell.StateCell]) {
			(**pair).Second = append((**pair).Second, v)
		},
	))

	// All passed by default.
	return &EthIndexer{
		filter:           filter,
		Version:          Version,
		UnorderedIndexer: idxer,
	}
}

func (this *EthIndexer) Import(trans []*statecell.StateCell) {
	ethTrans := slice.CopyIf(trans, func(_ int, v *statecell.StateCell) bool {
		return v.GetPath() != nil && this.filter(v) // None nil Eth Storage paths only.
	})
	this.UnorderedIndexer.Import(ethTrans)
}

// Remove the nil transitions from the index, because they are set by
// the finalizer.
func (this *EthIndexer) Finalize() {
	this.ParallelForeachDo(func(_ [20]byte, v **associative.Pair[*Account, []*statecell.StateCell]) {
		slice.RemoveIf(&((**v).Second), func(_ int, v *statecell.StateCell) bool { return v.GetPath() == nil })
	})

	// Remove accounts that have no transitions left after cleanning up
	pairsByAccount := this.UnorderedIndexer.Values()
	slice.RemoveIf(&pairsByAccount,
		func(_ int, v *associative.Pair[*Account, []*statecell.StateCell]) bool {
			return len(v.Second) == 0
		},
	)
}

// Merge indexers so they can be updated at once.
func (this *EthIndexer) Merge(idxers []*EthIndexer) *EthIndexer {
	if len(idxers) > 0 {
		_, maxIdxer := slice.MaxIf(idxers, func(idxer *EthIndexer) int64 { return idxer.Version })
		this.Version = maxIdxer.Version

		slice.RemoveIf(&idxers, func(_ int, idxer *EthIndexer) bool {
			return (idxer.dirtyAccounts) == nil
		})

		this.dirtyAccounts = slice.ConcateDo(idxers,
			func(idxer *EthIndexer) uint64 { return uint64(len(idxer.dirtyAccounts)) },
			func(idxer *EthIndexer) []*Account { return idxer.dirtyAccounts })
	}
	return this
}
