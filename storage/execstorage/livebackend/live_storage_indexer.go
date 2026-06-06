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

	statecell "github.com/arcology-network/common-lib/crdt/statecell"
	"github.com/arcology-network/common-lib/exp/slice"

	// intf "github.com/arcology-network/state-engine/interfaces"
	stgcodec "github.com/arcology-network/common-lib/storage/codec"
)

// An index by account address, transitions have the same Eth account address will be put together in a list
// This is for ETH storage, concurrent container related sub-paths won't be put into this index.
type LiveStgIndexer[V0, V1 any] struct {
	buffer []*statecell.StateCell // buffer for the transitions that are ready to be committed conflict-free detection.

	importBuffer []*statecell.StateCell // buffer for importing data, subject to conflict detection.
	// liveStg       *LiveStorage[V0, V1]
	partitionIDs  []uint64
	keyBuffer     []string
	valueBuffer   []*V0 // For cache using the typed values.
	encodedBuffer []*V1 //The encoded buffer contains the encoded values
	filter        func(*statecell.StateCell) bool

	codec *stgcodec.StorageCodec[string, V0, string, V1]
}

func NewLiveStgIndexer[V0, V1 any](
	// liveStg *LiveStorage[V0, V1],
	_ int64,
	codec *stgcodec.StorageCodec[string, V0, string, V1],
	filter func(*statecell.StateCell) bool,
) *LiveStgIndexer[V0, V1] {
	return &LiveStgIndexer[V0, V1]{
		buffer:       []*statecell.StateCell{},
		codec:        codec,
		importBuffer: []*statecell.StateCell{},
		// liveStg:       liveStg,
		partitionIDs:  []uint64{},
		filter:        filter,
		keyBuffer:     []string{},
		valueBuffer:   []*V0{},
		encodedBuffer: []*V1{},
	}
}

// An index by account address, transitions have the same Eth account address will be put together in a list
// This is for ETH storage, concurrent container related sub-paths won't be put into this index.
func (this *LiveStgIndexer[V0, V1]) Import(trans []*statecell.StateCell) {
	for i := range trans {
		if trans[i].GetPath() != nil && this.filter(trans[i]) {
			this.importBuffer = append(this.importBuffer, trans[i])
		}
	}
}

func (this *LiveStgIndexer[V0, V1]) Reset() {
	this.buffer = this.importBuffer
	this.importBuffer = []*statecell.StateCell{}
}

func (this *LiveStgIndexer[V0, V1]) Finalize() {
	slice.RemoveIf(&this.buffer, func(_ int, v *statecell.StateCell) bool {
		return v.GetPath() == nil
	}) // Remove the transitions that are marked

	// Extract the keys and values from the buffer
	this.keyBuffer = make([]string, len(this.buffer))
	this.valueBuffer = slice.ParallelTransform(this.buffer, runtime.NumCPU(),
		func(i int, v *statecell.StateCell) *V0 {
			this.keyBuffer[i] = *v.GetPath()
			if v.Value() != nil {
				val := v.Value().(V0)
				return &val
			}
			return nil
		},
	)

	// Encode the keys and values to the buffer so that they can be written to calcualte the root hash.
	// backendCodec := this.codec.ForwardConvert
	this.encodedBuffer = make([]*V1, len(this.valueBuffer))
	for i := 0; i < len(this.valueBuffer); i++ {
		if this.valueBuffer[i] != nil {
			_, v, _ := this.codec.ForwardConvert(this.keyBuffer[i], *this.valueBuffer[i])
			this.encodedBuffer[i] = &v
		}
	}
}

// Merge indexers so they can be updated at once.
func (this *LiveStgIndexer[V0, V1]) Merge(idxers []*LiveStgIndexer[V0, V1]) *LiveStgIndexer[V0, V1] {
	slice.Remove(&idxers, nil)

	this.partitionIDs = slice.ConcateDo(idxers,
		func(idxer *LiveStgIndexer[V0, V1]) uint64 { return uint64(len(idxer.partitionIDs)) },
		func(idxer *LiveStgIndexer[V0, V1]) []uint64 { return idxer.partitionIDs })

	this.keyBuffer = slice.ConcateDo(idxers,
		func(idxer *LiveStgIndexer[V0, V1]) uint64 { return uint64(len(idxer.keyBuffer)) },
		func(idxer *LiveStgIndexer[V0, V1]) []string { return idxer.keyBuffer })

	this.valueBuffer = slice.ConcateDo(idxers,
		func(idxer *LiveStgIndexer[V0, V1]) uint64 { return uint64(len(idxer.valueBuffer)) },
		func(idxer *LiveStgIndexer[V0, V1]) []*V0 { return idxer.valueBuffer })

	this.encodedBuffer = slice.ConcateDo(idxers,
		func(idxer *LiveStgIndexer[V0, V1]) uint64 { return uint64(len(idxer.encodedBuffer)) },
		func(idxer *LiveStgIndexer[V0, V1]) []*V1 { return idxer.encodedBuffer })

	return this
}
