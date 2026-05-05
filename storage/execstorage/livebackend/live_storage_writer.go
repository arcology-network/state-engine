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
	"errors"

	statecell "github.com/arcology-network/common-lib/crdt/statecell"
	slice "github.com/arcology-network/common-lib/exp/slice"
	stgcodec "github.com/arcology-network/common-lib/storage/codec"
	commonintf "github.com/arcology-network/common-lib/storage/interface"
)

// LiveStorageWriter is a struct that contains data structure and methods for writing data to concurrent storage.
// It manages buffered writes and supports both synchronous and asynchronous commit operations to the underlying storage.
type LiveStorageWriter[V0, V1 any] struct {
	*LiveStgIndexer[V0, V1]
	buffer []*LiveStgIndexer[V0, V1]

	store commonintf.BackendStore[string, V1]
	// store   *LiveStorage[V0, V1]
	version int64
	filter  func(*statecell.StateCell) bool
}

func NewLiveStorageWriter[V0, V1 any](
	store commonintf.BackendStore[string, V1],
	version int64,
	filter func(*statecell.StateCell) bool,
	codec *stgcodec.StorageCodec[string, V0, string, V1],
) *LiveStorageWriter[V0, V1] {
	return &LiveStorageWriter[V0, V1]{
		LiveStgIndexer: NewLiveStgIndexer(0, codec, filter),
		buffer:         []*LiveStgIndexer[V0, V1]{},
		store:          store,
		version:        version,
		filter:         filter,
	}
}

// Send the data to the downstream processor. This can be called multiple times
// before calling Await to commit the data to the state db.
func (this *LiveStorageWriter[V0, V1]) Precommit(isSync bool) error {
	if isSync {
		this.LiveStgIndexer.Reset()
	} else {
		this.LiveStgIndexer.Finalize() // Remove the nil transitions
		this.buffer = append(this.buffer, this.LiveStgIndexer)
		this.LiveStgIndexer = NewLiveStgIndexer(-1, this.LiveStgIndexer.codec, this.filter)
	}
	return nil
}

// Await commits the data to the state db.
func (this *LiveStorageWriter[V0, V1]) Commit(_ uint64) error {
	mergedIdxer := new(LiveStgIndexer[V0, V1]).Merge(this.buffer)
	keyToDel, _ := slice.MoveBothIf(
		&mergedIdxer.keyBuffer,
		&mergedIdxer.encodedBuffer,
		func(i int, s string, v *V1) bool {
			return v == nil
		},
	)
	this.store.DeleteBatch(keyToDel) // Update the local cache for deletions

	encoded := slice.Transform(mergedIdxer.encodedBuffer, func(i int, v *V1) V1 {
		return *v
	})

	// Update the persistent storage if it is not nil, otherwise just update the local cache.
	// var errs []error
	// backend := this.store.Backend()
	// if backend != nil {
	// 	backend.DeleteBatch(keyToDel)
	// 	errs = backend.SetBatch(mergedIdxer.keyBuffer, encoded)
	// }

	// Update the local cache, this is for the case when the persistent storage is not nil, we want to update the local cache as well to keep it
	// consistent with the persistent storage.

	// values := slice.Transform(mergedIdxer.valueBuffer, func(i int, v *V0) V0 {
	// 	return *v
	// })

	errs := this.store.SetBatch(mergedIdxer.keyBuffer, encoded) // update the local cache
	this.buffer = this.buffer[:0]

	var aggregateErr error
	for _, err := range errs {
		aggregateErr = errors.Join(aggregateErr, err)
	}
	return aggregateErr
}

func (this *LiveStorageWriter[V0, V1]) IsSync() bool { return false } // If the storage needs synchronous writes
func (this *LiveStorageWriter[V0, V1]) Name() string { return "Live Storage Writer" }
