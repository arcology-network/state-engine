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
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/triedb/hashdb"
	"github.com/holiman/uint256"
)

func TestHistoryStateProxyWithLvlDB(t *testing.T) {
	worldState := NewPebbleDataStore(t.TempDir()+"/eth_storage_test_db", &hashdb.Config{CleanCacheSize: 100 * 1024 * 1024})
	if worldState == nil {
		t.Fatal("failed to create Pebble world state")
	}
	defer worldState.Close()

	address := ethcommon.HexToAddress("0x1234")
	slot := ethcommon.HexToHash("0x01")
	value0 := ethcommon.HexToHash("0x10")
	stateDB := worldState.trieDB.StateDB()
	stateDB.SetBalance(address, uint256.NewInt(42), tracing.BalanceChangeUnspecified)
	stateDB.SetNonce(address, 1, tracing.NonceChangeUnspecified)
	stateDB.SetState(address, slot, value0)

	root0, err := worldState.trieDB.Commit(0)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := LoadEthTrieByRoot(worldState.EthDB(), root0)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.trieDB.StateDB().GetState(address, slot); got != value0 {
		t.Fatalf("storage mismatch: have %x want %x", got, value0)
	}

	value1 := ethcommon.HexToHash("0x20")
	reopened.trieDB.StateDB().SetState(address, slot, value1)
	root1, err := reopened.trieDB.Commit(1)
	if err != nil {
		t.Fatal(err)
	}
	if root1 == root0 {
		t.Fatal("root did not change after storage update")
	}
	second, err := LoadEthTrieByRoot(worldState.EthDB(), root1)
	if err != nil {
		t.Fatal(err)
	}
	if got := second.trieDB.StateDB().GetState(address, slot); got != value1 {
		t.Fatalf("updated storage mismatch: have %x want %x", got, value1)
	}
}

func TestAccountCode(t *testing.T) {
	accountState := &ethtypes.StateAccount{
		Nonce:    111,
		Balance:  uint256.NewInt(0),
		Root:     ethcommon.Hash{},
		CodeHash: []byte{9, 8, 0, 7},
	}
	encoded, err := rlp.EncodeToBytes(accountState)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ethtypes.StateAccount
	if err := rlp.DecodeBytes(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if accountState.Balance.Uint64() != decoded.Balance.Uint64() {
		t.Fatal("balance mismatch")
	}

	account := &Account{addr: ethcommon.BytesToAddress([]byte("3456")), StateAccount: *accountState, code: []byte{1, 2, 3, 4}}
	decodedAccount, err := (&Account{}).Decode(account.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if accountState.Balance.Uint64() != decodedAccount.Balance.Uint64() {
		t.Fatal("decoded account balance mismatch")
	}
}

func TestPersistToEthStoreRecordsCommittedRoot(t *testing.T) {
	worldState := NewParallelEthMemDataStore()
	defer worldState.Close()
	address := ethcommon.BytesToAddress([]byte("committed-account"))
	worldState.trieDB.StateDB().SetBalance(address, uint256.NewInt(1), tracing.BalanceChangeUnspecified)

	if err := worldState.persistToEthStore(7, nil); err != nil {
		t.Fatal(err)
	}
	if got := worldState.GetRootHash(7); got != worldState.Root() {
		t.Fatal("block root was not recorded")
	}
	if len(worldState.blockRoots) != 1 {
		t.Fatalf("expected one block root, got %d", len(worldState.blockRoots))
	}
}
