/*
 *   Copyright (c) 2025 Arcology Network

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
	"github.com/ethereum/go-ethereum/ethdb"
	ethmpt "github.com/ethereum/go-ethereum/trie"
	tridb "github.com/ethereum/go-ethereum/triedb"
)

// AccountStorage represents an Ethereum account with its associated
// storage trie and underlying databases.
type AccountStorage struct {
	StorageDirty bool
	storageTrie  *ethmpt.Trie // account storage trie
	ethdb        *tridb.Database
	diskdbShards [16]ethdb.Database
	err          error
}

// NewAccountStorage initializes a new AccountStorage instance.
func NewAccountStorage(ethdb *tridb.Database, diskdbShards [16]ethdb.Database, storageTrie *ethmpt.Trie) *AccountStorage {
	return &AccountStorage{
		ethdb:        ethdb,
		diskdbShards: diskdbShards,
		storageTrie:  storageTrie,
	}
}
