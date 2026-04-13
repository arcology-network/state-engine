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

// ExecutionStateCache is a read-only data store used for caching.

package cache

import (
	"runtime"

	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
	statecell "github.com/arcology-network/common-lib/crdt/statecell"
	slice "github.com/arcology-network/common-lib/exp/slice"
)

const (
	NUM_SHARDS = 32
)

// ShardedStateCache is a lockless data strucuture that wraps multiple ExecutionStateCache instances together, each of
// which is responsible for a subset of the data. It can be updated in parallel when a transaction generation
// is completed. But it isn't thread-safe.
type ShardedStateCache struct {
	backend crdtcommon.ReadOnlyStore
	caches  [NUM_SHARDS]*ExecutionStateCache
	hasher  func(string) uint64
	queue   chan *[]*statecell.StateCell
}

func NewShardedStateCache(backend crdtcommon.ReadOnlyStore, perPage int, numPages int, hasher func(string) uint64, args ...interface{}) *ShardedStateCache {
	writeCache := &ShardedStateCache{
		backend: backend,
		hasher:  hasher,
	}

	for i := 0; i < len(writeCache.caches); i++ {
		writeCache.caches[i] = NewExecutionStateCache(backend, perPage, numPages, args...)
	}
	writeCache.queue = make(chan *[]*statecell.StateCell, 64)
	return writeCache
}

func (this *ShardedStateCache) ReadOnlyStore() crdtcommon.ReadOnlyStore { return this.backend }
func (this *ShardedStateCache) Cache() [NUM_SHARDS]*ExecutionStateCache { return this.caches }

func (this *ShardedStateCache) NewStateCell(k string) *statecell.StateCell {
	return this.caches[this.hasher(k)].NewStateCell()
}

func (this *ShardedStateCache) Read(tx uint64, path string, T crdtcommon.CRDT) (interface{}, interface{}, uint64) {
	return this.caches[this.hasher(path)%NUM_SHARDS].Read(tx, path, T)
}

func (this *ShardedStateCache) Write(tx uint64, path string, v crdtcommon.CRDT) (int64, error) {
	return this.caches[this.hasher(path)%NUM_SHARDS].Write(tx, path, v)
}

// func (this *ShardedStateCache) GetIfCached(path string) (interface{}, bool) {
// 	return this.caches[this.hasher(path)%NUM_SHARDS].GetIfCached(path)
// }

func (this *ShardedStateCache) GetAs(path string, T crdtcommon.CRDT) (interface{}, error) {
	return this.caches[this.hasher(path)%NUM_SHARDS].GetAs(path, T)
}

func (this *ShardedStateCache) Has(path string) bool {
	return this.caches[this.hasher(path)%NUM_SHARDS].Has(path)
}

func (this *ShardedStateCache) Import(transitions []*statecell.StateCell) *ShardedStateCache {
	statecell.StateCells(transitions).SortByDepth() // To ensure that the parent  is inserted before the child

	// Precalculate the shard ID of each transition
	shards := slice.ParallelTransform(transitions, runtime.NumCPU(), func(i int, v *statecell.StateCell) uint64 {
		return this.hasher(*(v).GetPath())
	})

	// Insert each transition into the appropriate cache
	slice.ParallelForeach(this.caches[:], runtime.NumCPU(), func(num int, shard **ExecutionStateCache) {
		for i := 0; i < len(transitions); i++ {
			if shards[i] == uint64(num) {
				this.caches[num].set(transitions[i])
			}
		}
	})
	return this
}

// Reset the writecache to the initial state for the next round of processing.
// func (this *ShardedStateCache) Precommit([]uint32) [32]byte { return [32]byte{} }

func (this *ShardedStateCache) Reset() *ShardedStateCache {
	slice.ParallelForeach(this.caches[:], runtime.NumCPU(), func(i int, wcache **ExecutionStateCache) {
		(*wcache).Clear()
	})
	return this
}

func (this *ShardedStateCache) Equal(other *ShardedStateCache) bool {
	for i := 0; i < len(this.caches); i++ {
		if !this.caches[i].Equal(other.caches[i]) {
			return false
		}
	}
	return true
}

func (this *ShardedStateCache) KVs() ([][]string, [][]crdtcommon.CRDT) {
	keySet, valueSet := make([][]string, len(this.caches)), make([][]crdtcommon.CRDT, len(this.caches))
	for i := 0; i < len(this.caches); i++ {
		keySet[i], valueSet[i] = this.caches[i].KVs()
	}
	return keySet, valueSet
}

func (this *ShardedStateCache) Export(preprocs ...func([]*statecell.StateCell) []*statecell.StateCell) []*statecell.StateCell {
	valueSet := make([][]*statecell.StateCell, len(this.caches))
	for i := 0; i < len(this.caches); i++ {
		valueSet[i] = this.caches[i].Export()
	}
	return slice.Flatten(valueSet)
}
