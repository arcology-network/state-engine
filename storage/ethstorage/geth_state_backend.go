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
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/triedb"
)

// GethStateBackend owns Geth's canonical state and trie database objects.
// Arcology's storage writer translates accepted state transitions into this
// backend instead of managing account and storage tries directly.
type GethStateBackend struct {
	disk     ethdb.Database
	trieDB   *triedb.Database
	database *state.CachingDB
	stateDB  *state.StateDB
	root     common.Hash
	owned    bool
}

// NewGethStateBackend opens Geth state at root using the supplied disk and trie
// configuration. The caller transfers ownership of disk to the backend.
func NewGethStateBackend(disk ethdb.Database, config *triedb.Config, root common.Hash) (*GethStateBackend, error) {
	if disk == nil {
		return nil, fmt.Errorf("create Geth state backend: nil disk database")
	}
	if config == nil {
		config = triedb.HashDefaults
	}

	trieDB := triedb.NewDatabase(disk, config)
	backend, err := newGethStateBackend(disk, trieDB, root, true)
	if err != nil {
		_ = trieDB.Close()
		return nil, err
	}
	return backend, nil
}

// OpenGethStateBackend opens state using an existing trie database. The trie
// database remains owned by the caller and is not closed by Close.
func OpenGethStateBackend(trieDB *triedb.Database, root common.Hash) (*GethStateBackend, error) {
	if trieDB == nil {
		return nil, fmt.Errorf("open Geth state backend: nil trie database")
	}
	return newGethStateBackend(trieDB.Disk(), trieDB, root, false)
}

func newGethStateBackend(disk ethdb.Database, trieDB *triedb.Database, root common.Hash, owned bool) (*GethStateBackend, error) {
	database := state.NewDatabase(trieDB, nil)
	stateDB, err := state.New(root, database)
	if err != nil {
		return nil, fmt.Errorf("open Geth state at root %x: %w", root, err)
	}

	return &GethStateBackend{
		disk:     disk,
		trieDB:   trieDB,
		database: database,
		stateDB:  stateDB,
		root:     root,
		owned:    owned,
	}, nil
}

// Root returns the root at which the current StateDB was opened.
func (b *GethStateBackend) Root() common.Hash {
	return b.root
}

// StateDB returns the mutable Geth state used to materialize accepted Arcology
// transitions. It must not be mutated concurrently.
func (b *GethStateBackend) StateDB() *state.StateDB {
	return b.stateDB
}

// Database returns Geth's caching state database.
func (b *GethStateBackend) Database() *state.CachingDB {
	return b.database
}

// TrieDB returns the trie database backing the canonical Geth state.
func (b *GethStateBackend) TrieDB() *triedb.Database {
	return b.trieDB
}

// Reader returns a thread-safe reader fixed at root.
func (b *GethStateBackend) Reader(root common.Hash) (state.Reader, error) {
	return b.database.Reader(root)
}

// Commit materializes the pending StateDB changes, flushes the resulting trie
// root, and reopens a fresh StateDB at that root.
func (b *GethStateBackend) Commit(block uint64) (common.Hash, error) {
	// Arcology can materialize storage-only accounts before code, balance, or
	// nonce is populated. Keep them during the compatibility-store commit.
	root, err := b.stateDB.Commit(block, false, false)
	if err != nil {
		return common.Hash{}, fmt.Errorf("commit Geth state: %w", err)
	}
	if err := b.trieDB.Commit(root, false); err != nil {
		return common.Hash{}, fmt.Errorf("flush Geth trie root %x: %w", root, err)
	}

	stateDB, err := state.New(root, b.database)
	if err != nil {
		return common.Hash{}, fmt.Errorf("reopen Geth state at root %x: %w", root, err)
	}
	b.stateDB = stateDB
	b.root = root

	return root, nil
}

// Close closes the trie backend and its owned disk database.
func (b *GethStateBackend) Close() error {
	if b == nil || b.trieDB == nil || !b.owned {
		return nil
	}
	return errors.Join(b.trieDB.Close(), b.disk.Close())
}
