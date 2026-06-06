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

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/rlp"
	ethmpt "github.com/ethereum/go-ethereum/trie"
	tridb "github.com/ethereum/go-ethereum/triedb"
	// ethapi "github.com/ethereum/go-ethereum/internal/ethapi"
)

type ProofProvider struct {
	root        [32]byte
	totalVisits uint64 // Total number of times all the merkle trees have been accessed since this Merkle tree is created.
	visits      int    // Number of times this merkle Merkle has been accessed.
	DataStore   *EthWorldState
	Ethdb       *tridb.Database
}

func NewProofProvider(ethdb *tridb.Database, root [32]byte) (*ProofProvider, error) {
	store, err := LoadEthTrieByRoot(ethdb, root)
	if err != nil {
		return nil, err
	}

	return &ProofProvider{
		root,
		1,
		1,
		store,
		ethdb,
	}, nil
}

func (this *ProofProvider) Root() ethcommon.Hash { this.visits++; return this.root }

// GetProof returns a merkle proof for the given account and storage keys.
// Storage keys have to be in hex format with 0x prefix.
func (this *ProofProvider) GetProof(acctAddr ethcommon.Address, storageKeys []string) (*AccountResult, error) {
	this.visits++

	// Get the account either from the cache or from the database.
	account, err := this.DataStore.GetAccount(acctAddr, new(ethmpt.AccessListCache))
	// account := acct.(*Account)
	if account == nil || err != nil {
		return nil, err
	}

	// Debugging only. Will panic if the proof is invalid.
	// if data, _, err := account.IsAccountStorageProvable(storageKeys[0]); err != nil || len(data) == 0 {
	// 	panic(err)
	// }

	storageHash := account.GetStorageRoot()
	codeHash := account.GetCodeHash()

	// Create the storage proof for each storage key.
	var aggregatedErr error
	storageProof := make([]StorageResult, len(storageKeys))
	for i, hexKey := range storageKeys {
		key, keyLength, err := decodeHash(hexKey)
		if err != nil {
			return nil, err
		}

		// Output key encoding is a bit special: if the input was a 32-byte hash, it is
		// returned as such. Otherwise, we apply the QUANTITY encoding mandated by the
		// JSON-RPC spec for getProof. This behavior exists to preserve backwards
		// compatibility with older client versions.
		var outputKey string
		if keyLength != 32 {
			outputKey = hexutil.EncodeBig(key.Big())
		} else {
			outputKey = hexutil.Encode(key[:])
		}

		if account.storageTrie == nil {
			storageProof[i] = StorageResult{outputKey, &hexutil.Big{}, []string{}}
			continue
		}

		storageKey := crypto.Keccak256(key.Bytes()) // Calculate the key for retrieving the value from the trie.

		var proof proofList
		if err := account.storageTrie.Prove(storageKey, &proof); err != nil {
			return nil, err
		}
		// VerifyProof(account.storageTrie.Hash(), proof, key[:]) // Debugging only. Will panic if the proof is invalid.

		/* ETH code:
		value := (*hexutil.Big)(statedb.GetState(address, key).Big())
		storageProof[i] = StorageResult{outputKey, value, proof}
		*/

		v, _ := account.storageTrie.Get(storageKey)

		decoded := []byte{}
		if err := rlp.DecodeBytes(v, &decoded); err != nil {
			aggregatedErr = errors.Join(aggregatedErr, err)
		}
		storageProof[i] = StorageResult{outputKey, (*hexutil.Big)(ethcommon.BytesToHash(decoded).Big()), proof}
	}

	// create the account Proof
	accountProof, proofErr := this.DataStore.GetAccountProof(acctAddr) // Get the account proof
	if proofErr != nil {
		return nil, proofErr
	}

	VerifyProof(this.DataStore.worldStateTrie.Hash(), accountProof, acctAddr[:]) // Debugging only. Will panic if the proof is invalid.

	return &AccountResult{
		Address:      acctAddr,
		AccountProof: accountProof,
		Balance:      (*hexutil.Big)(account.StateAccount.Balance.ToBig()),
		CodeHash:     codeHash,
		Nonce:        hexutil.Uint64(account.StateAccount.Nonce),
		StorageHash:  storageHash,
		StorageProof: storageProof,
	}, nil // state.Error()
}

func IsAccountProvable(addr string, acctRoot [32]byte, worldTrie *ethmpt.Trie) ([]byte, error) {
	addrBytes, _ := hexutil.Decode(addr) // Decode to remove the 0x prefix
	keyHash := crypto.Keccak256(addrBytes)

	proofs := memorydb.New()
	if trie, _ := worldTrie.Get(keyHash); len(trie) > 0 {
		if err := worldTrie.Prove(keyHash, proofs); err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("Failed to get the proof")
	}

	v, err := ethmpt.VerifyProof(acctRoot, keyHash, proofs)
	if err != nil || len(v) == 0 {
		return v, errors.New("Failed to verify the proof")
	}
	return v, nil
}
