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
	"math"
	"runtime"
	"sort"
	"sync/atomic"

	"github.com/arcology-network/common-lib/common"
	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
	statecell "github.com/arcology-network/common-lib/crdt/statecell"
	"github.com/arcology-network/common-lib/exp/associative"
	mapi "github.com/arcology-network/common-lib/exp/map"
	"github.com/arcology-network/common-lib/exp/slice"
)

type Profile struct {
	sizeInMem   uint64
	firstLoaded uint32
	visits      uint64
}

type CacheProfile struct {
	liveCache *LiveCache
	minVisits uint64
	maxVisits uint64
	avgVisits uint64

	dist     [65536]uint64 // size distribution
	occupied uint64        // The total memory used by the cache.
	maxSize  uint64
}

func NewCacheProfile(maxSize uint64, liveCache *LiveCache) *CacheProfile {
	usage := &CacheProfile{
		// lookup:   make(map[string]*Profile),
		liveCache: liveCache,
		minVisits: 0,
		maxVisits: 0,
		// keys:      paged.NewPagedSlice[*Profile](1024, 100, 0),
		dist:     [65536]uint64{},
		occupied: 0,
		maxSize:  uint64(24 * 1024 * 1024 * 1024), // 0.8 of the minimum memory required.
	}

	if v, err := common.GetAvailableMemory(); err == nil {
		usage.maxSize = common.Min(maxSize, uint64(float64(v)*0.8))
	}
	return usage
}

func (this *CacheProfile) MaxVisits() uint64 { return this.maxVisits }
func (this *CacheProfile) MinVisits() uint64 { return this.occupied }

// Check if the cache has enough space to store the new values.
// If not, the cache will be cleared. If still not enough space,
// some new values won't be stored.
func (this *CacheProfile) PrepareSpace(univals *[]*statecell.StateCell, liveCache *LiveCache) {
	// The total memory required to store the new values.
	totalRequired := slice.Accumulate[*statecell.StateCell, uint64](*univals, 0, func(_ int, v *statecell.StateCell) uint64 {
		if v.Value() == nil {
			return 0
		}
		return v.Value().(crdtcommon.CRDT).MemSize()
	})

	// The available memory to store the new values.
	availableMemory, err := common.GetAvailableMemory()
	if err != nil {
		return
	}
	actualCap := common.Min(this.maxSize, availableMemory) // The actual cap of the cache.

	// The memory that needs to be freed to store the new values.
	toFree := int(totalRequired) - (int(actualCap) - int(this.occupied))
	if toFree <= 0 {
		this.occupied = totalRequired
		return // Enough space, no need to free memory.
	}

	// Check if the cache has enough space to store the new values even after freeing some memory.
	// If not, remove some new values.
	freedMemory := this.freeCache(uint64(toFree))

	totalAvailable := int(actualCap) - int(this.occupied) + int(freedMemory)
	// Not enough memory for all even after freeing some memory. Some new values won't be stored.
	// Sort the univalues by size in memory, so the smallest values will still have a chance to be stored.
	if int(totalRequired) > totalAvailable {
		// Some new values won't be stored in the cache. Sort the univalues by size in memory.
		// So the smallest values will still have a chance to be stored.
		sort.Slice(*univals, func(i, j int) bool {
			return (*univals)[i].Value().(crdtcommon.CRDT).MemSize() < (*univals)[j].Value().(crdtcommon.CRDT).MemSize()
		})

		// Find the index of the last value that can be stored in the cache.
		idx := 0
		accumSize := uint64(0) // Accumulated size of the values.
		for i, v := range *univals {
			if accumSize += v.Value().(crdtcommon.CRDT).MemSize(); int(accumSize) > totalAvailable {
				idx = i // Find out the last values that can be stored in the cache.
				break
			}
		}

		// Accumulated size minus the last value's size which it isn't stored in the cache.
		// Because it made the accumulated size exceed the available memory.
		this.occupied = accumSize - (*univals)[idx].Value().(crdtcommon.CRDT).MemSize()
		*univals = (*univals)[:idx] // Some new values won't be stored in cache.
		return
	}

	this.occupied += totalRequired - freedMemory
}

// freeCache frees the required memory.
func (this *CacheProfile) freeCache(sizeToFree uint64) uint64 {
	var totalFreed atomic.Uint64
	shards := this.liveCache.ConcurrentMap.Shards()

	// Calculate the minimum memory to free for each shard.
	sizeToFree = common.Max(
		uint64(math.Ceil(float64(sizeToFree)*float64(1.15))), // Add 15% more memory to free
		uint64(len(shards)*64))                               // The minimum memory to free for each shard anyway.

	// Calculte the memory to free for each shard.
	shardTarget := slice.New(len(shards), math.Ceil(float64(sizeToFree)/float64(len(shards)))) // The memory to free for each shard.

	slice.ParallelForeach(shards, runtime.NumCPU(), func(i int, _ *map[string]*associative.Pair[crdtcommon.CRDT, *Profile]) {
		if len(shards[i]) == 0 {
			return
		}

		ks, v := mapi.KVs(shards[i])
		scores := slice.ParallelTransform(v, runtime.NumCPU(), func(i int, v *associative.Pair[crdtcommon.CRDT, *Profile]) float32 {
			return float32(v.Second.visits) / float32(sizeToFree-uint64(v.Second.firstLoaded)) // The score of the value.
		})

		// Sort the keys by the score. This isn't necessary, but it's will help check the live cache
		// consistency across multiple nodes.
		slice.SortBy1st(scores, ks, func(v0, v1 float32) bool {
			return v0 < v1
		})

		// Remove the value with the lowest scores until the required memory is freed.
		for j, k := range ks {
			delete(shards[i], k) // Delete the value from the cache.

			if shardTarget[i] -= float64(v[j].Second.sizeInMem); shardTarget[i] <= 0 {
				totalFreed.Add(v[j].Second.sizeInMem) // Keep track of the freed memory.
				break
			}
		}
	})

	// for i := 0; i < len(shards); i++ {
	// 	if len(shards[i]) == 0 {
	// 		continue
	// 	}

	// 	ks, v := common.MapKVs(shards[i])
	// 	scores := slice.ParallelTransform(v, runtime.NumCPU(), func(i int, v *associative.Pair[crdtcommon.CRDT, *Profile]) float32 {
	// 		return float32(v.Second.visits) / float32(sizeToFree-uint64(v.Second.firstLoaded)) // The score of the value.
	// 	})

	// 	// Sort the keys by the score. This isn't necessary, but it's will help check the live cache
	// 	// consistency across multiple nodes.
	// 	slice.SortBy1st(scores, ks, func(v0, v1 float32) bool {
	// 		return v0 < v1
	// 	})

	// 	// Remove the value with the lowest scores until the required memory is freed.
	// 	for j, k := range ks {
	// 		delete(shards[i], k) // Delete the value from the cache.

	// 		if shardTarget[i] -= float64(v[j].Second.sizeInMem); shardTarget[i] <= 0 {
	// 			totalFreed.Add(v[j].Second.sizeInMem) // Keep track of the freed memory.
	// 			break
	// 		}
	// 	}
	// }
	return totalFreed.Load()
}
