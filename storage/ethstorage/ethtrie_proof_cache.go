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
	"sync"

	mapi "github.com/arcology-network/common-lib/exp/map"
	"github.com/arcology-network/common-lib/exp/slice"
	tridb "github.com/ethereum/go-ethereum/triedb"
	// ethapi "github.com/ethereum/go-ethereum/internal/ethapi"
)

// MkerkleProofManager is a manager for merkle proofs. It keeps track of the number of times a merkle
// tree has been accessed and keeps the most recently used merkle trees in memory. It a mkerkle tree isn't
// in memory, it will be loaded from the database.
type MerkleProofCache struct {
	maxCached  int                         // max number of merkle proofs to keep in memory
	merkleDict map[[32]byte]*ProofProvider // The merkle tree for each root.
	db         *tridb.Database
	lock       sync.Mutex
}

// NewMerkleProofCache creates a new MerkleProofCache, which keeps a cache of merkle trees in memory.
// When the cache is full, the merkle tree with the lowest ratio of visits/totalVisits will be removed.
func NewMerkleProofCache(maxCached int, db *tridb.Database) *MerkleProofCache {
	return &MerkleProofCache{
		maxCached:  maxCached,
		merkleDict: map[[32]byte]*ProofProvider{},
		db:         db,
	}
}

// GetProof returns a merkle proof for the given account and storage keys.
func (this *MerkleProofCache) GetProofProvider(rootHash [32]byte) (*ProofProvider, error) {
	this.lock.Lock()
	defer this.lock.Unlock()

	merkle, _ := this.merkleDict[rootHash]
	if merkle == nil {
		datastore, err := LoadEthDataStore(this.db, rootHash)
		if err != nil {
			return nil, err
		}

		// Create a new merkle tree and add it to the cache.
		merkle = &ProofProvider{root: rootHash, totalVisits: 1, visits: 1, DataStore: datastore, Ethdb: this.db}
		this.merkleDict[rootHash] = merkle

		// Check if the cache is full. Clean up the cache if it is full.
		if len(this.merkleDict) > this.maxCached {
			keys, merkles := mapi.KVs(this.merkleDict)

			// The visit ratio is the number of times a merkle tree has been accessed divided by the total number of times all the merkle trees have been accessed.
			ratios := slice.Transform(merkles, func(_ int, v *ProofProvider) float64 { return float64(v.visits) / float64(v.totalVisits) })

			// The entry has the lowest ratio of visits/totalVisits will be removed.
			idx, _ := slice.Extreme(ratios, func(v0, v1 float64) bool { return v0 < v1 })
			delete(this.merkleDict, keys[idx])
		}
	}

	// Increment the number of visits for all the merkle trees by 1.
	mapi.Foreach(this.merkleDict, func(_ [32]byte, v **ProofProvider) { (**v).totalVisits++ })
	return merkle, nil //Merkle.GetProof(acctStr, storageKeys)
}
