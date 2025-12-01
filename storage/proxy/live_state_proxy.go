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

	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
	"github.com/arcology-network/common-lib/crdt/commutative"
	statecell "github.com/arcology-network/common-lib/crdt/statecell"
	ccbadger "github.com/arcology-network/common-lib/storage/badger"
	"github.com/arcology-network/common-lib/storage/memdb"
	statecommon "github.com/arcology-network/state-engine/common"
	"github.com/arcology-network/state-engine/storage/ethstorage"
	ethstg "github.com/arcology-network/state-engine/storage/ethstorage"
	livebackend "github.com/arcology-network/state-engine/storage/execstorage/livebackend"
	livecache "github.com/arcology-network/state-engine/storage/execstorage/livecache"
	"github.com/ethereum/go-ethereum/triedb/hashdb"

	arcocodec "github.com/arcology-network/state-engine/storage/codec/arcocodec"
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
	platform    *statecommon.Platform
	execCache   *livecache.LiveCache // An object cache for the backend storage, only updated once at the end of the block.
	execBackend *livebackend.LiveStorage
	ethStorage  *ethstg.EthWorldState
}

// Cache may also have its storeage, this is the cache only store proxy, no storage.
// Used for testing and debugging.
func NewCacheOnlyStoreProxy() *StorageProxy {
	proxy := &StorageProxy{
		platform:   statecommon.NewPlatform(),
		ethStorage: ethstg.NewParallelEthMemDataStore(), //ethstg.NewParallelEthMemDataStore(),
		execBackend: livebackend.NewLiveStorage(
			nil,
			arcocodec.Codec{}.Encode,
			arcocodec.Codec{}.Decode,
		),
	}

	proxy.execCache = livecache.NewLiveCache(math.MaxUint64)
	return proxy
}

func NewMemDBStoreProxy() *StorageProxy {
	proxy := NewCacheOnlyStoreProxy()
	proxy.execBackend.SetBackend(memdb.NewMemoryDB())
	return proxy
}

func NewLevelDBStoreProxy(ethDBPath, execDBPath string, cacheCap uint64, cacheConfig *hashdb.Config) *StorageProxy {
	proxy := &StorageProxy{
		platform:   statecommon.NewPlatform(),
		ethStorage: ethstg.NewLevelDBDataStore(ethDBPath, cacheConfig), //ethstg.NewParallelEthMemDataStore(),
		execCache:  livecache.NewLiveCache(cacheCap),
		execBackend: livebackend.NewLiveStorage(
			// memdb.NewMemoryDB(),
			ccbadger.NewBadgerDB(execDBPath+"_badager"),
			// ccbadger.NewParaBadgerDB(dbpath+"_pbadager", common.Remainder),
			arcocodec.Codec{}.Encode,
			arcocodec.Codec{}.Decode,
		),
	}
	// proxy.execCache = livecache.NewLiveCache(math.MaxUint64)
	return proxy
}

// NewStoreProxyPersistentDB creates a new storage proxy with a persistent databases
// func NewTestLevelDBStoreProxy() *StorageProxy {
// 	return NewLevelDBStoreProxy("/tmp")
// }

func (this *StorageProxy) EnableCache()    { this.execCache.Enable() }
func (this *StorageProxy) DisableCache()   { this.execCache.Disable() }
func (this *StorageProxy) ClearExecCache() { this.execCache.Clear() }

func (this *StorageProxy) ExecCache() *livecache.LiveCache     { return this.execCache }
func (this *StorageProxy) ExecStore() *livebackend.LiveStorage { return this.execBackend } // Arcology storage
func (this *StorageProxy) EthStore() *ethstg.EthWorldState     { return this.ethStorage }  // Eth storage

// Check if the key exists in the execution storage.
func (this *StorageProxy) ReadStorage(key string, T any) (any, error) {
	if v, ok := this.execCache.Get(key); ok { // Check the cache first
		return v, nil
	}
	return this.execBackend.Retrieve(key, T)
}

func (this *StorageProxy) Retrieve(key string, v any) (any, error) {
	return this.ReadStorage(key, v)
}

func (this *StorageProxy) Preload(data []byte) any {
	return this.ethStorage.Preload(data)
}

// Check if the key exists in the source, which can be a cache or a storage.
func (this *StorageProxy) IfExists(key string) bool {
	if _, ok := this.execCache.Get(key); ok { // Check the cache first
		return true
	}
	return this.execBackend.IfExists(key)
}

// Directly inject the value into the storage, on for initializing concurrent container storage
func (this *StorageProxy) Inject(key string, v any) error {
	return this.execBackend.Inject(key, v)
}

// Get the stores that can be
func (this *StorageProxy) GetWriters() []crdtcommon.Writer[*statecell.StateCell] {
	return []crdtcommon.Writer[*statecell.StateCell]{
		livecache.NewLiveCacheWriter(this.execCache, -1, this.RemoveTransients),
		ethstorage.NewEthStorageWriter(this.ethStorage, -1, this.EthOnly),
		livebackend.NewLiveStorageWriter(this.execBackend, -1, this.RemoveTransients),
	}
}

// Get the stores that can be
func (this *StorageProxy) SyncWriters() []crdtcommon.Writer[*statecell.StateCell] {
	return []crdtcommon.Writer[*statecell.StateCell]{
		livecache.NewLiveCacheWriter(this.execCache, -1, this.RemoveTransients),
	}
}

func (this *StorageProxy) AsyncWriters() []crdtcommon.Writer[*statecell.StateCell] {
	return []crdtcommon.Writer[*statecell.StateCell]{
		ethstorage.NewEthStorageWriter(this.ethStorage, -1, this.EthOnly),
		livebackend.NewLiveStorageWriter(this.execBackend, -1, this.RemoveTransients),
	}
}

// Filter out the transitions that are not needed to be persisted.
func (this *StorageProxy) RemoveTransients(tran *statecell.StateCell) bool {
	// System paths only get reset if they are transient.
	if v := (*tran).Value(); v != nil && v.(crdtcommon.Type).TypeID() == commutative.PATH && v.(*commutative.Path).IsBlockBound() && this.platform.IsSysPath(*(*tran).GetPath()) {
		v.(*commutative.Path).Reset()
	}

	// Other transient transitions get no chance to be persisted.
	return !(*tran).IsBlockBound()
}

// Filter out the transitions that are not needed to be persisted.
func (this *StorageProxy) EthOnly(tran *statecell.StateCell) bool {
	return statecommon.ShouldPersistToEth(*tran.GetPath())
}
