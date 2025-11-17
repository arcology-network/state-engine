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

import statecell "github.com/arcology-network/storage-committer/type/statecell"

// LiveCacheWriter writes to the LiveCache.

type LiveCacheWriter struct {
	*LiveCacheIndexer
	liveCache *LiveCache
	buffer    []*LiveCacheIndexer             // For multiple generations. Each geneartion has its own indexer.
	version   int64                           // The version of the indexer, used for debugging and tracking.
	filter    func(*statecell.StateCell) bool // Filter function to select transitions to be indexed
}

func NewLiveCacheWriter(cache *LiveCache, version int64, filter func(*statecell.StateCell) bool) *LiveCacheWriter {
	return &LiveCacheWriter{
		LiveCacheIndexer: NewLiveCacheIndexer(cache, version, filter),
		liveCache:        cache,
		buffer:           make([]*LiveCacheIndexer, 0),
		version:          version,
		filter:           filter,
	}
}

// Import the transitions into the indexer
func (this *LiveCacheWriter) Import(transitions []*statecell.StateCell) {
	if !this.liveCache.Status() {
		return // Cache is disabled, do nothing.
	}
	this.LiveCacheIndexer.Import(transitions)
}

// Send the data to the downstream processor, this is called for each generation.
// If there are multiple generations, this can be called multiple times before Await.
// Each generation
func (this *LiveCacheWriter) Precommit(isSync bool) {
	if !this.liveCache.Status() {
		return // Cache is disabled, do nothing.
	}

	if isSync {
		this.LiveCacheIndexer.PreCommit()
	} else {
		this.LiveCacheIndexer.Finalize()                                             // Remove the nil transitions
		this.buffer = append(this.buffer, this.LiveCacheIndexer)                     // Append the indexer to the buffer
		this.LiveCacheIndexer = NewLiveCacheIndexer(this.liveCache, -1, this.filter) // Reset the indexer with a default version number
	}
}

// Triggered by the block commit.
func (this *LiveCacheWriter) Commit(block uint64) {
	if !this.liveCache.Status() {
		return // Cache is disabled, do nothing.
	}

	merged := new(LiveCacheIndexer).Merge(this.buffer) // Merge indexers
	this.liveCache.Commit(merged.buffer, block)        // commit univalues directly
	this.buffer = make([]*LiveCacheIndexer, 0)         // Reset the indexer buffer
}

func (this *LiveCacheWriter) IsSync() bool { return true }
func (this *LiveCacheWriter) Name() string { return "Live Cache Writer" }
