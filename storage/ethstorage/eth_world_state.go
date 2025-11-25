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

	mapi "github.com/arcology-network/common-lib/exp/map"
	"github.com/arcology-network/common-lib/exp/slice"
	platform "github.com/arcology-network/state-engine/common"
	statecommon "github.com/arcology-network/state-engine/common"
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

	// accountCacheEnabled bool
	accountCache map[ethcommon.Address]*Account // Account cache holds the accountCache that are being accessed in the current cycle.

	backend *EthShardDB
	dbErr   error

	lock       sync.RWMutex
	blockRoots map[uint64][32]byte // lookup the root hash for a block number

	encoder func(string, any) ([]byte, error)
	decoder func(string, []byte, any) any
}

// LoadEthDataStore loads the trie from the database with the root provided.
func LoadEthDataStore(trieDB *triedb.Database, root [32]byte) (*EthWorldState, error) {
	trie, err := ethmpt.New(ethmpt.TrieID(root), trieDB)
	if err != nil || trie == nil {
		return nil, errors.Join(err, errors.New("Failed to load the trie from the database with the root provided!"))
	}

	diskdb := triedb.GetBackendDB(trieDB).DBs()
	backend := &EthShardDB{
		mainTrieDB:   trieDB,
		diskShardDBs: diskdb,
	}

	return NewEthDataStore(trie, backend), nil
}

func NewEthDataStore(trie *ethmpt.Trie, backend *EthShardDB) *EthWorldState {
	// trieDbConfig := &hashdb.Config{CleanCacheSize: 1024 * 1024 * 100} // 100MB of the shared cache
	return &EthWorldState{
		blockRoots: map[uint64][32]byte{},

		worldStateTrie: trie,
		backend:        backend,

		accountCache: map[ethcommon.Address]*Account{},
		encoder:      ethrlp.RlpCodec{}.Encode,
		decoder:      ethrlp.RlpCodec{}.Decode,
	}
}

// NewParallelEthMemDataStore creates a new EthWorldState with a memory database.
func NewParallelEthMemDataStore() *EthWorldState {
	shardMemDB := NewEthShardMemDB(&hashdb.Config{CleanCacheSize: 1024 * 1024 * 100})
	return NewEthDataStore(ethmpt.NewEmptyParallel(shardMemDB.mainTrieDB), shardMemDB)
}

// NewParallelEthMemDataStore creates a new EthWorldState with a memory database.
func NewParallelEthMemDataStoreWithSharedCache(trieDbConfig *hashdb.Config, cleanCache *fastcache.Cache) *EthWorldState {
	shardMemDB := NewEthShardMemDB(trieDbConfig)
	paraTrie := ethmpt.NewEmptyParallel(shardMemDB.mainTrieDB)
	return NewEthDataStore(paraTrie, shardMemDB)
}

// NewLevelDBDataStore creates a new EthWorldState with a leveldb database.
func NewLevelDBDataStore(dir string) *EthWorldState {
	shardLvlDB, err := NewEthShardLvlDB(dir, &hashdb.Config{CleanCacheSize: 1024 * 1024 * 100})
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
	acct, _ := this.GetAccount(ethcommon.BytesToAddress(addr), common.Reference(ethmpt.AccessListCache{}))
	if acct == nil {
		acct = NewAccountWithSharedCache( // Account not found, create a new account
			ethcommon.BytesToAddress(addr),
			// this.diskShardDBs,
			this.backend,
			EmptyAccountState(),
			// this.backend.encodedCache,
		)
	}

	// this.encodedCache) // empty account state
	return acct
}

func (this *EthWorldState) Hash(key string) []byte {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write([]byte(key))
	sum := hasher.Sum(nil)
	return sum
}

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
func (this *EthWorldState) IfExists(key string) bool {
	accesses := ethmpt.AccessListCache{}
	_, acctKey, suffix := platform.ParseAccountAddr(key)
	if len(suffix) == 0 {
		return false
	}

	if len(acctKey) == 0 {
		return false
	}

	acctBytes, err := hexutil.Decode(acctKey)
	if err != nil {
		return false
	}

	address := ethcommon.BytesToAddress(acctBytes)
	if v := this.accountCache[address]; v != nil {
		return len(key) == statecommon.ETH_ACCOUNT_FULL_LENGTH+1 || v.Has(key) // If the account has the key
	}

	// Not in cache, look up in the trie
	buffer, _ := this.worldStateTrie.ThreadSafeGet([]byte(address[:]), &accesses)
	if len(buffer) == 0 {
		return false // Not found
	}

	if len(suffix) == 0 {
		return true
	}

	var stateAccount types.StateAccount
	if err := rlp.DecodeBytes(buffer, &stateAccount); err != nil {
		return false
	}

	// address = ethcommon.BytesToAddress([]byte(key))
	return NewAccount(address, this.backend, stateAccount).Has(key) // Load the account but don't keep it in the cache.
}

// Get the account from the cache first, if not found, get it from the trie.
func (this *EthWorldState) GetAccount(address ethcommon.Address, accesses *ethmpt.AccessListCache) (*Account, error) {
	if len(address) > 0 {
		if v := this.accountCache[address]; v != nil { // Lookup in the cache first
			return v, nil
		}
		return this.GetAccountFromTrie(address, accesses)
	}
	return nil, errors.New("Invalid account: " + address.String())
}

// Get the account from the trie
func (this *EthWorldState) GetAccountFromTrie(address ethcommon.Address, accesses *ethmpt.AccessListCache) (*Account, error) {
	acctHash := crypto.Keccak256(address.Bytes()) // Hash the key string
	buffer, err := this.worldStateTrie.Get(acctHash)
	if err == nil && len(buffer) > 0 { // Not found
		var acctState types.StateAccount
		rlp.DecodeBytes(buffer, &acctState)

		stgTrie, err := ethmpt.New(ethmpt.TrieID(acctState.Root), this.EthDB()) // Get the storage trie
		if stgTrie != nil && err == nil {
			return &Account{
				addr:         address,
				StateAccount: acctState,
				code:         common.First(this.backend.diskShardDBs[0].Get(acctState.CodeHash)).([]byte), // code
				storageTrie:  stgTrie,
				// TrieDirty:    false,
				// ethdb:        this.backend.mainTrieDB,
				// diskdbShards: this.backend.diskShardDBs,
				err: nil,
			}, nil
		}
		return nil, err
	}
	return nil, err
}

// Skip the cache and get from the trie
func (this *EthWorldState) Retrieve(key string, T any) (any, error) {
	accesses := ethmpt.AccessListCache{}
	_, acctKey, _ := platform.ParseAccountAddr(key) // Get the address
	if len(acctKey) == 0 {
		return nil, errors.New("Invalid account: " + acctKey)
	}

	acctBytes, err := hexutil.Decode(acctKey)
	if err != nil {
		return nil, errors.New("Invalid account format: " + acctKey)
	}

	// Get the account the key belongs to.
	address := ethcommon.BytesToAddress(acctBytes)
	account, err := this.GetAccount(address, &accesses)

	if account != nil {
		return account.Retrieve(key, T) // Get the storage from the key
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

// Calculate the root hash for the world trie
func (this *EthWorldState) LatestWorldTrieRoot() [32]byte {
	return this.worldStateTrie.Hash() // Store the root hash for the block
}

func (this *EthWorldState) ShouldPersistToEth(blockNum uint64, dirtyAccounts []*Account) {
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
	dict := map[ethcommon.Address][]*Account{}
	for _, acct := range dirtyAccounts {
		if acct != nil {
			dict[acct.addr] = []*Account{}
		}
		dict[acct.addr] = append((dict[acct.addr]), acct)
	}
	_, uniqueDirties := mapi.KVs(dict)

	// Write the world trie
	slice.ParallelForeach(uniqueDirties, runtime.NumCPU(), func(_ int, dirties *[]*Account) {
		for _, acct := range *dirties { // There may be multiple updates for the same account.
			if err := (acct).Commit(blockNum); err != nil {
				panic(err)
			}
		}
	})

	var err error
	this.worldStateTrie, err = this.backend.commitTrieToDB(this.worldStateTrie, blockNum)
	// this.worldStateTrie, err = parallelcommitToEthDB(this.worldStateTrie, this.backend.mainTrieDB, blockNum) // Reload the trie for the next block
	if err != nil {
		this.dbErr = errors.Join(this.dbErr, err)
	}
}

func (this *EthWorldState) BatchRetrieve(keys []string, T []any) []any {
	values := make([]any, len(keys))
	for i := 0; i < len(keys); i++ {
		values[i], _ = this.Retrieve(keys[i], T[i])
	}
	return values
}

func (this *EthWorldState) DiskDBs() [16]ethdb.Database {
	return this.backend.diskShardDBs
}

// Place holders
func (this *EthWorldState) Root() [32]byte                                { return this.worldStateTrie.Hash() }
func (this *EthWorldState) Encoder(any) func(string, any) ([]byte, error) { return this.encoder }
func (this *EthWorldState) Decoder(any) func(string, []byte, any) any     { return this.decoder }
func (this *EthWorldState) EthDB() *triedb.Database                       { return this.backend.mainTrieDB }
func (this *EthWorldState) Trie() *ethmpt.Trie                            { return this.worldStateTrie }
func (this *EthWorldState) UpdateCacheStats([]any)                        {}
func (this *EthWorldState) Print()                                        {}
func (this *EthWorldState) CheckSum() [32]byte                            { return [32]byte{} }
func (this *EthWorldState) Query(string, func(string, string) bool) ([]string, [][]byte, error) {
	return nil, nil, nil
}

// func (this *EthWorldState) EnableAccountCache()                         { this.accountCacheEnabled = true }
// func (this *EthWorldState) DisableAccountCache()                        { this.accountCacheEnabled = false }
func (this *EthWorldState) AccountCache() map[ethcommon.Address]*Account { return this.accountCache }
func (this *EthWorldState) Clear()                                       {}

func (this *EthWorldState) Inject(key string, value any) error { return nil }

func (this *EthWorldState) GetRootHash(blockNum uint64) [32]byte {
	this.lock.RLock()
	defer this.lock.RUnlock()
	return this.blockRoots[blockNum]
}
