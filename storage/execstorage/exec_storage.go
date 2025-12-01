/*
 *   Copyright (c) 2025 Arcology Network

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

package execstorage

import (
	"math"

	livebackend "github.com/arcology-network/state-engine/storage/execstorage/livebackend"
	livecache "github.com/arcology-network/state-engine/storage/execstorage/livecache"

	ccbadger "github.com/arcology-network/common-lib/storage/badger"
	"github.com/arcology-network/common-lib/storage/memdb"

	arcocodec "github.com/arcology-network/state-engine/storage/codec/arcocodec"
)

// ExecStorage is the execution-time storage view used by the VM.
// It composes a high-level object cache on top of a byte-level
// cached persistent execution store.
//
// Layout:
//   - LiveCache    : object-level cache (decoded)
//   - LiveStorage  : encoded-byte cache + persistent backend
type ExecStorage struct {
	cache   *livecache.LiveCache     // Object-level speculative execution cache
	storage *livebackend.LiveStorage // storage is the persistent execution-state backend.
}

// NewCacheOnlyExecStorage creates an execution storage with only an in-memory cache.
func NewCacheOnlyExecStorage(cacheSize uint64) *ExecStorage {
	return &ExecStorage{
		cache: livecache.NewLiveCache(cacheSize),
		storage: livebackend.NewLiveStorage(
			nil,
			arcocodec.Codec{}.Encode,
			arcocodec.Codec{}.Decode,
		),
	}
}

// NewBadgerExecStorage creates an execution storage with a BadgerDB backend.
func NewBadgerExecStorage(dbpath string) *ExecStorage {
	execStorage := NewCacheOnlyExecStorage(math.MaxUint64)
	execStorage.storage.SetBackend(ccbadger.NewBadgerDB(dbpath + "_badger"))
	return execStorage
}

// NewMemDBExecStorage creates an execution storage with an in-memory database backend.
func NewMemDBExecStorage(cacheSize uint64) *ExecStorage {
	proxy := NewCacheOnlyExecStorage(cacheSize)
	proxy.storage.SetBackend(memdb.NewMemoryDB())
	return proxy
}

func (this *ExecStorage) EnableCache() *ExecStorage {
	this.cache.Enable()
	return this
}

func (this *ExecStorage) DisableCache() *ExecStorage {
	this.cache.Disable()
	return this
}

func (this *ExecStorage) ClearExecCache() { this.cache.Clear() }
