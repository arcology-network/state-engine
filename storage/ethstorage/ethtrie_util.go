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

	"github.com/arcology-network/common-lib/common"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethmpt "github.com/ethereum/go-ethereum/trie"

	// "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	// ethmpt "github.com/ethereum/go-ethereum/trie"
)

// ethapi "github.com/ethereum/go-ethereum/internal/ethapi"

// func commitTrieToDB(trie *ethmpt.Trie, ethdb *triedb.Database, block uint64) (*ethmpt.Trie, error) {
// 	root, nodes, err := trie.Commit(false) // Finalized the trie
// 	if err != nil {
// 		return nil, errors.Join(errors.New("trie.Commit:"), err)
// 	}

// 	if nodes != nil {
// 		if err := ethdb.Update(root, types.EmptyRootHash, block, trienode.NewWithNodeSet(nodes), nil); err != nil { // Move to DB dirty node set
// 			return nil, errors.Join(errors.New("ethdb.Update"), err)
// 		}

// 		if err := ethdb.Commit(root, false); err != nil {
// 			return nil, errors.Join(errors.New("ethdb.Commit:"), err)
// 		}
// 	}
// 	newTrie, err := ethmpt.NewParallel(ethmpt.TrieID(root), ethdb)
// 	if err != nil {
// 		err = errors.Join(errors.New("ethmpt.NewParallel:"), err)
// 	}
// 	return newTrie, err
// }

// func parallelcommitToEthDB(trie *ethmpt.Trie, ethdb *triedb.Database, block uint64) (*ethmpt.Trie, error) {
// 	root, nodes, err := trie.Commit(false) // Finalized the trie
// 	if err != nil {
// 		return nil, err
// 	}

// 	if nodes != nil {
// 		if err := ethdb.Update(root, types.EmptyRootHash, block, trienode.NewWithNodeSet(nodes), nil); err != nil { // Move to DB dirty node set
// 			return nil, err
// 		}

// 		if err := ethdb.Commit(root, false); err != nil {
// 			return nil, err
// 		}
// 	}
// 	return ethmpt.NewParallel(ethmpt.TrieID(root), ethdb)
// }

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

func VerifyProof(rootHash ethcommon.Hash, proof []string, addr []byte) {
	proofDB, _ := ProofArrayToDB(proof)
	data, err := ethmpt.VerifyProof(rootHash, crypto.Keccak256(addr), proofDB)
	if err != nil || len(data) == 0 {
		panic(err)
	}
}

// Move to EthShardDB
