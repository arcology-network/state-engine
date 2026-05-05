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
	"runtime"

	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
	statecell "github.com/arcology-network/common-lib/crdt/statecell"
	"github.com/arcology-network/common-lib/exp/slice"
)

// LiveCacheIndexer is simpliest  of indexers. It does not index anything, just stores the transitions.
type LiveCacheIndexer struct {
	Version      int64
	buffer       []*statecell.StateCell
	importBuffer []*statecell.StateCell
	keys         []string
	values       []crdtcommon.CRDT
	filter       func(*statecell.StateCell) bool
}

func NewLiveCacheIndexer(Version int64, filter func(*statecell.StateCell) bool) *LiveCacheIndexer {
	return &LiveCacheIndexer{
		Version:      Version,
		importBuffer: []*statecell.StateCell{},
		keys:         []string{},
		filter:       filter,
		values:       []crdtcommon.CRDT{},
	}
}

// An index by account address, transitions have the same Eth account address will be put together in a list
// This is for ETH storage, concurrent container related sub-paths won't be put into this index.
func (this *LiveCacheIndexer) Import(transitions []*statecell.StateCell) {
	for i := range transitions {
		if this.filter(transitions[i]) {
			this.importBuffer = append(this.importBuffer, transitions[i])
		}
	}
}

func (this *LiveCacheIndexer) PreCommit() {} // Placeholder functions

func (this *LiveCacheIndexer) Reset() { // Clear the buffer
	this.buffer = this.importBuffer
	this.importBuffer = []*statecell.StateCell{}
}

func (this *LiveCacheIndexer) Finalize() {
	slice.RemoveIf((*[]*statecell.StateCell)(&this.buffer), func(i int, v *statecell.StateCell) bool { return v.GetPath() == nil })

	this.keys = make([]string, len(this.buffer))
	this.values = slice.ParallelTransform(this.buffer, runtime.NumCPU(), func(i int, v *statecell.StateCell) crdtcommon.CRDT {
		this.keys[i] = *v.GetPath()
		if v.Value() != nil {
			return v.Value().(crdtcommon.CRDT)
		}
		return nil // A deletion
	})
}

// Merge indexers so they can be updated at once. This is useful when working
// with multiple indexers at once.
func (this *LiveCacheIndexer) Merge(idxers []*LiveCacheIndexer) *LiveCacheIndexer {
	slice.Remove(&idxers, nil)

	this.buffer = slice.ConcateDo(idxers,
		func(idxer *LiveCacheIndexer) uint64 { return uint64(len(idxer.buffer)) },
		func(idxer *LiveCacheIndexer) []*statecell.StateCell { return idxer.buffer })

	// this.keys = slice.ConcateDo(idxers,
	// 	func(idxer *LiveCacheIndexer) uint64 { return uint64(len(idxer.keys)) },
	// 	func(idxer *LiveCacheIndexer) []string { return idxer.keys })

	// this.values = slice.ConcateDo(idxers,
	// 	func(idxer *LiveCacheIndexer) uint64 { return uint64(len(idxer.values)) },
	// 	func(idxer *LiveCacheIndexer) []crdtcommon.CRDT { return idxer.values })

	return this
}
