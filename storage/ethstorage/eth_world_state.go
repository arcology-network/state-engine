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
	"sync"

	"github.com/VictoriaMetrics/fastcache"
	libcommon "github.com/arcology-network/common-lib/common"
	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
	platform "github.com/arcology-network/state-engine/common"
	ethrlp "github.com/arcology-network/state-engine/storage/codec/ethcodec/rlp"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/ethereum/go-ethereum/triedb/hashdb"
	"golang.org/x/crypto/sha3"
)

// EthWorldState translates Arcology paths and CRDT values into canonical Geth
// StateDB operations. Geth owns account encoding, storage tries, code storage,
// root calculation, and persistence.
type EthWorldState struct {
	accountCache        map[ethcommon.Address]*Account
	accountCacheEnabled bool

	trieDB *EthShardTrieDB
	dbErr  error

	lock       sync.RWMutex
	blockRoots map[uint64][32]byte

	encoder func(string, any) ([]byte, error)
	decoder func(string, []byte, any) (any, error)
}

func LoadEthTrieByRoot(trieDB *triedb.Database, root [32]byte) (*EthWorldState, error) {
	backend, err := OpenGethStateBackend(trieDB, root)
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to load Ethereum state at the requested root"))
	}
	return NewEthDataStore(newEthShardTrieDB(backend, hashdb.Defaults)), nil
}

func NewEthWorldState(rootHash ethcommon.Hash, dir string, cacheConfig *hashdb.Config) *EthWorldState {
	shardDB, err := NewEthShardPebbleDB(dir, cacheConfig)
	if err != nil {
		return nil
	}
	if rootHash != (ethcommon.Hash{}) {
		backend, err := newGethStateBackend(shardDB.backend.disk, shardDB.mainTrieDB, rootHash, true)
		if err != nil {
			_ = shardDB.backend.Close()
			return nil
		}
		shardDB.backend = backend
	}
	return NewEthDataStore(shardDB)
}

func NewEthDataStore(trieDB *EthShardTrieDB) *EthWorldState {
	return &EthWorldState{
		blockRoots:   map[uint64][32]byte{},
		trieDB:       trieDB,
		accountCache: map[ethcommon.Address]*Account{},
		encoder:      ethrlp.RlpCodec{}.Encode,
		decoder:      ethrlp.RlpCodec{}.Decode,
	}
}

func NewParallelEthMemDataStore() *EthWorldState {
	return NewEthDataStore(NewEthShardTrieMemDB(&hashdb.Config{CleanCacheSize: 100 * 1024 * 1024}))
}

func NewParallelEthMemDataStoreSharedCache(config *hashdb.Config, cleanCache *fastcache.Cache) *EthWorldState {
	_ = cleanCache // Geth's caching database owns trie-node caching.
	return NewEthDataStore(NewEthShardTrieMemDB(config))
}

func NewPebbleDataStore(dir string, cacheConfig *hashdb.Config) *EthWorldState {
	shardDB, err := NewEthShardPebbleDB(dir, cacheConfig)
	if err != nil {
		return nil
	}
	return NewEthDataStore(shardDB)
}

func (s *EthWorldState) Preload(addr []byte) any {
	address := ethcommon.BytesToAddress(addr)
	account, _ := s.GetAccount(address, nil)
	if account == nil {
		account = NewAccountWithSharedCache(address, s.trieDB, EmptyAccountState())
	}
	return account
}

func (s *EthWorldState) Hash(key string) []byte {
	hasher := sha3.NewLegacyKeccak256()
	_, _ = hasher.Write([]byte(key))
	return hasher.Sum(nil)
}

func (s *EthWorldState) WorldStateTrie() state.Trie {
	accountTrie, err := s.accountTrie()
	if err != nil {
		s.dbErr = errors.Join(s.dbErr, err)
		return nil
	}
	return accountTrie
}

func (s *EthWorldState) accountTrie() (state.Trie, error) {
	if accountTrie := s.trieDB.StateDB().GetTrie(); accountTrie != nil {
		return accountTrie, nil
	}
	return s.trieDB.StateDatabase().OpenTrie(s.trieDB.Root())
}

func (s *EthWorldState) GetAccountProof(addr ethcommon.Address) ([]string, error) {
	if !s.trieDB.StateDB().Exist(addr) {
		return []string{}, nil
	}
	var proof proofList
	accountTrie, err := s.accountTrie()
	if err != nil {
		return nil, err
	}
	err = accountTrie.Prove(crypto.Keccak256(addr.Bytes()), &proof)
	return proof, err
}

func (s *EthWorldState) Has(path string) bool {
	_, accountKey, suffix := platform.ParseAccountAddr(path)
	if len(accountKey) == 0 || len(suffix) == 0 {
		return false
	}
	accountBytes, err := hexutil.Decode(accountKey)
	if err != nil {
		return false
	}
	account, err := s.GetAccount(ethcommon.BytesToAddress(accountBytes), nil)
	return err == nil && account != nil && account.Has(path)
}

func (s *EthWorldState) Get(path string) (any, error) {
	_, accountKey, _ := platform.ParseAccountAddr(path)
	if len(accountKey) == 0 {
		return nil, errors.New("invalid account: " + accountKey)
	}
	accountBytes, err := hexutil.Decode(accountKey)
	if err != nil {
		return nil, errors.New("invalid account format: " + accountKey)
	}
	account, err := s.GetAccount(ethcommon.BytesToAddress(accountBytes), nil)
	if account == nil {
		return nil, err
	}
	return account.Get(path)
}

func (s *EthWorldState) GetAs(path string, hint any) (any, error) {
	data, err := s.Get(path)
	if err != nil {
		return nil, err
	}
	if libcommon.IsType[crdtcommon.CRDT](data) {
		return data, nil
	}
	if buffer, ok := data.([]byte); ok && hint != nil {
		return s.decoder(path, buffer, hint)
	}
	return data, nil
}

// The second argument is retained as an unused compatibility parameter while
// callers migrate away from the removed AccessListCache type.
func (s *EthWorldState) GetAccount(address ethcommon.Address, _ any) (*Account, error) {
	if account := s.accountCache[address]; account != nil {
		account.refresh()
		s.openStorageTrie(account)
		return account, nil
	}
	stateDB := s.trieDB.StateDB()
	if !stateDB.Exist(address) {
		return nil, nil
	}
	accountState := types.StateAccount{
		Nonce:    stateDB.GetNonce(address),
		Balance:  stateDB.GetBalance(address).Clone(),
		Root:     stateDB.GetStorageRoot(address),
		CodeHash: stateDB.GetCodeHash(address).Bytes(),
	}
	account := NewAccount(address, s.trieDB, accountState)
	account.code = stateDB.GetCode(address)
	s.openStorageTrie(account)
	return account, nil
}

func (s *EthWorldState) openStorageTrie(account *Account) {
	accountTrie, err := s.accountTrie()
	if err != nil {
		return
	}
	storageTrie, err := s.trieDB.StateDatabase().OpenStorageTrie(
		s.trieDB.Root(), account.addr, account.Root, accountTrie,
	)
	if err == nil {
		account.storageTrie = storageTrie
	}
}

// WriteWorldTrie finalizes pending StateDB mutations. Geth performs independent
// storage-trie processing in parallel before updating the account trie.
func (s *EthWorldState) WriteWorldTrie(_ []*Account) [32]byte {
	return s.trieDB.StateDB().IntermediateRoot(false)
}

func (s *EthWorldState) persistToEthStore(blockNum uint64, _ []*Account) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	for len(s.blockRoots) >= 1024 {
		oldest := ^uint64(0)
		for block := range s.blockRoots {
			if block < oldest {
				oldest = block
			}
		}
		delete(s.blockRoots, oldest)
	}

	root, err := s.trieDB.Commit(blockNum)
	if err != nil {
		s.dbErr = errors.Join(s.dbErr, err)
		return s.dbErr
	}
	s.blockRoots[blockNum] = root
	return s.dbErr
}

func (s *EthWorldState) TrieDB() *EthShardTrieDB     { return s.trieDB }
func (s *EthWorldState) DiskDBs() [16]ethdb.Database { return s.trieDB.diskShardDBs }
func (s *EthWorldState) Error() error                { return s.dbErr }
func (s *EthWorldState) Root() [32]byte {
	if accountTrie := s.trieDB.StateDB().GetTrie(); accountTrie != nil {
		return accountTrie.Hash()
	}
	return s.trieDB.Root()
}

func (s *EthWorldState) Encoder(any) func(string, any) ([]byte, error) {
	return s.encoder
}

func (s *EthWorldState) Decoder(any) func(string, []byte, any) (any, error) {
	return s.decoder
}

func (s *EthWorldState) EthDB() *triedb.Database { return s.trieDB.mainTrieDB }
func (s *EthWorldState) Trie() state.Trie        { return s.WorldStateTrie() }
func (s *EthWorldState) UpdateCacheStats([]any)  {}
func (s *EthWorldState) Print()                  {}
func (s *EthWorldState) CheckSum() [32]byte      { return [32]byte{} }
func (s *EthWorldState) Query(string, func(string, []byte) bool) ([]string, [][]byte, error) {
	return nil, nil, nil
}

func (s *EthWorldState) EnableAccountCache()                          { s.accountCacheEnabled = true }
func (s *EthWorldState) DisableAccountCache()                         { s.accountCacheEnabled = false }
func (s *EthWorldState) AccountCache() map[ethcommon.Address]*Account { return s.accountCache }
func (s *EthWorldState) Clear()                                       {}
func (s *EthWorldState) Close() error                                 { return s.trieDB.backend.Close() }
func (s *EthWorldState) Inject(string, crdtcommon.CRDT) error         { return nil }

func (s *EthWorldState) GetRootHash(blockNum uint64) [32]byte {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.blockRoots[blockNum]
}
