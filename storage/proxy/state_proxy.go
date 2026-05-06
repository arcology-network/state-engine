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
	"errors"
	"math"

	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
	"github.com/arcology-network/common-lib/crdt/commutative"
	statecell "github.com/arcology-network/common-lib/crdt/statecell"
	cachedstore "github.com/arcology-network/common-lib/storage/cachedstore"
	stgcodec "github.com/arcology-network/common-lib/storage/codec"
	commonintf "github.com/arcology-network/common-lib/storage/interface"
	storageintf "github.com/arcology-network/common-lib/storage/interface"
	"github.com/arcology-network/common-lib/storage/memdb"
	pebbledb "github.com/arcology-network/common-lib/storage/pebble"

	statecommon "github.com/arcology-network/state-engine/common"

	"github.com/arcology-network/state-engine/storage/ethstorage"
	ethstg "github.com/arcology-network/state-engine/storage/ethstorage"
	livebackend "github.com/arcology-network/state-engine/storage/execstorage/livebackend"
	livecache "github.com/arcology-network/state-engine/storage/execstorage/livecache"
	"github.com/ethereum/go-ethereum/triedb/hashdb"

	filedb "github.com/arcology-network/common-lib/storage/filedb"
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
	*cachedstore.CachedStore[string, crdtcommon.CRDT, string, []byte] // An object cache for the backend storage, only updated once at the end of the block.
	ethStorage                                                        *ethstg.EthWorldState
	platform                                                          *statecommon.Platform
}

// Cache may also have its storeage, this is a cache only store proxy, no storage.
// Used for testing and debugging.
func NewCacheOnlyStoreProxy() *StorageProxy {
	proxy := &StorageProxy{
		platform:    statecommon.NewPlatform(),
		CachedStore: livecache.NewLiveCache(math.MaxUint64, nil, newLiveCacheCodec()),
		ethStorage:  ethstg.NewParallelEthMemDataStore(), //ethstg.NewParallelEthMemDataStore(),
	}
	return proxy
}

// NewMemDBStoreProxy creates a new storage proxy with in-memory databases for
// both execution and Ethereum storage.
func NewMemDBStoreProxy() *StorageProxy {
	// backend := initByteLiveStorage(memdb.NewMemoryDB())
	return &StorageProxy{
		platform:    statecommon.NewPlatform(),
		CachedStore: livecache.NewLiveCache(math.MaxUint64, memdb.NewMemoryDB(), newLiveCacheCodec()),
		ethStorage:  ethstg.NewParallelEthMemDataStore(), //ethstg.NewParallelEthMemDataStore(),
	}
}

// File DB for execution storage, LevelDB for Ethereum storage.
func NewFileStoreProxy(ethDBPath, execDBPath string, cacheCap uint64, cacheConfig *hashdb.Config) *StorageProxy {
	fileDB, err := filedb.NewFileDB(execDBPath, 2, 1)
	if err != nil {
		return nil
	}

	// backend := initByteLiveStorage(db)
	proxy := &StorageProxy{
		platform:    statecommon.NewPlatform(),
		CachedStore: livecache.NewLiveCache(cacheCap, fileDB, newLiveCacheCodec()),
		ethStorage:  ethstg.NewLevelDBDataStore(ethDBPath, cacheConfig), //ethstg.NewParallelEthMemDataStore(),
	}
	return proxy
}

// File DB for execution storage, LevelDB for Ethereum storage.
func NewPebbleDBProxy(ethDBPath, execDBPath string, cacheCap uint64, cacheConfig *hashdb.Config) *StorageProxy {
	pebbleDB, err := pebbledb.NewPebbleDB(execDBPath)
	if err != nil {
		return nil
	}

	// backend := initByteLiveStorage(db)
	proxy := &StorageProxy{
		platform:    statecommon.NewPlatform(),
		CachedStore: livecache.NewLiveCache(cacheCap, pebbleDB, newLiveCacheCodec()),
		ethStorage:  ethstg.NewLevelDBDataStore(ethDBPath, cacheConfig), //ethstg.NewParallelEthMemDataStore(),
	}
	return proxy
}

func (this *StorageProxy) EthStore() *ethstg.EthWorldState { return this.ethStorage } // Eth storage

func (this *StorageProxy) Preload(data []byte) any {
	return this.ethStorage.Preload(data)
}

// Get the stores that can be
func (this *StorageProxy) GetWriters() []storageintf.StoreWriter[*statecell.StateCell] {
	return append(this.SyncWriters(), this.AsyncWriters()...)
}

// Get the stores that can be
func (this *StorageProxy) SyncWriters() []storageintf.StoreWriter[*statecell.StateCell] {
	return []storageintf.StoreWriter[*statecell.StateCell]{
		livecache.NewLiveCacheWriter(this.CachedStore.Cache(), -1, this.NonTransientOnly),
	}
}

func (this *StorageProxy) AsyncWriters() []storageintf.StoreWriter[*statecell.StateCell] {
	return []storageintf.StoreWriter[*statecell.StateCell]{
		ethstorage.NewEthStorageWriter(this.ethStorage, -1, this.NonTransientOnly),
		livebackend.NewLiveStorageWriter(this.CachedStore.Backend(), -1, this.NonTransientOnly, this.CachedStore.Codec()),
	}
}

// Filter out the transitions that are not needed to be persisted.
func (this *StorageProxy) NonTransientOnly(tran *statecell.StateCell) bool {
	// System paths only get reset if they are transient.
	if v := (*tran).Value(); v != nil &&
		v.(crdtcommon.CRDT).TypeID() == commutative.PATH &&
		v.(*commutative.Path).IsBlockBound() &&
		this.platform.IsSysPath(*(*tran).GetPath()) {
		v.(*commutative.Path).Reset()
	}

	// Other transient transitions get no chance to be persisted.
	return !(*tran).IsBlockBound()
}

// Filter out the transitions that are not needed to be persisted.
func (*StorageProxy) EthOnly(tran *statecell.StateCell) bool {
	return statecommon.ShouldPersistToEth(*tran.GetPath())
}

// Allow all transitions to be written.
func (*StorageProxy) All(tran *statecell.StateCell) bool {
	return true
}

// Set the version for the storage proxy, only works for snapshot storages.
func (this *StorageProxy) SetVersion(_ [32]byte) error { return nil }

func (this *StorageProxy) DebugEnableCache()    {}
func (this *StorageProxy) DebugDisableCache()   {}
func (this *StorageProxy) DebugClearexecStore() { this.CachedStore.Cache().Clear() }

func initLiveStorage(db commonintf.BackendStore[string, []byte]) *cachedstore.CachedStore[string, crdtcommon.CRDT, string, []byte] {
	codec := stgcodec.NewStorageCodec(
		func(key string, value crdtcommon.CRDT) (string, []byte, error) {
			if value != nil {
				encoded, err := arcocodec.Codec{}.Encode(key, value)
				return key, encoded, err
			}
			return key, nil, nil
		},
		func(key string, value []byte) (string, crdtcommon.CRDT, error) {
			v := (arcocodec.Codec{}).Decode(key, value, nil)
			if v == nil {
				return "", nil, errors.New("failed to decode data")
			}
			return key, v.(crdtcommon.CRDT), nil
		},
	)

	return livebackend.NewLiveStorage(
		func(v crdtcommon.CRDT) uint64 {
			if v == nil {
				return 0
			}
			return v.MemSize()
		},
		db, // Backend storage
		codec,
	)
}

func initByteLiveStorage(db commonintf.BackendStore[string, []byte]) *cachedstore.CachedStore[string, []byte, string, []byte] {
	codec := stgcodec.NewStorageCodec(
		func(key string, value []byte) (string, []byte, error) { return key, value, nil },
		func(key string, value []byte) (string, []byte, error) { return key, value, nil },
	)

	return livebackend.NewLiveStorage(
		func(v []byte) uint64 { return uint64(len(v)) },
		db, // Backend storage
		codec,
	)
}

func newLiveCacheCodec() *stgcodec.StorageCodec[string, crdtcommon.CRDT, string, []byte] {
	return stgcodec.NewStorageCodec(
		func(key string, value crdtcommon.CRDT) (string, []byte, error) {
			encoded, err := arcocodec.Codec{}.Encode(key, value)
			return key, encoded, err
		},
		func(key string, value []byte) (string, crdtcommon.CRDT, error) {
			decoded := arcocodec.Codec{}.Decode(key, value, nil)
			if decoded == nil {
				return key, nil, nil
			}
			return key, decoded.(crdtcommon.CRDT), nil
		},
	)
}
