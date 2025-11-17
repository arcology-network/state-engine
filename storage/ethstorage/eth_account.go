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
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/arcology-network/common-lib/codec"
	common "github.com/arcology-network/common-lib/common"
	"github.com/arcology-network/common-lib/exp/slice"
	stgcommon "github.com/arcology-network/storage-committer/common"
	commutative "github.com/arcology-network/storage-committer/type/commutative"
	noncommutative "github.com/arcology-network/storage-committer/type/noncommutative"
	statecell "github.com/arcology-network/storage-committer/type/statecell"

	platform "github.com/arcology-network/storage-committer/type/common"
	ethcommon "github.com/ethereum/go-ethereum/common"
	hexutil "github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	ethmpt "github.com/ethereum/go-ethereum/trie"
	tridb "github.com/ethereum/go-ethereum/triedb"
	"github.com/ethereum/go-ethereum/triedb/hashdb"
	"github.com/holiman/uint256"
	"golang.org/x/crypto/sha3"
)

type Account struct {
	addr ethcommon.Address

	ethtypes.StateAccount
	code []byte

	storageTrie  *ethmpt.Trie // account storage trie
	StorageDirty bool

	ethdb        *tridb.Database
	diskdbShards [16]ethdb.Database
	err          error
}

// The diskdbs need to able to handle concurrent accesses themselve
func NewAccount(addr ethcommon.Address, diskdbs [16]ethdb.Database, state types.StateAccount) *Account {
	ethdb, trie, err := LoadTrie(diskdbs, state.Root)
	return &Account{
		addr:         addr,
		storageTrie:  trie,
		StorageDirty: false,
		ethdb:        ethdb,
		diskdbShards: diskdbs,
		StateAccount: state,
		err:          err,
	}
}

// The diskdbs need to able to handle concurrent accesses themselve
func NewAccountWithSharedCache(addr ethcommon.Address, diskdbs [16]ethdb.Database, state types.StateAccount, dbConfig *hashdb.Config, sharedCache *fastcache.Cache) *Account {
	ethdb, trie, err := LoadTrieWithSharedCache(diskdbs, state.Root, dbConfig, sharedCache)
	return &Account{
		addr:         addr,
		storageTrie:  trie,
		StorageDirty: false,
		ethdb:        ethdb,
		diskdbShards: diskdbs,
		StateAccount: state,
		err:          err,
	}
}

func EmptyAccountState() types.StateAccount {
	return ethtypes.StateAccount{
		Nonce:    0,
		Balance:  uint256.NewInt(0),
		Root:     types.EmptyRootHash,
		CodeHash: codec.Bytes32(crypto.Keccak256Hash(nil)).Encode(),
	}
}

func LoadTrie(diskdbs [16]ethdb.Database, root ethcommon.Hash) (*tridb.Database, *trie.Trie, error) {
	ethdb := tridb.NewParallelDatabase(diskdbs, nil)
	trie, err := ethmpt.NewParallel(ethmpt.TrieID(root), ethdb)
	return ethdb, trie, err
}

func LoadTrieWithSharedCache(diskdbs [16]ethdb.Database, root ethcommon.Hash, dbConfig interface{}, sharedCache *fastcache.Cache) (*tridb.Database, *trie.Trie, error) {
	ethdb := tridb.NewParallelDatabaseWithSharedCache(diskdbs, sharedCache, nil)
	trie, err := ethmpt.NewParallel(ethmpt.TrieID(root), ethdb)
	return ethdb, trie, err
}

func (this *Account) GetState(key [32]byte) []byte {
	data, _ := this.storageTrie.Get(key[:])
	return data
}

func (this *Account) SetAddress(addr ethcommon.Address) { this.addr = addr }
func (this *Account) Address() ethcommon.Address        { return this.addr }
func (this *Account) Trie() *ethmpt.Trie                { return this.storageTrie }

func (this *Account) GetStorageRoot() [32]byte {
	if this.storageTrie == nil {
		return types.EmptyRootHash
	}
	return this.storageTrie.Hash()
}

func (this *Account) GetCodeHash() [32]byte {
	if this.storageTrie == nil {
		return crypto.Keccak256Hash(nil) // so the codeHash is the hash of an empty byteslice.
	}
	return codec.Bytes32{}.Decode(this.CodeHash).(codec.Bytes32)
}

// The function is used to prove the storage of an account. It only works with
// NATIVE storage for now.
func (this *Account) IsStorageProvable(key string) ([]byte, []string, error) {
	decoded, _, _ := decodeHash(key)
	keyBytes := crypto.Keccak256(decoded[:])
	data, err := this.storageTrie.Get(keyBytes) // Get the storage value

	var proofs proofList
	if len(data) > 0 && err == nil {
		if err := this.storageTrie.Prove(keyBytes, &proofs); err != nil { // Get the storage proof and save it to the proof DB
			return nil, proofs, err
		}
	} else {
		return nil, proofs, errors.New("Failed to find the proof")
	}

	proofdb, err := ProofArrayToDB(proofs)
	if err != nil {
		return nil, proofs, err
	}

	v, err := ethmpt.VerifyProof(this.StateAccount.Root, keyBytes, proofdb)
	if err != nil || !bytes.Equal(v, data) {
		return nil, proofs, errors.New("Failed to find the proof for storage key: " + key)
	}
	return data, proofs, nil
}

func (this *Account) DB(key string) ethdb.Database {
	if len(key) == 0 {
		return this.diskdbShards[0]
	}
	return this.diskdbShards[key[0]>>4]
}

// The function parses the key in a forward slash separated format into a hex
// string that can be accepted by the storage trie.
func (this *Account) ToStorageKey(key string) string {
	if k := platform.GetPathUnder(key, "/storage/native/"); len(k) > 0 {
		kstr, err := hexutil.Decode(k) // For native storage, the key is hex encoded.
		if err != nil {
			panic(err)
		}

		kstr = this.Hash(kstr) // For native storage, the key is the hash of the key with prefix.
		return string(kstr)
	}
	return string(this.Hash([]byte(key))) // For non-native storage, the key is the hash of the key with prefix.
}

func (this *Account) Has(key string) bool {
	if strings.HasSuffix(key, "/balance") || strings.HasSuffix(key, "/nonce") {
		return true
	}

	if strings.HasSuffix(key, "/code") {
		return len(this.code) > 0
	}

	buffer, _ := this.storageTrie.Get([]byte(this.ToStorageKey(key)))
	return len(buffer) > 0
}

func (this *Account) Retrive(key string, T any) (interface{}, error) {
	if strings.HasSuffix(key, "/balance") {
		balance, _ := uint256.FromBig(this.StateAccount.Balance.ToBig())
		v := commutative.NewUnboundedU256()
		v.SetValue(*balance)
		return v, nil
	}

	if strings.HasSuffix(key, "/nonce") {
		v := commutative.NewUnboundedUint64()
		v.SetValue(this.StateAccount.Nonce)
		return v, nil
	}

	if strings.HasSuffix(key, "/code") {
		var err error
		if this.code == nil {
			if this.code, err = this.DB(key).Get(this.CodeHash); err != nil {
				return nil, err
			}
		}
		return noncommutative.NewBytes(this.code), nil
	}

	k := this.ToStorageKey(key)
	buffer, err := this.storageTrie.Get([]byte(k))
	if len(buffer) == 0 {
		return nil, nil
	}

	if T == nil { // A deletion
		return T, nil
	}

	return T.(stgcommon.Type).StorageDecode(key, buffer), err
}

func (this *Account) UpdateAccountTrie(keys []string, typedVals []stgcommon.Type) error {
	if pos, _ := slice.FindFirstIf(keys, func(_ int, k string) bool { return len(k) == stgcommon.ETH_ACCOUNT_FULL_LENGTH+1 }); pos >= 0 {
		slice.RemoveAt(&keys, pos)
		slice.RemoveAt(&typedVals, pos)
	}

	if pos, _ := slice.FindFirstIf(keys, func(_ int, k string) bool { return strings.HasSuffix(k, "/nonce") }); pos >= 0 {
		this.Nonce = typedVals[pos].Value().(uint64)
		slice.RemoveAt(&keys, pos)
		slice.RemoveAt(&typedVals, pos)
	}

	if pos, _ := slice.FindFirstIf(keys, func(_ int, k string) bool { return strings.HasSuffix(k, "/balance") }); pos >= 0 {
		balance := typedVals[pos].Value().(uint256.Int)
		this.Balance = balance.Clone()
		slice.RemoveAt(&keys, pos)
		slice.RemoveAt(&typedVals, pos)
	}

	if pos, _ := slice.FindFirstIf(keys, func(_ int, k string) bool { return strings.HasSuffix(k, "/code") }); pos >= 0 {
		this.code = typedVals[pos].Value().(codec.Bytes)
		this.StateAccount.CodeHash = this.Hash(this.code)
		if err := this.DB(keys[pos]).Put(this.CodeHash, this.code); err != nil { // Save to DB directly, only for code
			return err // failed to save the code
		}
		slice.RemoveAt(&keys, pos)
		slice.RemoveAt(&typedVals, pos)
	}
	this.StorageDirty = len(keys) > 0

	// Encode the keys
	numThd := common.IfThen(len(keys) < 1024, 4, 8)
	encodedKeys := slice.ParallelTransform(keys, numThd, func(i int, _ string) []byte {
		return []byte(this.ToStorageKey(keys[i])) // Remove the prefix to get the keys.
	})

	// Encode the values
	encodedVals := slice.ParallelTransform(typedVals, numThd, func(i int, _ stgcommon.Type) []byte {
		return common.IfThenDo1st(typedVals[i] != nil, func() []byte {
			return typedVals[i].StorageEncode(keys[i])
		}, []byte{})
	})

	slice.SortBy1st(encodedKeys, encodedVals, func(v0, v1 []byte) bool {
		return string(v0) < string(v1)
	})

	// Update the storage trie with the encoded keys and values.
	errs := this.storageTrie.ParallelUpdate(encodedKeys, encodedVals)
	if _, err := slice.FindFirstIf(errs, func(_ int, v error) bool { return v != nil }); err != nil {
		return *err
	}

	this.Root = this.storageTrie.Hash()
	return nil
}

// Write the account changes to theirs Eth Trie
func (this *Account) ApplyChanges(transitions [][]*statecell.StateCell, getter func([]*statecell.StateCell) (string, stgcommon.Type)) ([]string, []stgcommon.Type, error) {
	keys := make([]string, len(transitions))
	typedVals := slice.Transform(transitions, func(i int, vals []*statecell.StateCell) stgcommon.Type {
		_, v := getter(vals)
		keys[i] = *vals[i].GetPath()
		return v
	})

	this.err = this.UpdateAccountTrie(keys, typedVals)
	return keys, typedVals, this.err
}

func (this *Account) Encode() []byte {
	encoded, _ := rlp.EncodeToBytes(&this.StateAccount)
	return encoded
}

func (*Account) Decode(buffer []byte) *Account {
	var acctState types.StateAccount
	rlp.DecodeBytes(buffer, &acctState)
	return &Account{StateAccount: acctState}
}

// Write the DB
func (this *Account) Commit(block uint64) error {
	var err error
	if this.StorageDirty {
		this.storageTrie, err = commitToEthDB(this.storageTrie, this.ethdb, block) // Commit the change to the storage trie.
		this.StorageDirty = false
	}
	return err // Write to DB
}

func (this *Account) Hash(key []byte) []byte {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write([]byte(key))
	sum := hasher.Sum(nil)
	return sum
}

func (this *Account) Print() {
	fmt.Println("addr: ", this.addr)
	PrintStateAccount(this.StateAccount)
	fmt.Println("code: ", this.code)
	fmt.Println("storageTrie: ", this.storageTrie)
	fmt.Println("ethdb: ", this.ethdb)
	fmt.Println("diskdbShards: ", this.diskdbShards)
	fmt.Println("err: ", this.err)
}

func PrintStateAccount(state ethtypes.StateAccount) {
	fmt.Println("Nonce: ", state.Nonce)
	fmt.Println("Balance: ", state.Balance)
	fmt.Println("Root: ", state.Root)
	fmt.Println("CodeHash: ", state.CodeHash)
}
