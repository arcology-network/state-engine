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
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	ethmpt "github.com/ethereum/go-ethereum/trie"
	tridb "github.com/ethereum/go-ethereum/triedb"
)

type ProofProvider struct {
	root        [32]byte
	totalVisits uint64
	visits      int
	DataStore   *EthWorldState
	Ethdb       *tridb.Database
}

func NewProofProvider(ethdb *tridb.Database, root [32]byte) (*ProofProvider, error) {
	store, err := LoadEthTrieByRoot(ethdb, root)
	if err != nil {
		return nil, err
	}
	return &ProofProvider{root: root, totalVisits: 1, visits: 1, DataStore: store, Ethdb: ethdb}, nil
}

func (p *ProofProvider) Root() ethcommon.Hash {
	p.visits++
	return p.root
}

func (p *ProofProvider) GetProof(address ethcommon.Address, storageKeys []string) (*AccountResult, error) {
	p.visits++
	account, err := p.DataStore.GetAccount(address, nil)
	if err != nil || account == nil {
		return nil, err
	}

	storageProof := make([]StorageResult, len(storageKeys))
	for i, encodedKey := range storageKeys {
		slot, keyLength, err := decodeHash(encodedKey)
		if err != nil {
			return nil, err
		}
		outputKey := hexutil.Encode(slot[:])
		if keyLength != ethcommon.HashLength {
			outputKey = hexutil.EncodeBig(slot.Big())
		}

		if account.storageTrie == nil {
			storageProof[i] = StorageResult{Key: outputKey, Value: &hexutil.Big{}, Proof: []string{}}
			continue
		}
		var proof proofList
		if err := account.storageTrie.Prove(crypto.Keccak256(slot.Bytes()), &proof); err != nil {
			return nil, err
		}
		value := p.DataStore.trieDB.StateDB().GetState(address, slot)
		storageProof[i] = StorageResult{
			Key:   outputKey,
			Value: (*hexutil.Big)(value.Big()),
			Proof: proof,
		}
	}

	accountProof, err := p.DataStore.GetAccountProof(address)
	if err != nil {
		return nil, err
	}
	VerifyProof(p.root, accountProof, address[:])
	account.refresh()

	return &AccountResult{
		Address:      address,
		AccountProof: accountProof,
		Balance:      (*hexutil.Big)(account.Balance.ToBig()),
		CodeHash:     account.GetCodeHash(),
		Nonce:        hexutil.Uint64(account.Nonce),
		StorageHash:  account.GetStorageRoot(),
		StorageProof: storageProof,
	}, nil
}

func IsAccountProvable(addr string, accountRoot [32]byte, worldTrie state.Trie) ([]byte, error) {
	address, err := hexutil.Decode(addr)
	if err != nil {
		return nil, err
	}
	key := crypto.Keccak256(address)
	proof := memorydb.New()
	if err := worldTrie.Prove(key, proof); err != nil {
		return nil, err
	}
	value, err := ethmpt.VerifyProof(accountRoot, key, proof)
	if err != nil || len(value) == 0 {
		return value, errors.New("failed to verify account proof")
	}
	return value, nil
}
