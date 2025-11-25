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
package livecache

import (
	"fmt"
	"runtime"

	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
	statecell "github.com/arcology-network/common-lib/crdt/statecell"
	"github.com/arcology-network/common-lib/exp/associative"
	"github.com/arcology-network/common-lib/exp/slice"
	cache "github.com/arcology-network/common-lib/storage/cache"

	// intf "github.com/arcology-network/state-engine/interfaces"

	"github.com/cespare/xxhash/v2"
)

// LiveCache acts as an in-memory cache layer for state data, providing fast access to frequently used values.
// When a state access request is made, the system first checks the LiveCache for the data.
// Only if the requested data is not found in the LiveCache does the system query the underlying storage.
// This approach reduces storage access latency and improves overall system performance by keeping hot data
// readily available.

type LiveCache struct {
	*cache.ReadCache[string, *associative.Pair[crdtcommon.Type, *Profile]]               // Provide Readonly interface
	profile                                                                *CacheProfile // Memory usage of the cache itself.
}

func NewLiveCache(cacheCap uint64) *LiveCache {
	cache := &LiveCache{
		ReadCache: cache.NewReadCache(
			4096, // 4096 shards to avoid lock contention
			func(v *associative.Pair[crdtcommon.Type, *Profile]) bool {
				return v == nil
			},
			func(k string) uint64 {
				return uint64(xxhash.Sum64String(k))
			},
		),
	}

	cache.profile = NewCacheProfile(cacheCap, cache) // To keep track of the cache memory usage.
	return cache
}

func (this *LiveCache) Profile() *CacheProfile { return this.profile }
func (this *LiveCache) Size() uint64           { return this.profile.occupied }

func (this *LiveCache) CacheChecksum() [32]byte {
	encoders := func(k string, v *associative.Pair[crdtcommon.Type, *Profile]) ([]byte, []byte) {
		return []byte(k), v.First.Encode()
	}

	less := func(k0, k1 string) bool {
		return k0 < k1
	}
	return this.ReadCache.Checksum(less, encoders)
}

func (this *LiveCache) Delete(keys []string) {
	this.ReadCache.BatchSet(keys, make([]*associative.Pair[crdtcommon.Type, *Profile], len(keys)))
}

func (this *LiveCache) Get(key string) (crdtcommon.Type, bool) {
	v, ok := this.ReadCache.Get(key)
	if !ok {
		return nil, ok
	}
	return (*v).First, ok
}

// Get the raw value from the cache with the usage information.
func (this *LiveCache) GetRaw(key string) (*associative.Pair[crdtcommon.Type, *Profile], bool) {
	v, ok := this.ReadCache.Get(key)
	if !ok {
		return nil, ok
	}
	return *v, ok
}

func (this *LiveCache) Commit(univals []*statecell.StateCell, block uint64) {
	// Cache is disabled, do nothing.
	if !this.Status() {
		return
	}

	// Prepare the space for the new values in the cache, some univalues may be deleted because of the memory limit.
	this.profile.PrepareSpace(&univals, this)

	// Extract the keys and values from the univalues.
	keys := slice.ParallelTransform(univals, runtime.NumCPU(), func(i int, v *statecell.StateCell) string {
		return *v.GetPath()
	})

	pairedVals := slice.ParallelTransform(univals, runtime.NumCPU(), func(i int, v *statecell.StateCell) *associative.Pair[crdtcommon.Type, *Profile] {
		if v.Value() == nil {
			return nil
		}

		pair := &associative.Pair[crdtcommon.Type, *Profile]{
			First: v.Value().(crdtcommon.Type),
			Second: &Profile{
				sizeInMem:   v.Value().(crdtcommon.Type).MemSize(),
				visits:      uint64(v.Reads()) + uint64(v.Writes()) + uint64(v.DeltaWrites()),
				firstLoaded: uint32(block),
			},
		}

		// The entry may already exist in the cache, update the visits.
		if metav, _ := this.GetRaw(*v.GetPath()); metav != nil {
			pair.Second.visits += metav.Second.visits
			pair.Second.firstLoaded = metav.Second.firstLoaded
		}

		return pair
	})

	this.ReadCache.DebugCommit(keys, pairedVals) // update the local cache with the new values in the indexer
}

func (this *LiveCache) Print() {
	keys, vals := this.ReadCache.KVs()
	slice.SortBy1st(keys, vals, func(k0, k1 string) bool {
		return k0 < k1
	})

	fmt.Println("occupied:", this.profile.occupied)

	for i, k := range keys {
		println(k, "      ", vals[i].First)
	}
}
