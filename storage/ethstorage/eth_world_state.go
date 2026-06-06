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

package ethstorage

import (
	"errors"
	"runtime"
	"sync"

	"github.com/VictoriaMetrics/fastcache"
	common "github.com/arcology-network/common-lib/common"
	libcommon "github.com/arcology-network/common-lib/common"
	crdtcommon "github.com/arcology-network/common-lib/crdt/common"

	mapi "github.com/arcology-network/common-lib/exp/map"
	"github.com/arcology-network/common-lib/exp/slice"
	platform "github.com/arcology-network/state-engine/common"
	ethrlp "github.com/arcology-network/state-engine/storage/codec/ethcodec/rlp"
	ethcommon "github.com/ethereum/go-ethereum/common"
	hexutil "github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	ethdb "github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	ethmpt "github.com/ethereum/go-ethereum/trie"
	triedb "github.com/ethereum/go-ethereum/triedb"
	"github.com/ethereum/go-ethereum/triedb/hashdb"
	"golang.org/x/crypto/sha3"
)

// EthWorldState manages the Ethereum world state, including the global state trie,
// per-account storage tries, and the shard-aware database backend.
//
// It is responsible for reading and updating account state, writing storage
// changes to the database, and tracking state roots for each block.
//
// The backend has 16 logical shards. In the current implementation all shard
// handles point to the same underlying LevelDB instance. LevelDB does not
// clearly document whether multiple independent database instances within the
// same process are supported or recommended, so only one instance is used for
// safety. The design, however, allows future physical sharding by assigning
// each shard to its own database directory and storage engine.
//
// EthWorldState also keeps an optional account cache for accounts accessed during
// the current execution cycle, and uses pluggable encoders/decoders for value
// serialization.

type EthWorldState struct {
	worldStateTrie *ethmpt.Trie

	// Account cache holds the accountCache that are being accessed in the current cycle.
	accountCache        map[ethcommon.Address]*Account
	accountCacheEnabled bool

	trieDB *EthShardTrieDB
	dbErr  error

	lock       sync.RWMutex
	blockRoots map[uint64][32]byte // lookup the root hash for a block number

	encoder func(string, any) ([]byte, error)
	decoder func(string, []byte, any) (any, error)
}

// LoadEthTrieByRoot loads the trie from the database with the root provided.
func LoadEthTrieByRoot(trieDB *triedb.Database, root [32]byte) (*EthWorldState, error) {
	trie, err := ethmpt.New(ethmpt.TrieID(root), trieDB)
	if err != nil || trie == nil {
		return nil, errors.Join(err, errors.New("Failed to load the trie from the database with the root provided!"))
	}

	diskdb := triedb.GetBackendDB(trieDB).DBs()
	backend := &EthShardTrieDB{
		mainTrieDB:   trieDB,
		diskShardDBs: diskdb,
	}

	return NewEthDataStore(trie, backend), nil
}

func NewEthWorldState(rootHash ethcommon.Hash, dir string, cacheConfig *hashdb.Config) *EthWorldState {
	// &hashdb.Config{CleanCacheSize: 1024 * 1024 * 100}
	shardLvlDB, err := NewEthShardLvlDB(dir, cacheConfig)
	if err != nil {
		return nil
	}

	// Initialize an empty that can be accessed in parallel.
	paraTrie := ethmpt.NewEmptyParallel(shardLvlDB.mainTrieDB) // A new empty trie
	return NewEthDataStore(paraTrie, shardLvlDB)
}

func NewEthDataStore(trie *ethmpt.Trie, trieDB *EthShardTrieDB) *EthWorldState {
	return &EthWorldState{
		blockRoots: map[uint64][32]byte{},

		worldStateTrie: trie,
		trieDB:         trieDB,

		accountCache: map[ethcommon.Address]*Account{},
		encoder:      ethrlp.RlpCodec{}.Encode,
		decoder:      ethrlp.RlpCodec{}.Decode,
	}
}

// NewParallelEthMemDataStore creates a new EthWorldState with a shard memory database
// for its backend.
func NewParallelEthMemDataStore() *EthWorldState {
	shardMemDB := NewEthShardTrieMemDB(&hashdb.Config{CleanCacheSize: 1024 * 1024 * 100})
	return NewEthDataStore(ethmpt.NewEmptyParallel(shardMemDB.mainTrieDB), shardMemDB)
}

// NewParallelEthMemDataStoreSharedCache creates a new EthWorldState with a memory database, its
// trie database uses the shared cache provided.
func NewParallelEthMemDataStoreSharedCache(trieDbConfig *hashdb.Config, cleanCache *fastcache.Cache) *EthWorldState {
	shardMemDB := NewEthShardTrieMemDB(trieDbConfig)
	paraTrie := ethmpt.NewEmptyParallel(shardMemDB.mainTrieDB)
	return NewEthDataStore(paraTrie, shardMemDB)
}

// NewLevelDBDataStore creates a new EthWorldState with a leveldb database.
func NewLevelDBDataStore(dir string, cacheConfig *hashdb.Config) *EthWorldState {
	// &hashdb.Config{CleanCacheSize: 1024 * 1024 * 100}
	shardLvlDB, err := NewEthShardLvlDB(dir, cacheConfig)
	if err != nil {
		return nil
	}
	paraTrie := ethmpt.NewEmptyParallel(shardLvlDB.mainTrieDB) // A new empty trie
	return NewEthDataStore(paraTrie, shardLvlDB)
}

// Preload loads an existing account from the trie and the disk db.
// If the account is not found, it creates a new account with default account state and shared cache.
func (this *EthWorldState) Preload(addr []byte) any {
	// AccessListCache doesn't serve any purpose for now. It is only a place holder, the parallelized trie update requires its presence.
	acct, _ := this.GetAccount(ethcommon.BytesToAddress(addr), libcommon.Reference(ethmpt.AccessListCache{}))
	if acct == nil {
		acct = NewAccountWithSharedCache( // Account not found, create a new account
			ethcommon.BytesToAddress(addr),
			// this.diskShardDBs,
			this.trieDB,
			EmptyAccountState(),
			// this.backend.encodedCache,
		)
	}
	return acct
}

func (this *EthWorldState) Hash(key string) []byte {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write([]byte(key))
	sum := hasher.Sum(nil)
	return sum
}

// Get the world state trie
func (this *EthWorldState) WorldStateTrie() *ethmpt.Trie {
	return this.worldStateTrie
}

// Get the account proof for the address provided.
func (this *EthWorldState) GetAccountProof(addr ethcommon.Address) ([]string, error) {
	addrHash := crypto.Keccak256(addr.Bytes())

	var proof proofList
	if trie, _ := this.worldStateTrie.Get(addrHash); len(trie) > 0 {
		err := this.worldStateTrie.Prove(addrHash, &proof)
		// VerifyProof(this.worldStateTrie, proof, addr[:]) // Debugging only
		return proof, err
	}
	return []string{}, nil
}

// Get the account from the cache first, if not found, get it from the trie.
func (this *EthWorldState) Has(key string) bool {
	_, acctKey, suffix := platform.ParseAccountAddr(key)
	if len(acctKey) == 0 {
		return false
	}

	// Should have a suffix like /balance, /nonce, /code, or /storage/ for storage keys.
	// or a "/" for account itself.
	if len(suffix) == 0 {
		return false
	}

	acctBytes, err := hexutil.Decode(acctKey)
	if err != nil {
		return false
	}

	address := ethcommon.BytesToAddress(acctBytes)

	// Check if the account is in the cache
	if acct := this.accountCache[address]; acct != nil {
		return acct.Has(key) // If the account has the key
	}

	// Not in cache, look up in the trie
	buffer, _ := this.worldStateTrie.ThreadSafeGet(crypto.Keccak256(address[:]), &ethmpt.AccessListCache{})
	if len(buffer) == 0 {
		return false // Not found
	}

	// Found in the trie.
	var stateAccount types.StateAccount
	if err := rlp.DecodeBytes(buffer, &stateAccount); err != nil {
		return false
	}

	acct := NewAccount(address, this.trieDB, stateAccount)
	acct.storageTrie, err = LoadTrie(this.trieDB.mainTrieDB, stateAccount.Root)
	if err != nil {
		return false
	}

	// key can be an account or a storage key under the account.
	// so we need to make that distinction here. The current code does not do that.
	// it assume every key is a storage key under the account, not the account itself.
	return acct.Has(key)
}

func (this *EthWorldState) Get(path string) (any, error) {
	_, acctKey, _ := platform.ParseAccountAddr(path) // Get the address
	if len(acctKey) == 0 {
		return nil, errors.New("Invalid account: " + acctKey)
	}

	acctBytes, err := hexutil.Decode(acctKey)
	if err != nil {
		return nil, errors.New("Invalid account format: " + acctKey)
	}

	// Get the account the key belongs to.
	address := ethcommon.BytesToAddress(acctBytes)
	account, err := this.GetAccount(address, new(ethmpt.AccessListCache))

	if account != nil {
		return account.Get(path) // Get the storage from the key
	}
	return nil, err
}

func (this *EthWorldState) GetAs(path string, hint any) (any, error) {
	data, err := this.Get(path)
	if err != nil {
		return nil, err
	}

	if libcommon.IsType[crdtcommon.CRDT](data) {
		return data, nil
	}

	if buffer, ok := data.([]byte); ok && hint != nil {
		return this.decoder(path, buffer, hint)
	}
	return data, nil
}

// Get the account from the cache first, if not found, get it from the trie.
func (this *EthWorldState) GetAccount(address ethcommon.Address, _ *ethmpt.AccessListCache) (*Account, error) {
	if len(address) == 0 {
		return nil, errors.New("Invalid account: " + address.String())
	}

	// Lookup in the cache first
	if v := this.accountCache[address]; v != nil {
		return v, nil
	}

	acctHash := crypto.Keccak256(address.Bytes()) // Hash the key string
	buffer, err := this.worldStateTrie.Get(acctHash)
	if len(buffer) == 0 || err != nil {
		return nil, err // Not found
	}

	var acctState types.StateAccount
	if err := rlp.DecodeBytes(buffer, &acctState); err != nil {
		return nil, err
	}

	stgTrie, err := ethmpt.New(ethmpt.TrieID(acctState.Root), this.EthDB()) // Get the storage trie
	if stgTrie != nil && err == nil {
		return &Account{
			addr:         address,
			StateAccount: acctState,
			code:         libcommon.First(this.trieDB.diskShardDBs[0].Get(acctState.CodeHash)).([]byte), // code
			storageTrie:  stgTrie,
			err:          nil,
		}, nil
	}
	return nil, err
}

// The WriteWorldTrie writes the updated accounts to the world trie.
func (this *EthWorldState) WriteWorldTrie(dirtyAccounts []*Account) [32]byte {
	encodedAddrs, encodedAcct := [][]byte{}, [][]byte{} // Encode the account key and values
	common.ParallelExecute(
		func() { // Account keys
			encodedAddrs = slice.Transform(dirtyAccounts, func(_ int, acct *Account) []byte {
				return crypto.Keccak256(acct.addr[:]) // Hash the account address
			})
		},
		func() { // Encode the account content.
			encodedAcct = slice.Transform(dirtyAccounts, func(_ int, acct *Account) []byte {
				return acct.Encode() // Encode the account
			})
		},
	)

	// Write the world tree and return the first error if any.
	errs := this.worldStateTrie.ParallelUpdate(encodedAddrs, encodedAcct)
	this.dbErr = errors.Join(this.dbErr, errors.Join(errs...))
	return this.worldStateTrie.Hash()
}

// ShouldPersistToEth determines whether to persist the current state to Ethereum-compatible storage.
func (this *EthWorldState) persistToEthStore(blockNum uint64, dirtyAccounts []*Account) error {
	this.lock.Lock()
	defer this.lock.Unlock()

	// It Only keeps the last 1024 root hashes. Remove the oldest root hash if the root hash map is full.
	for len(this.blockRoots) >= 1024 {
		_, minBlockNum := slice.Min(mapi.Keys(this.blockRoots))
		delete(this.blockRoots, minBlockNum)
	}

	// Write the world root hash to the root hash map.
	this.blockRoots[blockNum] = this.worldStateTrie.Hash() // Store the root hash for the block

	// dirtyAccounts may contain the same account multiple times, updating them in parallel directly may cause concurrency issues.
	// So, we need to merge the accounts and put them in slices, identical accounts will be put together in the same slice.
	uniqueAccts := map[ethcommon.Address][]*Account{}
	for _, acct := range dirtyAccounts {
		if acct == nil {
			continue // Skip the nil accounts.
		}

		if uniqueAccts[acct.addr] == nil {
			uniqueAccts[acct.addr] = []*Account{}
		}
		uniqueAccts[acct.addr] = append((uniqueAccts[acct.addr]), acct)
	}
	_, uniqueDirties := mapi.KVs(uniqueAccts)

	// Write the world trie
	slice.ParallelForeach(uniqueDirties, runtime.NumCPU(), func(_ int, dirties *[]*Account) {
		for _, acct := range *dirties { // There may be multiple updates for the same account.
			if err := (acct).Commit(blockNum); err != nil {
				this.dbErr = errors.Join(this.dbErr, err)
			}
		}
	})

	// this.backend.Commit(this.worldStateTrie, blockNum)

	// var err error
	var err error
	if _, this.worldStateTrie, err = this.trieDB.Commit(this.worldStateTrie, blockNum); err != nil {
		this.dbErr = errors.Join(this.dbErr, err)
	}
	return this.dbErr
}

func (this *EthWorldState) TrieDB() *EthShardTrieDB     { return this.trieDB }
func (this *EthWorldState) DiskDBs() [16]ethdb.Database { return this.trieDB.diskShardDBs }

// Place holders
func (this *EthWorldState) Root() [32]byte                                     { return this.worldStateTrie.Hash() }
func (this *EthWorldState) Encoder(any) func(string, any) ([]byte, error)      { return this.encoder }
func (this *EthWorldState) Decoder(any) func(string, []byte, any) (any, error) { return this.decoder }
func (this *EthWorldState) EthDB() *triedb.Database                            { return this.trieDB.mainTrieDB }
func (this *EthWorldState) Trie() *ethmpt.Trie                                 { return this.worldStateTrie }
func (this *EthWorldState) UpdateCacheStats([]any)                             {}
func (this *EthWorldState) Print()                                             {}
func (this *EthWorldState) CheckSum() [32]byte                                 { return [32]byte{} }
func (this *EthWorldState) Query(string, func(string, []byte) bool) ([]string, [][]byte, error) {
	return nil, nil, nil
}

func (this *EthWorldState) EnableAccountCache()                          { this.accountCacheEnabled = true }
func (this *EthWorldState) DisableAccountCache()                         { this.accountCacheEnabled = false }
func (this *EthWorldState) AccountCache() map[ethcommon.Address]*Account { return this.accountCache }
func (this *EthWorldState) Clear()                                       {}
func (this *EthWorldState) Close() error                                 { return this.trieDB.mainTrieDB.Close() }

func (this *EthWorldState) Inject(key string, value crdtcommon.CRDT) error { return nil }
func (this *EthWorldState) GetRootHash(blockNum uint64) [32]byte {
	this.lock.RLock()
	defer this.lock.RUnlock()
	return this.blockRoots[blockNum]
}
