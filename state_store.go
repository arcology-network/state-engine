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

package statestore

import (
	// "github.com/arcology-network/concurrenturl/commutative"

	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
	statecommon "github.com/arcology-network/state-engine/common"
	committer "github.com/arcology-network/state-engine/state/committer"

	statecell "github.com/arcology-network/common-lib/crdt/statecell"
	statecache "github.com/arcology-network/state-engine/state/cache"
	proxy "github.com/arcology-network/state-engine/storage/proxy"
	"github.com/cespare/xxhash/v2"
)

// Buffer is simpliest  of indexers. It does not index anything, just stores the transitions.
type StateStore struct {
	*statecache.ExecutionStateCache // execution cache, cleared at the end of each block.
	*committer.StateCommitter
	backend proxy.ReadWriteStore
}

// New creates a new StateCommitter instance.
func NewStateStore(backend proxy.ReadWriteStore) *StateStore {
	store := &StateStore{
		backend: backend,
		ExecutionStateCache: statecache.NewExecutionStateCache(
			backend,
			16,
			1,
			func(k string) uint64 {
				return xxhash.Sum64String(k)
			},
		),
	}
	store.StateCommitter = committer.NewStateCommitter(store.ExecutionStateCache, store.GetWriters())

	// Commit initial transitions to the store if any.
	initTrans := []*statecell.StateCell{
		// statecell.NewStateCell(statecommon.SYSTEM, stgcommon.GAS_PREPAYERS, 0, 1, 0, commutative.NewPath(), nil),
	}

	for _, tran := range initTrans {
		tran.SkipConflictCheck(true) // Skip conflict check for initial transitions
	}

	committer := committer.NewStateCommitter(store, store.GetWriters())
	committer.Import(initTrans)
	committer.DebugPrecommit([]uint64{statecommon.SYSTEM})
	committer.DebugCommit(0)
	return store
}

func (this *StateStore) Backend() proxy.ReadWriteStore          { return this.backend }
func (this *StateStore) Cache() *statecache.ExecutionStateCache { return this.ExecutionStateCache }
func (this *StateStore) Import(trans []*statecell.StateCell)    { this.StateCommitter.Import(trans) }
func (this *StateStore) Preload(key []byte) any {
	return this.backend.Preload(key)
}
func (this *StateStore) Clear() { this.ExecutionStateCache.Clear() }

func (this *StateStore) GetWriters() []crdtcommon.Writer[*statecell.StateCell] {
	selfWriter := []crdtcommon.Writer[*statecell.StateCell]{
		statecache.NewExecutionCacheWriter(this.ExecutionStateCache, -1)}

	if this.backend == nil {
		return selfWriter
	}

	return append(selfWriter, this.backend.GetWriters()...)
}
