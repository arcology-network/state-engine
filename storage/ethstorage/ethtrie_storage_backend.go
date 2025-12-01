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
	"errors"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/arcology-network/common-lib/exp/slice"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	ethmpt "github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/trienode"
	triedb "github.com/ethereum/go-ethereum/triedb"
	"github.com/ethereum/go-ethereum/triedb/hashdb"

	// ethmpt "github.com/ethereum/go-ethereum/trie"
	tridb "github.com/ethereum/go-ethereum/triedb"
)

// EthStroageBackend represents an Ethereum account with its associated
// storage trie and underlying databases.
type EthShardTrieDB struct {
	mainTrieDB   *tridb.Database    // main ETH trie db
	mainDBConfig *hashdb.Config     // config for the main ETH trie db
	diskShardDBs [16]ethdb.Database // 16 LevelDB shards
	encodedCache *fastcache.Cache   // A shared cache by shard DBs for encoded nodes
	err          error
}

// NewEthShardTrieMemDB creates a new EthShardTrieDB with in-memory databases for both
// the main trie and the shard databases.
func NewEthShardTrieMemDB(mainTrieDbConfig *hashdb.Config) *EthShardTrieDB {
	diskdbs := [16]ethdb.Database{}
	slice.Fill(diskdbs[:], rawdb.NewMemoryDatabase())
	db := triedb.NewParallelDatabase(diskdbs, nil)

	if mainTrieDbConfig == nil {
		mainTrieDbConfig = &hashdb.Config{CleanCacheSize: 1024 * 1024 * 100} // 100MB of the shared cache
	}
	return &EthShardTrieDB{
		mainDBConfig: mainTrieDbConfig,
		mainTrieDB:   db,
		diskShardDBs: diskdbs,
		encodedCache: fastcache.New(mainTrieDbConfig.CleanCacheSize),
		err:          nil,
	}
}

// NewEthShardLvlDB creates a new EthShardTrieDB with LevelDB databases for the
// shard databases stored in the specified directory.
func NewEthShardLvlDB(dir string, mainTrieDbConfig *hashdb.Config) (*EthShardTrieDB, error) {
	leveldb, err := rawdb.NewLevelDBDatabase(dir, 256, 16, "temp", false)
	if err != nil {
		return nil, err
	}

	diskdbs := [16]ethdb.Database{}
	slice.Fill(diskdbs[:], leveldb)
	if mainTrieDbConfig == nil {
		mainTrieDbConfig = &hashdb.Config{CleanCacheSize: 1024 * 1024 * 100} // 100MB of the shared cache
	}

	mainTrieDB := triedb.NewParallelDatabase(diskdbs, nil)
	return &EthShardTrieDB{
		mainDBConfig: mainTrieDbConfig,
		mainTrieDB:   mainTrieDB,
		diskShardDBs: diskdbs,
		encodedCache: fastcache.New(mainTrieDbConfig.CleanCacheSize),
		err:          nil,
	}, nil
}

func (this *EthShardTrieDB) MainTrieDB() *tridb.Database { return this.mainTrieDB }

// Selects the appropriate shard database based on the first byte of the key.
func (this *EthShardTrieDB) ShardFromKey(key string) ethdb.Database {
	if len(key) == 0 {
		return this.diskShardDBs[0]
	}
	return this.diskShardDBs[key[0]>>4]
}

// Commit finalizes the trie and persists the changes to the underlying databases.
func (this *EthShardTrieDB) Commit(trie *ethmpt.Trie, block uint64) (common.Hash, *ethmpt.Trie, error) {
	root, nodes, err := trie.Commit(false) // Finalized the trie
	if err != nil {
		return common.Hash{}, nil, errors.Join(errors.New("trie.Commit:"), err)
	}

	// this.mainTrieDB
	if nodes != nil {
		if err := this.mainTrieDB.Update(root, types.EmptyRootHash, block, trienode.NewWithNodeSet(nodes), nil); err != nil { // Move to DB dirty node set
			return common.Hash{}, nil, errors.Join(errors.New("ethdb.Update:"), err)
		}

		if err := this.mainTrieDB.Commit(root, false); err != nil {
			return common.Hash{}, nil, errors.Join(errors.New("ethdb.Commit:"), err)
		}
	}

	// Create a new trie for further updates.
	newTrie, err := ethmpt.NewParallel(ethmpt.TrieID(root), this.mainTrieDB)
	if err != nil {
		err = errors.Join(errors.New("ethmpt.NewParallel:"), err)
	}
	return root, newTrie, err
}
