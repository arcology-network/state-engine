/*
 *   Copyright (c) 2026 Arcology Network

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
	"testing"

	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/holiman/uint256"

	noncommutative "github.com/arcology-network/common-lib/crdt/noncommutative"
	statecommon "github.com/arcology-network/state-engine/common"
)

func TestGethStateBackendCommitAndReopen(t *testing.T) {
	backend, err := NewGethStateBackend(rawdb.NewMemoryDatabase(), triedb.HashDefaults, common.Hash{})
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()

	var (
		address = common.HexToAddress("0x1234")
		slot    = common.HexToHash("0x01")
		value   = common.HexToHash("0xabcdef")
		code    = []byte{0x60, 0x00, 0x60, 0x00, 0xf3}
	)

	stateDB := backend.StateDB()
	stateDB.SetBalance(address, uint256.NewInt(42), tracing.BalanceChangeUnspecified)
	stateDB.SetNonce(address, 7, tracing.NonceChangeUnspecified)
	stateDB.SetCode(address, code)
	stateDB.SetState(address, slot, value)

	root, err := backend.Commit(1)
	if err != nil {
		t.Fatal(err)
	}
	if root == (common.Hash{}) {
		t.Fatal("committed an empty root")
	}
	if backend.Root() != root {
		t.Fatalf("backend root mismatch: have %x want %x", backend.Root(), root)
	}

	reader, err := backend.Reader(root)
	if err != nil {
		t.Fatal(err)
	}
	account, err := reader.Account(address)
	if err != nil {
		t.Fatal(err)
	}
	if account == nil {
		t.Fatal("committed account is missing")
	}
	if account.Balance.Cmp(uint256.NewInt(42)) != 0 {
		t.Fatalf("balance mismatch: have %s want 42", account.Balance)
	}
	if account.Nonce != 7 {
		t.Fatalf("nonce mismatch: have %d want 7", account.Nonce)
	}

	storedCode, err := reader.Code(address, common.BytesToHash(account.CodeHash))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(storedCode, code) {
		t.Fatalf("code mismatch: have %x want %x", storedCode, code)
	}
	storedValue, err := reader.Storage(address, slot)
	if err != nil {
		t.Fatal(err)
	}
	if storedValue != value {
		t.Fatalf("storage mismatch: have %x want %x", storedValue, value)
	}

	accountTrie, err := backend.Database().OpenTrie(root)
	if err != nil {
		t.Fatal(err)
	}
	proof := memorydb.New()
	if err := accountTrie.Prove(address.Bytes(), proof); err != nil {
		t.Fatal(err)
	}
	if _, err := trie.VerifyProof(root, crypto.Keccak256(address.Bytes()), proof); err != nil {
		t.Fatalf("verify account proof: %v", err)
	}
}

func TestAccountNativeStorageRoundTrip(t *testing.T) {
	world := NewParallelEthMemDataStore()
	defer world.Close()

	address := common.BytesToAddress([]byte("alice"))
	account := NewAccountWithSharedCache(address, world.trieDB, EmptyAccountState())
	path := statecommon.ETH_ACCOUNT_PREFIX + address.Hex() + "/storage/native/0x09"
	if err := account.UpdateAccountStorageTrie([]string{path}, []crdtcommon.CRDT{noncommutative.NewString("435")}); err != nil {
		t.Fatal(err)
	}
	slot := common.HexToHash("0x09")
	if got := world.trieDB.StateDB().GetState(address, slot); got == (common.Hash{}) {
		t.Fatal("native storage mutation was not applied")
	}
	root, err := world.trieDB.Commit(1)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := LoadEthTrieByRoot(world.EthDB(), root)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := reopened.GetAccount(address, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := stored.IsAccountStorageProvable("0x09"); err != nil {
		t.Fatal(err)
	}
}

func TestGethStateBackendMatchesCanonicalRawTrieRoot(t *testing.T) {
	address := common.HexToAddress("0x1234")
	slot := common.HexToHash("0x09")
	value := common.HexToHash("0x01b3")

	manualTrieDB := triedb.NewDatabase(rawdb.NewMemoryDatabase(), triedb.HashDefaults)
	defer manualTrieDB.Close()
	storageTrie := trie.NewEmpty(manualTrieDB)
	storageValue, err := rlp.EncodeToBytes(bytes.TrimLeft(value.Bytes(), "\x00"))
	if err != nil {
		t.Fatal(err)
	}
	if err := storageTrie.Update(crypto.Keccak256(slot.Bytes()), storageValue); err != nil {
		t.Fatal(err)
	}
	accountState := types.StateAccount{
		Nonce:    7,
		Balance:  uint256.NewInt(42),
		Root:     storageTrie.Hash(),
		CodeHash: types.EmptyCodeHash.Bytes(),
	}
	accountValue, err := rlp.EncodeToBytes(&accountState)
	if err != nil {
		t.Fatal(err)
	}
	accountTrie := trie.NewEmpty(manualTrieDB)
	if err := accountTrie.Update(crypto.Keccak256(address.Bytes()), accountValue); err != nil {
		t.Fatal(err)
	}
	manualRoot := accountTrie.Hash()

	backend, err := NewGethStateBackend(rawdb.NewMemoryDatabase(), triedb.HashDefaults, common.Hash{})
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	backend.StateDB().SetBalance(address, uint256.NewInt(42), tracing.BalanceChangeUnspecified)
	backend.StateDB().SetNonce(address, 7, tracing.NonceChangeUnspecified)
	backend.StateDB().SetState(address, slot, value)
	gethRoot, err := backend.Commit(1)
	if err != nil {
		t.Fatal(err)
	}
	if gethRoot != manualRoot {
		t.Fatalf("state root mismatch: Geth %x manual canonical trie %x", gethRoot, manualRoot)
	}
}
