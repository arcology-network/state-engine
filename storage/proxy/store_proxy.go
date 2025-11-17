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

package proxy

import (
	"math"

	ccbadger "github.com/arcology-network/common-lib/storage/badger"
	memdb "github.com/arcology-network/common-lib/storage/memdb"
	"github.com/arcology-network/storage-committer/type/commutative"
	statecell "github.com/arcology-network/storage-committer/type/statecell"

	// intf "github.com/arcology-network/storage-committer/interfaces"
	intf "github.com/arcology-network/storage-committer/common"

	"github.com/arcology-network/storage-committer/storage/ethstorage"
	ethstg "github.com/arcology-network/storage-committer/storage/ethstorage"
	livecache "github.com/arcology-network/storage-committer/storage/livecache"
	ccstorage "github.com/arcology-network/storage-committer/storage/livestorage"
	livestg "github.com/arcology-network/storage-committer/storage/livestorage"
	stgtypecommon "github.com/arcology-network/storage-committer/type/common"
)

// StorageProxy is a proxy for the storage, it consists of multiple storages and caches.
// The LiveCache is a memory cache of the liveStorage, used for the execution, it holds some of the
// latest state data, depending on the capacity of the cache, regardless of the storage type.
//
// The LiveStorage is a persistent storage, it holds all the latest state data, regardless of the
// storage type. EVM uses the LiveCache and LiveStorage for the execution ONLY.
//
// EthStorage is used for the Ethereum storage, which is a persistent storage, it holds only the Ethereum state data.
// The EthStorage won't be used for the execution cache, it is only used for user APIs to query the Ethereum state data.

type StorageProxy struct {
	platform    *stgtypecommon.Platform
	execCache   *livecache.LiveCache // An object cache for the backend storage, only updated once at the end of the block.
	execStorage *livestg.LiveStorage
	ethStorage  *ethstg.EthDataStore
}

// Cache may also have its storeage, this is the cache only store proxy, no storage.
func NewCacheOnlyStoreProxy() *StorageProxy {
	proxy := &StorageProxy{
		platform:   stgtypecommon.NewPlatform(),
		ethStorage: ethstg.NewParallelEthMemDataStore(), //ethstg.NewParallelEthMemDataStore(),
		execStorage: livestg.NewLiveStorage(
			nil,
			stgtypecommon.Codec{}.Encode,
			stgtypecommon.Codec{}.Decode,
		),
	}

	proxy.execCache = livecache.NewLiveCache(math.MaxUint64)
	return proxy
}

func NewMemDBStoreProxy() *StorageProxy {
	proxy := NewCacheOnlyStoreProxy()
	proxy.execStorage.SetDB(memdb.NewMemoryDB())
	return proxy
}

func NewLevelDBStoreProxy(dbpath string) *StorageProxy {
	proxy := &StorageProxy{
		platform:   stgtypecommon.NewPlatform(),
		ethStorage: ethstg.NewLevelDBDataStore(dbpath), //ethstg.NewParallelEthMemDataStore(),
		execCache:  livecache.NewLiveCache(math.MaxUint64),
		execStorage: livestg.NewLiveStorage(
			// memdb.NewMemoryDB(),
			ccbadger.NewBadgerDB(dbpath+"_badager"),
			// ccbadger.NewParaBadgerDB(dbpath+"_pbadager", common.Remainder),
			stgtypecommon.Codec{}.Encode,
			stgtypecommon.Codec{}.Decode,
		),
	}
	// proxy.execCache = livecache.NewLiveCache(math.MaxUint64)
	return proxy
}

// NewStoreProxyPersistentDB creates a new storage proxy with a persistent databases
// func NewTestLevelDBStoreProxy() *StorageProxy {
// 	return NewLevelDBStoreProxy("/tmp")
// }

func (this *StorageProxy) EnableCache() *StorageProxy {
	this.execCache.Enable()
	return this
}

func (this *StorageProxy) DisableCache() *StorageProxy {
	this.execCache.Disable()
	return this
}

func (this *StorageProxy) ClearExecCache() { this.execCache.Clear() }

func (this *StorageProxy) ExecCache() *livecache.LiveCache { return this.execCache }
func (this *StorageProxy) ExecStore() *livestg.LiveStorage { return this.execStorage } // Arcology storage

// Check if the key exists in th storage.
func (this *StorageProxy) ReadStorage(key string, T any) (any, error) {
	if v, ok := this.execCache.Get(key); ok { // Check the cache first
		return v, nil
	}
	return this.execStorage.Retrive(key, T)
}

func (this *StorageProxy) Retrive(key string, v any) (any, error) {
	return this.ReadStorage(key, v)
}

func (this *StorageProxy) EthStore() *ethstg.EthDataStore { return this.ethStorage } // Eth storage

func (this *StorageProxy) Preload(data []byte) any {
	return this.ethStorage.Preload(data)
}

// Check if the key exists in the source, which can be a cache or a storage.
func (this *StorageProxy) IfExists(key string) bool {
	if _, ok := this.execCache.Get(key); ok { // Check the cache first
		return true
	}
	return this.execStorage.IfExists(key)
}

// Directly inject the value into the storage, on for the concurrent container storage
func (this *StorageProxy) Inject(key string, v any) error {
	return this.execStorage.Inject(key, v)
}

// Get the stores that can be
func (this *StorageProxy) GetWriters() []intf.Writer[*statecell.StateCell] {
	return []intf.Writer[*statecell.StateCell]{
		livecache.NewLiveCacheWriter(this.execCache, -1, this.RemoveTransients),
		ethstorage.NewEthStorageWriter(this.ethStorage, -1, this.EthOnly),
		ccstorage.NewLiveStorageWriter(this.execStorage, -1, this.RemoveTransients),
	}
}

// Get the stores that can be
func (this *StorageProxy) SyncWriters() []intf.Writer[*statecell.StateCell] {
	return []intf.Writer[*statecell.StateCell]{
		livecache.NewLiveCacheWriter(this.execCache, -1, this.RemoveTransients),
	}
}

func (this *StorageProxy) AsyncWriters() []intf.Writer[*statecell.StateCell] {
	return []intf.Writer[*statecell.StateCell]{
		ethstorage.NewEthStorageWriter(this.ethStorage, -1, this.EthOnly),
		ccstorage.NewLiveStorageWriter(this.execStorage, -1, this.RemoveTransients),
	}
}

// Filter out the transitions that are not needed to be persisted.
func (this *StorageProxy) RemoveTransients(tran *statecell.StateCell) bool {
	// System paths only get reset if they are transient.
	if v := (*tran).Value(); v != nil && v.(intf.Type).TypeID() == commutative.PATH && v.(*commutative.Path).IsBlockBound() && this.platform.IsSysPath(*(*tran).GetPath()) {
		v.(*commutative.Path).Reset()
	}

	// Other transient transitions get no chance to be persisted.
	return !(*tran).IsBlockBound()
}

// Filter out the transitions that are not needed to be persisted.
func (this *StorageProxy) EthOnly(tran *statecell.StateCell) bool {
	return stgtypecommon.IsEthPath(*tran.GetPath())
}
