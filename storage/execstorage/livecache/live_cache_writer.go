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
	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
	statecell "github.com/arcology-network/common-lib/crdt/statecell"
	"github.com/arcology-network/common-lib/exp/slice"
	"github.com/arcology-network/common-lib/storage/cache"
)

type LiveCacheWriter struct {
	*LiveCacheIndexer
	cache   *cache.Cache[string, crdtcommon.CRDT]
	buffer  []*LiveCacheIndexer             // For multiple generations. Each geneartion has its own indexer.
	version int64                           // The version of the indexer, used for debugging and tracking.
	filter  func(*statecell.StateCell) bool // Filter function to select transitions to be indexed
}

func NewLiveCacheWriter(
	cache *cache.Cache[string, crdtcommon.CRDT],
	version int64,
	filter func(*statecell.StateCell) bool,
) *LiveCacheWriter {
	return &LiveCacheWriter{
		LiveCacheIndexer: NewLiveCacheIndexer(version, filter),
		cache:            cache,
		buffer:           make([]*LiveCacheIndexer, 0),
		version:          version,
		filter:           filter,
	}
}

// Import the transitions into the indexer
func (this *LiveCacheWriter) Import(transitions []*statecell.StateCell) {
	this.LiveCacheIndexer.Import(transitions)
}

// Send the data to the downstream processor, this is called for each generation.
// If there are multiple generations, this can be called multiple times before isSync==true
func (this *LiveCacheWriter) Precommit(isSync bool) error {
	if isSync {
		this.LiveCacheIndexer.Reset() // In the sync phase, clear the buffer.
	} else {
		this.LiveCacheIndexer.Finalize()                             // Remove the nil transitions
		this.buffer = append(this.buffer, this.LiveCacheIndexer)     // Append the indexer to the buffer
		this.LiveCacheIndexer = NewLiveCacheIndexer(-1, this.filter) // Reset the indexer with a default version number
	}
	return nil
}

func (this *LiveCacheWriter) Reset() {
	this.LiveCacheIndexer.Reset()
}

// Triggered by the block commit.
func (this *LiveCacheWriter) Commit(block uint64) error {
	_ = block
	merged := new(LiveCacheIndexer).Merge(this.buffer) // Merge indexers

	keys := make([]string, len(merged.buffer))
	values := make([]crdtcommon.CRDT, len(merged.buffer))
	for i, cell := range merged.buffer {
		keys[i] = *cell.GetPath()
		if cell.Value() == nil {
			continue
		}
		values[i] = cell.Value().(crdtcommon.CRDT)
	}

	keyToDel, _ := slice.MoveBothIf(&keys, &values, func(i int, s string, v crdtcommon.CRDT) bool {
		return v == nil
	})

	this.cache.DeleteBatch(keyToDel)
	this.cache.SetBatch(keys, values)
	this.cache.Evict()

	this.buffer = make([]*LiveCacheIndexer, 0) // Reset the indexer buffer
	return nil
}

func (this *LiveCacheWriter) IsSync() bool { return true }
func (this *LiveCacheWriter) Name() string { return "Live Cache Writer" }
