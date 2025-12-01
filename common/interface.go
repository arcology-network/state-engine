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

package common

import (
	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
)

// DB Writer interface for writing data to storage.
// type Writer[T any] interface {
// 	Import([]T)
// 	Precommit(bool)
// 	Commit(uint64)
// 	IsSync() bool // If the writer is synchronous, it will block until the commit is done.
// 	Name() string
// }

// ReadOnlyStore interface for reading data from storage.
// type ReadOnlyStore interface {
// 	IfExists(string, uint64) bool                 // Check if the key exists in the source, which can be a cache or a storage.
// 	ReadStorage(string, any, uint64) (any, error) // Get from persistent storage directly.
// 	Retrieve(string, any, uint64) (any, error)    // Get from cache or persistent storage, with cache lookup first.
// 	Preload([]byte, uint64) any
// }

type Hasher func(crdtcommon.Type) []byte
