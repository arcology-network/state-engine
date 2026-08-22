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

package ethstorage

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/ethdb/pebble"
	tridb "github.com/ethereum/go-ethereum/triedb"
	"github.com/ethereum/go-ethereum/triedb/hashdb"
)

// EthStroageBackend represents an Ethereum account with its associated
// storage trie and underlying databases.
type EthShardTrieDB struct {
	backend      *GethStateBackend
	mainTrieDB   *tridb.Database    // main ETH trie db
	mainDBConfig *hashdb.Config     // config for the main ETH trie db
	diskShardDBs [16]ethdb.Database // Compatibility views routed to the Pebble state database.
	err          error
}

// NewEthShardTrieMemDB creates a new EthShardTrieDB with in-memory databases for both
// the main trie and the shard databases.
func NewEthShardTrieMemDB(mainTrieDbConfig *hashdb.Config) *EthShardTrieDB {
	if mainTrieDbConfig == nil {
		mainTrieDbConfig = &hashdb.Config{CleanCacheSize: 1024 * 1024 * 100} // 100MB of the shared cache
	}
	backend, err := NewGethStateBackend(
		rawdb.NewMemoryDatabase(),
		&tridb.Config{HashDB: mainTrieDbConfig},
		common.Hash{},
	)
	if err != nil {
		return &EthShardTrieDB{mainDBConfig: mainTrieDbConfig, err: err}
	}
	return newEthShardTrieDB(backend, mainTrieDbConfig)
}

// NewEthShardPebbleDB creates a persistent Ethereum trie database backed by
// Pebble in the specified directory.
func NewEthShardPebbleDB(dir string, mainTrieDbConfig *hashdb.Config) (*EthShardTrieDB, error) {
	diskDB, err := pebble.New(dir, 256, 16, "state-engine/ethstorage", false)
	if err != nil {
		return nil, err
	}

	if mainTrieDbConfig == nil {
		mainTrieDbConfig = &hashdb.Config{CleanCacheSize: 1024 * 1024 * 100} // 100MB of the shared cache
	}
	disk := rawdb.NewDatabase(diskDB)
	backend, err := NewGethStateBackend(disk, &tridb.Config{HashDB: mainTrieDbConfig}, common.Hash{})
	if err != nil {
		_ = disk.Close()
		return nil, err
	}
	return newEthShardTrieDB(backend, mainTrieDbConfig), nil
}

func newEthShardTrieDB(backend *GethStateBackend, config *hashdb.Config) *EthShardTrieDB {
	db := &EthShardTrieDB{
		backend:      backend,
		mainTrieDB:   backend.TrieDB(),
		mainDBConfig: config,
	}
	for i := range db.diskShardDBs {
		db.diskShardDBs[i] = backend.disk
	}
	return db
}

func (this *EthShardTrieDB) MainTrieDB() *tridb.Database { return this.mainTrieDB }

// Selects the appropriate shard database based on the first byte of the key.
func (this *EthShardTrieDB) ShardFromKey(key string) ethdb.Database {
	if len(key) == 0 {
		return this.diskShardDBs[0]
	}
	return this.diskShardDBs[key[0]>>4]
}

func (this *EthShardTrieDB) Commit(block uint64) (common.Hash, error) {
	return this.backend.Commit(block)
}

func (this *EthShardTrieDB) StateDB() *state.StateDB         { return this.backend.StateDB() }
func (this *EthShardTrieDB) StateDatabase() *state.CachingDB { return this.backend.Database() }
func (this *EthShardTrieDB) Root() common.Hash               { return this.backend.Root() }
