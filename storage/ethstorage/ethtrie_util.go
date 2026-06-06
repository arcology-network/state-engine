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
	// "errors"

	"errors"

	"github.com/arcology-network/common-lib/common"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
	ethmpt "github.com/ethereum/go-ethereum/trie"

	// "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	// ethmpt "github.com/ethereum/go-ethereum/trie"
)

// ProofArrayToDB builds an in-memory trie database from the provided hex-encoded proof nodes.
// The nodes are decoded, optionally hashed when larger than 32 bytes, and inserted into memorydb.
func ProofArrayToDB(proofs []string) (*memorydb.Database, error) {
	proofDB := memorydb.New()
	for i := 0; i < len(proofs); i++ {
		proofBytes := hexutil.MustDecode(proofs[i])

		keyBytes := common.IfThen(len(proofBytes) >= 32, crypto.Keccak256([]byte(proofBytes)), proofBytes)
		if err := proofDB.Put(keyBytes, proofBytes); err != nil {
			return nil, err
		}
	}
	return proofDB, nil
}

// VerifyProof ensures that the supplied Merkle proof corresponds to the given root hash and address.
// It panics if the verification fails or if the proof data is empty.
func VerifyProof(rootHash ethcommon.Hash, proof []string, addr []byte) {
	proofDB, _ := ProofArrayToDB(proof)
	data, err := ethmpt.VerifyProof(rootHash, crypto.Keccak256(addr), proofDB)
	if err != nil || len(data) == 0 {
		panic(err)
	}
}

func GetFromTrie[T any](trie *ethmpt.Trie, key []byte) (T, error) {
	acctHash := crypto.Keccak256(key) // Hash the key string
	buffer, err := trie.Get(acctHash)

	var result T
	if err != nil {
		return result, err
	}

	if len(buffer) == 0 {
		return result, errors.New("key not found in trie")
	}

	err = rlp.DecodeBytes(buffer, &result)
	return result, err
}
