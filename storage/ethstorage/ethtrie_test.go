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

	"github.com/arcology-network/common-lib/exp/slice"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/triedb/hashdb"
	"github.com/holiman/uint256"
)

func TestHistoryStateProxyWithLvlDB(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDBDir := tmpDir + "/eth_storage_test_db"
	worldState := NewLevelDBDataStore(tmpDBDir, &hashdb.Config{CleanCacheSize: 1024 * 1024 * 100})
	paraTrie := worldState.WorldStateTrie()

	key0 := []byte("test_key_0")
	value0 := []byte("test_value_0")

	key1 := []byte("test_key_1")
	value1 := []byte("test_value_1")

	// Update the trie with two key-value pairs in parallel
	errs := paraTrie.ParallelUpdate([][]byte{key0, key1}, [][]byte{value0, value1})
	if _, err := slice.FindFirstIf(errs, func(i int, err error) bool { return err != nil }); err != nil {
		t.Fatalf("Failed to update the trie: %v", *err)
	}

	// Now retrieve the values
	outVals, err := paraTrie.ParallelGet([][]byte{key0, key1})
	if err != nil {
		t.Fatalf("Failed to get values from the trie: %v", err)
	}

	if outVals[0] == nil || string(outVals[0]) != string(value0) {
		t.Fatalf("Failed to get the correct value for key0: expected %s, got %s", string(value0), string(outVals[0]))
	}

	if outVals[1] == nil || string(outVals[1]) != string(value1) {
		t.Fatalf("Failed to get the correct value for key1: expected %s, got %s", string(value1), string(outVals[1]))
	}

	root0, newParaTrie, err := worldState.trieDB.Commit(paraTrie, 0)
	if err != nil {
		t.Fatalf("Failed to commit the trie: %v", err)
	}

	// Reopen the trie by its root
	reopened, err := LoadEthTrieByRoot(worldState.trieDB.mainTrieDB, root0)
	if err != nil {
		t.Fatalf("Failed to load trie by root: %v", err)
	}

	outVals, err = reopened.WorldStateTrie().ParallelGet([][]byte{key0, key1})
	if err != nil {
		t.Fatalf("Failed to get values from the trie: %v", err)
	}

	if outVals[0] == nil || string(outVals[0]) != string(value0) {
		t.Fatalf("Failed to get the correct value for key0: expected %s, got %s", string(value0), string(outVals[0]))
	}

	if outVals[1] == nil || string(outVals[1]) != string(value1) {
		t.Fatalf("Failed to get the correct value for key1: expected %s, got %s", string(value1), string(outVals[1]))
	}

	// // Update the trie again with two more key-value pairs in parallel
	key2 := []byte("test_key_0")
	value2 := []byte("test_value_01")

	key3 := []byte("test_key_3")
	value3 := []byte("test_value_3")

	errs = newParaTrie.ParallelUpdate([][]byte{key2, key3}, [][]byte{value2, value3})
	if _, err := slice.FindFirstIf(errs, func(i int, err error) bool { return err != nil }); err != nil {
		t.Fatalf("Failed to update the trie: %v", *err)
	}

	_2ndRoot, _, err := worldState.trieDB.Commit(newParaTrie, 0)
	if err != nil {
		t.Fatalf("Failed to commit the trie: %v", err)
	}

	// Reopen the second trie by its root
	_2nd_reopened, err := LoadEthTrieByRoot(worldState.trieDB.mainTrieDB, _2ndRoot)
	if err != nil {
		t.Fatalf("Failed to load trie by root: %v", err)
	}

	outVals, err = _2nd_reopened.WorldStateTrie().ParallelGet([][]byte{key2, key3})
	if err != nil {
		t.Fatalf("Failed to get values from the trie: %v", err)
	}

	if outVals[0] == nil || string(outVals[0]) != string(value2) {
		t.Fatalf("Failed to get the correct value for key2: expected %s, got %s", string(value2), string(outVals[0]))
	}

	if outVals[1] == nil || string(outVals[1]) != string(value3) {
		t.Fatalf("Failed to get the correct value for key3: expected %s, got %s", string(value3), string(outVals[1]))
	}
}

func TestAccountCode(t *testing.T) {
	state := &ethtypes.StateAccount{
		Nonce:    111,
		Balance:  uint256.NewInt(0),
		Root:     ethcommon.Hash{}, // merkle root of the storage trie
		CodeHash: []byte{9, 8, 0, 7},
	}

	if encoded, err := rlp.EncodeToBytes(state); err != nil {
		t.Error("Error: Should be empty!!")
	} else {
		var acct ethtypes.StateAccount
		rlp.DecodeBytes(encoded, &acct)
		if state.Balance.Uint64() != acct.Balance.Uint64() {
			t.Error("Error: Blance mismatched!!")
		}
	}

	acct := &Account{
		addr:         ethcommon.BytesToAddress([]byte("3456")),
		StateAccount: *state,
		code:         []byte{1, 2, 3, 4},
	}
	buffer := acct.Encode()

	decodeAcct, err := (&Account{}).Decode(buffer)
	if err != nil {
		t.Error("Error: Failed to decode account!!")
	}
	if state.Balance.Uint64() != decodeAcct.Balance.Uint64() {
		t.Error("Error: Blance mismatched!!")
	}
}
