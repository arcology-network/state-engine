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
	"errors"
	"runtime"

	statecell "github.com/arcology-network/common-lib/crdt/statecell"
	"github.com/arcology-network/common-lib/exp/associative"
	"github.com/arcology-network/common-lib/exp/slice"
)

type EthStorageWriter struct {
	*EthIndexer
	buffer   []*EthIndexer
	ethStore *EthWorldState
	filter   func(*statecell.StateCell) bool // Filter function to select transitions to be indexed
	Err      error
}

func NewEthStorageWriter(ethStore *EthWorldState, version int64, filter func(*statecell.StateCell) bool) *EthStorageWriter {
	return &EthStorageWriter{
		EthIndexer: NewEthIndexer(ethStore, version, filter),
		ethStore:   ethStore,
		buffer:     []*EthIndexer{},
		filter:     filter,
	}
}

func (this *EthStorageWriter) Precommit(isSync bool) {
	this.EthIndexer.Finalize() // Remove the nil transitions
	this.buffer = append(this.buffer, this.EthIndexer)

	accounts := this.EthIndexer.UnorderedIndexer.Values()                                                    // Export all the pairs to be written to the db
	this.EthIndexer.dirtyAccounts = (associative.Pairs[*Account, []*statecell.StateCell])(accounts).Firsts() // Get the accounts.

	// Account cache holds the accounts that are being updated in the current block.
	// TODO: Need to check if this is necessary or could be moved to the import phase instead.
	slice.Foreach(this.EthIndexer.dirtyAccounts, func(_ int, pair **Account) {
		this.ethStore.accountCache[(**pair).Address()] = (*pair) // Add the account to the cache
	})

	slice.ParallelForeach(accounts, runtime.NumCPU(), func(i int, acctTrans **associative.Pair[*Account, []*statecell.StateCell]) {
		if len((*acctTrans).Second) == 0 {
			return // All removed
		}

		keys, vals := statecell.StateCells((*acctTrans).Second).KVs()                // Get all transitions under the same account
		err := this.EthIndexer.dirtyAccounts[i].UpdateAccountStorageTrie(keys, vals) //
		if err != nil {
			this.ethStore.dbErr = errors.Join(this.ethStore.dbErr, err)
		}
	})

	this.ethStore.WriteWorldTrie(this.EthIndexer.dirtyAccounts)     // Update the world trie
	this.EthIndexer = NewEthIndexer(this.ethStore, -1, this.filter) // Reset the indexer with a default version number.
	this.EthIndexer.UnorderedIndexer.Clear()
}

// Signals a block is completed, time to write to the db.
func (this *EthStorageWriter) Commit(version uint64) {
	mergedIdxer := new(EthIndexer).Merge(this.buffer[:]) // Merge all the indexers together to commit to the db at once.
	this.ethStore.ShouldPersistToEth(uint64(mergedIdxer.Version), mergedIdxer.dirtyAccounts)
	this.buffer = this.buffer[:0]
}

func (this *EthStorageWriter) IsSync() bool { return false }
func (this *EthStorageWriter) Name() string { return "Eth Storage Writer" }
