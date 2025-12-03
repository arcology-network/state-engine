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

	"github.com/cespare/xxhash/v2"

	slice "github.com/arcology-network/common-lib/exp/slice"
	cache "github.com/arcology-network/common-lib/storage/cache"
	commonintf "github.com/arcology-network/common-lib/storage/interface"
)

type LiveStorage struct {
	db    commonintf.PersistentStorage
	cache *cache.ReadCache[string, any]

	encoder func(string, any) ([]byte, error)
	decoder func(string, []byte, any) any
}

// numShards uint64, isNil func(V) bool, hasher func(K) uint64, cachePolicy *policy.CachePolicy
func NewLiveStorage(
	db commonintf.PersistentStorage,
	encoder func(string, any) ([]byte, error),
	decoder func(string, []byte, any) any,
) *LiveStorage {
	LiveStorage := &LiveStorage{
		cache: cache.NewReadCache(
			16,
			func(T any) bool {
				return T == nil
			},
			func(k string) uint64 {
				return uint64(xxhash.Sum64String(k))
			},
		),

		db:      db,
		encoder: encoder,
		decoder: decoder,
	}

	LiveStorage.cache.Disable()
	return LiveStorage
}

// Placeholder only
func (this *LiveStorage) Preload(data []byte) any                       { return nil }
func (this *LiveStorage) Cache(any) any                                 { return this.cache }
func (this *LiveStorage) Encoder(any) func(string, any) ([]byte, error) { return this.encoder }
func (this *LiveStorage) Decoder(any) func(string, []byte, any) any     { return this.decoder }

func (this *LiveStorage) GetBackend() commonintf.PersistentStorage   { return this.db }
func (this *LiveStorage) SetBackend(db commonintf.PersistentStorage) { this.db = db }

// Write directly to the local cache. This only happends
// when we are doing special initialization of the storage.
func (this *LiveStorage) Write(key string, v any) error {
	this.cache.Set(key, v)
	encoded, _ := this.encoder(key, v)
	return this.db.BatchSet([]string{key}, [][]byte{encoded})
}

// Batch Write directly to the local cache.
func (this *LiveStorage) BatchWrite(keys []string, values []any) error {
	this.cache.BatchSet(keys, values) // update the local cache
	encoded := make([][]byte, len(keys))
	for i := 0; i < len(keys); i++ {
		encoded[i], _ = this.encoder(keys[i], values[i])
	}
	return this.db.BatchSet(keys, encoded)
}

// No access tracking
func (this *LiveStorage) IfExists(key string) bool {
	v, _ := this.Retrieve(key, nil)
	return v != nil
}

// Get from the underlying storage directly.
func (this *LiveStorage) ReadStorage(key string, T any) (any, error) {
	if this.db == nil {
		return nil, errors.New("Error: DB not found")
	}

	bytes, err := this.db.Get(key) // Get from the underlying storage
	if len(bytes) > 0 && err == nil {
		if T == nil {
			return bytes, nil
		}
		return this.decoder(key, bytes, T), nil
	}
	return nil, err
}

// Get from the local cache first, then from the underlying storage.
func (this *LiveStorage) Retrieve(key string, T any) (any, error) {
	// Read from the local cache first
	if v, _ := this.cache.Get(key); v != nil {
		return *v, nil
	}

	v, err := this.ReadStorage(key, T)
	if err == nil && T != nil {
		this.cache.Set(key, v) //update to the local cache and add all the missing values to the cache
	}
	return v, err
}

func (this *LiveStorage) BatchRetrieve(keys []string, T []any) []any {
	values, _ := this.cache.BatchGet(keys) // From the local cache first
	if slice.Count(values, nil) == 0 {     // All found
		return values
	}

	/* Find the values missing from the local cache*/
	queryKeys, queryIdxes := make([]string, 0, len(keys)), make([]int, 0, len(keys))
	for i := 0; i < len(keys); i++ {
		if values[i] == nil {
			queryKeys = append(queryKeys, keys[i])
			queryIdxes = append(queryIdxes, i)
		}
	}

	if data, err := this.db.BatchGet(queryKeys); err == nil { // search for the values that aren't in the cache
		for i, idx := range queryIdxes {
			if data[i] != nil {
				if len(T) > 0 {
					values[idx] = this.decoder(queryKeys[i], data[i], T[i])
				} else {
					values[idx] = this.decoder(queryKeys[i], data[i], nil)
				}
			}
		}
		this.cache.BatchSet(keys, values) //update to the local cache and add all the missing values to the cache
	}
	return values
}
