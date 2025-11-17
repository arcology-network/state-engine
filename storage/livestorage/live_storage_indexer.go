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

package ccstorage

import (
	"runtime"

	"github.com/arcology-network/common-lib/exp/slice"
	statecell "github.com/arcology-network/storage-committer/type/statecell"
	// intf "github.com/arcology-network/storage-committer/interfaces"
)

// An index by account address, transitions have the same Eth account address will be put together in a list
// This is for ETH storage, concurrent container related sub-paths won't be put into this index.
type LiveStgIndexer struct {
	buffer       []*statecell.StateCell
	importBuffer []*statecell.StateCell
	liveStg      *LiveStorage

	partitionIDs  []uint64
	keyBuffer     []string
	valueBuffer   []any
	encodedBuffer [][]byte //The encoded buffer contains the encoded values
	filter        func(*statecell.StateCell) bool
}

func NewLiveStgIndexer(liveStg *LiveStorage, _ int64, filter func(*statecell.StateCell) bool) *LiveStgIndexer {
	return &LiveStgIndexer{
		// buffer:       []*statecell.StateCell{},
		importBuffer: []*statecell.StateCell{},
		liveStg:      liveStg,

		partitionIDs:  []uint64{},
		filter:        filter,
		keyBuffer:     []string{},
		valueBuffer:   []any{},
		encodedBuffer: [][]byte{},
	}
}

// An index by account address, transitions have the same Eth account address will be put together in a list
// This is for ETH storage, concurrent container related sub-paths won't be put into this index.
func (this *LiveStgIndexer) Import(trans []*statecell.StateCell) {
	for i := range trans {
		if trans[i].GetPath() != nil && this.filter(trans[i]) {
			this.importBuffer = append(this.importBuffer, trans[i])
		}
	}
}

func (this *LiveStgIndexer) PreCommit() {
	this.buffer = this.importBuffer
	this.importBuffer = []*statecell.StateCell{}
}

func (this *LiveStgIndexer) Finalize() {
	slice.RemoveIf(&this.buffer, func(_ int, v *statecell.StateCell) bool {
		return v.GetPath() == nil
	}) // Remove the transitions that are marked

	// Extract the keys and values from the buffer
	this.keyBuffer = make([]string, len(this.buffer))
	this.valueBuffer = slice.ParallelTransform(this.buffer, runtime.NumCPU(), func(i int, v *statecell.StateCell) any {
		this.keyBuffer[i] = *v.GetPath()
		return v.Value()
	})

	// Encode the keys and values to the buffer so that they can be written to calcualte the root hash.
	this.encodedBuffer = make([][]byte, len(this.valueBuffer))
	for i := 0; i < len(this.valueBuffer); i++ {
		if this.valueBuffer[i] != nil {
			this.encodedBuffer[i] = this.liveStg.encoder(this.keyBuffer[i], this.valueBuffer[i])
		}
	}
}

// Merge indexers so they can be updated at once.
func (this *LiveStgIndexer) Merge(idxers []*LiveStgIndexer) *LiveStgIndexer {
	slice.Remove(&idxers, nil)

	this.partitionIDs = slice.ConcateDo(idxers,
		func(idxer *LiveStgIndexer) uint64 { return uint64(len(idxer.partitionIDs)) },
		func(idxer *LiveStgIndexer) []uint64 { return idxer.partitionIDs })

	this.keyBuffer = slice.ConcateDo(idxers,
		func(idxer *LiveStgIndexer) uint64 { return uint64(len(idxer.keyBuffer)) },
		func(idxer *LiveStgIndexer) []string { return idxer.keyBuffer })

	this.valueBuffer = slice.ConcateDo(idxers,
		func(idxer *LiveStgIndexer) uint64 { return uint64(len(idxer.valueBuffer)) },
		func(idxer *LiveStgIndexer) []any { return idxer.valueBuffer })

	this.encodedBuffer = slice.ConcateDo(idxers,
		func(idxer *LiveStgIndexer) uint64 { return uint64(len(idxer.encodedBuffer)) },
		func(idxer *LiveStgIndexer) [][]byte { return idxer.encodedBuffer })

	return this
}
