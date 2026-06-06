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
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/trie"
	ethmpt "github.com/ethereum/go-ethereum/trie"
	// ethapi "github.com/ethereum/go-ethereum/internal/ethapi"
)

// Validate verifies the account proof and storage proof based on the account result.
func (this *AccountResult) Validate(rootHash ethcommon.Hash) error {
	proofdb, err := ProofArrayToDB(this.AccountProof) // Add the proof to the memorydb for verification.
	if err != nil {
		return err
	}

	v, err := ethmpt.VerifyProof(rootHash, crypto.Keccak256(this.Address[:]), proofdb)
	if err != nil || len(v) == 0 {
		return errors.New("Failed to find the validate the account proof!!!")
	}

	for i, entry := range this.StorageProof {
		db := memorydb.New()
		for j, encodedStr := range entry.Proof {
			var nodeKey []byte
			if len(encodedStr) >= 32 { // small MPT nodes are not hashed
				nodeKey = crypto.Keccak256(hexutil.MustDecode(encodedStr))
			}

			if err := db.Put(nodeKey, hexutil.MustDecode(encodedStr)); err != nil {
				return fmt.Errorf("failed to load storage proof node %d of storage value %d into mem db: %w", j, i, err)
			}
		}

		decoded := hexutil.MustDecode(entry.Key)
		path := crypto.Keccak256(decoded)
		val, err := trie.VerifyProof(this.StorageHash, path, db)
		if err != nil || val == nil {
			return fmt.Errorf("failed to verify storage value %d with key %s (path %x) in storage trie %s: %w", i, entry.Key, path, this.StorageHash, err)
		}
	}
	return nil
}

// ------------------------Types copied from ETH------------------------
type AccountResult struct {
	Address      common.Address  `json:"address"` // Address of the account
	AccountProof []string        `json:"accountProof"`
	Balance      *hexutil.Big    `json:"balance"`
	CodeHash     common.Hash     `json:"codeHash"`
	Nonce        hexutil.Uint64  `json:"nonce"`
	StorageHash  common.Hash     `json:"storageHash"`
	StorageProof []StorageResult `json:"storageProof"`
}

type StorageResult struct {
	Key   string       `json:"key"`
	Value *hexutil.Big `json:"value"`
	Proof []string     `json:"proof"`
}

type proofList []string

func (n *proofList) Put(key []byte, value []byte) error {
	*n = append(*n, hexutil.Encode(value))
	return nil
}

func (n *proofList) Delete(key []byte) error {
	panic("not supported")
}

func decodeHash(s string) (h common.Hash, inputLength int, err error) {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		s = s[2:]
	}
	if (len(s) & 1) > 0 {
		s = "0" + s
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return common.Hash{}, 0, errors.New("hex string invalid")
	}
	if len(b) > 32 {
		return common.Hash{}, len(b), errors.New("hex string too long, 32 bytes at most")
	}
	return common.BytesToHash(b), len(b), nil
}

// ------------------------End of types copied from ETH------------------------
