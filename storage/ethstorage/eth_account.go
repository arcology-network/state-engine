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

	"github.com/arcology-network/common-lib/codec"
	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
	commutative "github.com/arcology-network/common-lib/crdt/commutative"
	noncommutative "github.com/arcology-network/common-lib/crdt/noncommutative"
	statecell "github.com/arcology-network/common-lib/crdt/statecell"
	stgintf "github.com/arcology-network/common-lib/storage/interface"
	statecommon "github.com/arcology-network/state-engine/common"
	ethrlp "github.com/arcology-network/state-engine/storage/codec/ethcodec/rlp"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	ethmpt "github.com/ethereum/go-ethereum/trie"
	"github.com/holiman/uint256"
)

// Account is Arcology's account-facing compatibility view. Canonical account,
// code, and storage data are owned by the backend StateDB.
type Account struct {
	addr ethcommon.Address

	types.StateAccount
	code []byte

	storageTrie state.Trie
	backend     *EthShardTrieDB
	err         error
}

func NewAccount(addr ethcommon.Address, backend *EthShardTrieDB, account types.StateAccount) *Account {
	return &Account{addr: addr, backend: backend, StateAccount: account}
}

func NewAccountWithSharedCache(addr ethcommon.Address, backend *EthShardTrieDB, account types.StateAccount) *Account {
	return NewAccount(addr, backend, account)
}

func EmptyAccountState() types.StateAccount {
	return types.StateAccount{
		Balance:  uint256.NewInt(0),
		Root:     types.EmptyRootHash,
		CodeHash: codec.Bytes32(crypto.Keccak256Hash(nil)).Encode(),
	}
}

func (a *Account) refresh() {
	if a.backend == nil || a.backend.StateDB() == nil || !a.backend.StateDB().Exist(a.addr) {
		return
	}
	stateDB := a.backend.StateDB()
	a.Nonce = stateDB.GetNonce(a.addr)
	a.Balance = stateDB.GetBalance(a.addr).Clone()
	a.Root = stateDB.GetStorageRoot(a.addr)
	a.CodeHash = stateDB.GetCodeHash(a.addr).Bytes()
	a.code = stateDB.GetCode(a.addr)
}

func (a *Account) GetState(key [32]byte) []byte {
	return storageWordToRLP(a.backend.StateDB().GetState(a.addr, key))
}

func (a *Account) SetAddress(addr ethcommon.Address) { a.addr = addr }
func (a *Account) Address() ethcommon.Address        { return a.addr }
func (a *Account) Trie() state.Trie                  { return a.storageTrie }

func (a *Account) GetStorageRoot() [32]byte {
	if a.backend == nil {
		return types.EmptyRootHash
	}
	return a.backend.StateDB().GetStorageRoot(a.addr)
}

func (a *Account) GetCode() []byte {
	if a.backend != nil {
		return a.backend.StateDB().GetCode(a.addr)
	}
	return a.code
}

func (a *Account) GetCodeHash() [32]byte {
	if a.backend != nil {
		return a.backend.StateDB().GetCodeHash(a.addr)
	}
	return ethcommon.BytesToHash(a.CodeHash)
}

// storageSlot converts an Arcology path to the unhashed EVM storage key that
// StateDB expects. StateTrie performs the Keccak hashing exactly once.
func (a *Account) storageSlot(path string) (ethcommon.Hash, error) {
	if key := statecommon.GetPathUnder(path, "/storage/native/"); len(key) > 0 {
		decoded, err := hexutil.Decode(key)
		if err != nil {
			return ethcommon.Hash{}, err
		}
		return ethcommon.BytesToHash(decoded), nil
	}
	return crypto.Keccak256Hash([]byte(path)), nil
}

// ToStorageKey returns the hashed MPT key used by proof verification. Reads and
// writes must pass storageSlot to StateDB and let Geth hash it once.
func (a *Account) ToStorageKey(path string) []byte {
	slot, err := a.storageSlot(path)
	if err != nil {
		panic(err)
	}
	return crypto.Keccak256(slot.Bytes())
}

func (a *Account) isAccountRoot(path string) bool {
	return len(path) == statecommon.ETH_ACCOUNT_FULL_LENGTH+1
}

func (a *Account) Has(path string) bool {
	if a.backend == nil {
		return false
	}
	stateDB := a.backend.StateDB()
	if !stateDB.Exist(a.addr) {
		return false
	}
	// Arcology account namespaces are virtual when the canonical state lives in
	// Geth. They are not trie values, but execution writes require these parent
	// paths to exist before attaching profile or storage entries beneath them.
	if strings.HasSuffix(path, statecommon.PATH_FUNC_PROFILE) ||
		strings.HasSuffix(path, statecommon.PATH_STORAGE_ROOT) ||
		strings.HasSuffix(path, statecommon.PATH_STORAGE_CONTAINER) ||
		strings.HasSuffix(path, statecommon.PATH_STORAGE_NATIVE) {
		return true
	}
	if a.isAccountRoot(path) || strings.HasSuffix(path, statecommon.PATH_BALANCE) || strings.HasSuffix(path, statecommon.PATH_NONCE) {
		return true
	}
	if strings.HasSuffix(path, statecommon.PATH_CODE) {
		return len(stateDB.GetCode(a.addr)) > 0
	}
	slot, err := a.storageSlot(path)
	return err == nil && stateDB.GetState(a.addr, slot) != (ethcommon.Hash{})
}

func (a *Account) Get(path string) (any, error) {
	if a.backend == nil {
		return nil, stgintf.ErrNotFound
	}
	stateDB := a.backend.StateDB()
	if strings.HasSuffix(path, statecommon.PATH_BALANCE) {
		value := commutative.NewUnboundedU256()
		value.SetValue(*stateDB.GetBalance(a.addr).Clone())
		return value, nil
	}
	if strings.HasSuffix(path, statecommon.PATH_NONCE) {
		value := commutative.NewUnboundedUint64()
		value.SetValue(stateDB.GetNonce(a.addr))
		return value, nil
	}
	if strings.HasSuffix(path, statecommon.PATH_CODE) {
		code := stateDB.GetCode(a.addr)
		if len(code) == 0 {
			return nil, stgintf.ErrNotFound
		}
		return noncommutative.NewBytes(code), nil
	}
	slot, err := a.storageSlot(path)
	if err != nil {
		return nil, err
	}
	word := stateDB.GetState(a.addr, slot)
	if word == (ethcommon.Hash{}) {
		return nil, stgintf.ErrNotFound
	}
	return storageWordToRLP(word), nil
}

func storageWordToRLP(word ethcommon.Hash) []byte {
	content := bytes.TrimLeft(word.Bytes(), "\x00")
	encoded, _ := rlp.EncodeToBytes(content)
	return encoded
}

func storageValueToWord(path string, value crdtcommon.CRDT) (ethcommon.Hash, error) {
	if value == nil {
		return ethcommon.Hash{}, nil
	}
	encoded, err := (ethrlp.RlpCodec{}).Encode(path, value)
	if err != nil {
		return ethcommon.Hash{}, err
	}
	_, content, rest, err := rlp.Split(encoded)
	if err != nil {
		return ethcommon.Hash{}, err
	}
	if len(rest) != 0 {
		return ethcommon.Hash{}, errors.New("storage RLP contains trailing data")
	}
	if len(content) > ethcommon.HashLength {
		return ethcommon.Hash{}, fmt.Errorf("storage value at %q is %d bytes; EVM storage permits at most 32", path, len(content))
	}
	return ethcommon.BytesToHash(content), nil
}

// UpdateAccountStorageTrie translates an accepted account transition group to
// StateDB mutations. Calls must be serialized because StateDB is not thread-safe.
func (a *Account) UpdateAccountStorageTrie(keys []string, values []crdtcommon.CRDT) error {
	if len(keys) != len(values) {
		return fmt.Errorf("account update key/value mismatch: %d != %d", len(keys), len(values))
	}
	stateDB := a.backend.StateDB()
	if !stateDB.Exist(a.addr) {
		stateDB.CreateAccount(a.addr)
	}
	for i, path := range keys {
		value := values[i]
		switch {
		case a.isAccountRoot(path):
			continue
		case strings.HasSuffix(path, statecommon.PATH_NONCE):
			if value == nil {
				stateDB.SetNonce(a.addr, 0, tracing.NonceChangeUnspecified)
				continue
			}
			stateDB.SetNonce(a.addr, value.Value().(uint64), tracing.NonceChangeUnspecified)
		case strings.HasSuffix(path, statecommon.PATH_BALANCE):
			if value == nil {
				stateDB.SetBalance(a.addr, uint256.NewInt(0), tracing.BalanceChangeUnspecified)
				continue
			}
			balance := value.Value().(uint256.Int)
			stateDB.SetBalance(a.addr, balance.Clone(), tracing.BalanceChangeUnspecified)
		case strings.HasSuffix(path, statecommon.PATH_CODE):
			if value == nil {
				stateDB.SetCode(a.addr, nil)
				continue
			}
			stateDB.SetCode(a.addr, []byte(value.Value().(codec.Bytes)))
		default:
			slot, err := a.storageSlot(path)
			if err != nil {
				return err
			}
			word, err := storageValueToWord(path, value)
			if err != nil {
				return err
			}
			stateDB.SetState(a.addr, slot, word)
		}
	}
	if err := stateDB.Error(); err != nil {
		return err
	}
	return nil
}

func (a *Account) ApplyChanges(transitions [][]*statecell.StateCell, getter func([]*statecell.StateCell) (string, crdtcommon.CRDT)) ([]string, []crdtcommon.CRDT, error) {
	keys := make([]string, len(transitions))
	values := make([]crdtcommon.CRDT, len(transitions))
	for i, transition := range transitions {
		_, values[i] = getter(transition)
		keys[i] = *transition[i].GetPath()
	}
	a.err = a.UpdateAccountStorageTrie(keys, values)
	return keys, values, a.err
}

func (a *Account) Encode() []byte {
	a.refresh()
	encoded, _ := rlp.EncodeToBytes(&a.StateAccount)
	return encoded
}

func (*Account) Decode(buffer []byte) (*Account, error) {
	var account types.StateAccount
	if err := rlp.DecodeBytes(buffer, &account); err != nil {
		return nil, err
	}
	return &Account{StateAccount: account}, nil
}

// Commit is retained for source compatibility. StateDB commits all account and
// storage tries atomically through EthShardTrieDB.Commit.
func (a *Account) Commit(uint64) error { return a.err }

func (a *Account) IsAccountStorageProvable(key string) ([]byte, []string, error) {
	slot, _, err := decodeHash(key)
	if err != nil {
		return nil, nil, err
	}
	if a.storageTrie == nil {
		return nil, nil, errors.New("storage trie is unavailable")
	}
	hashedKey := crypto.Keccak256(slot.Bytes())
	data, err := a.storageTrie.GetStorage(a.addr, slot.Bytes())
	if err != nil || len(data) == 0 {
		return nil, nil, errors.New("failed to find storage key: " + key)
	}
	var proof proofList
	if err := a.storageTrie.Prove(hashedKey, &proof); err != nil {
		return nil, proof, err
	}
	proofDB, err := ProofArrayToDB(proof)
	if err != nil {
		return nil, proof, err
	}
	encoded, _ := rlp.EncodeToBytes(data)
	verified, err := ethmpt.VerifyProof(a.GetStorageRoot(), hashedKey, proofDB)
	if err != nil || !bytes.Equal(verified, encoded) {
		return nil, proof, errors.New("failed to verify storage key: " + key)
	}
	return encoded, proof, nil
}

func (a *Account) Print() {
	a.refresh()
	fmt.Println("addr: ", a.addr)
	PrintStateAccount(a.StateAccount)
	fmt.Println("code: ", a.code)
	fmt.Println("storageTrie: ", a.storageTrie)
	fmt.Println("err: ", a.err)
}

func PrintStateAccount(account types.StateAccount) {
	fmt.Println("Nonce: ", account.Nonce)
	fmt.Println("Balance: ", account.Balance)
	fmt.Println("Root: ", account.Root)
	fmt.Println("CodeHash: ", account.CodeHash)
}
