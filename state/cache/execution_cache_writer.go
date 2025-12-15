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

package cache

// ExecutionCacheWriter is a struct that contains data strucuture and methods for writing data to cache.
// The indexer is used to index the input transitions as they are received, in a way that they can be
// isCommitted efficiently later.
type ExecutionCacheWriter struct {
	*ExecutionCacheIndexer
	*StateCache
}

func NewExecutionCacheWriter(writeCache *StateCache, version int64) *ExecutionCacheWriter {
	return &ExecutionCacheWriter{
		ExecutionCacheIndexer: NewExecutionCacheIndexer(nil, int64(version), nil),
		StateCache:            writeCache,
	}
}

// write cache updates itself every generation. It doesn't need to write to the database.
func (this *ExecutionCacheWriter) Precommit(_ bool) {
	this.ExecutionCacheIndexer.Finalize() // Remove the nil transitions
	for i := range this.ExecutionCacheIndexer.buffer {
		this.StateCache.localCells[*this.ExecutionCacheIndexer.buffer[i].GetPath()] = this.ExecutionCacheIndexer.buffer[i]
	}
	this.ExecutionCacheIndexer = NewExecutionCacheIndexer(nil, -1, nil)

}

// The generation cache is transient and will clear itself when all the transitions are isCommitted to
// the database.
func (this *ExecutionCacheWriter) Commit(_ uint64) {
	this.StateCache.Clear()
	this.ExecutionCacheIndexer.buffer = this.ExecutionCacheIndexer.buffer[:0]
}

func (this *ExecutionCacheWriter) IsSync() bool { return true } // Execution cache is always synchronous.
func (this *ExecutionCacheWriter) Name() string { return "Execution Cache Writer" }
