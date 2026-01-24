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
package proxy

import (
	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
	statecell "github.com/arcology-network/common-lib/crdt/statecell"
	"github.com/arcology-network/state-engine/storage/ethstorage"
	ethstg "github.com/arcology-network/state-engine/storage/ethstorage"
)

// EthStateVersion represents a specific version of the Ethereum world state at a given block.
// It is mainly used for historical state queries and transaction execution.
type EthStateVersion struct {
	*ethstg.EthWorldState
}

func NewEthStateVersion(rootHash [32]byte, backend *ethstg.EthShardTrieDB) (*EthStateVersion, error) {
	ethWorldState, err := ethstg.LoadEthTrieByRoot(backend.MainTrieDB(), rootHash)
	if err != nil {
		return nil, err
	}

	return &EthStateVersion{
		EthWorldState: ethWorldState,
	}, nil
}

func (this *EthStateVersion) GetWriters() []crdtcommon.Writer[*statecell.StateCell] {
	return []crdtcommon.Writer[*statecell.StateCell]{
		ethstorage.NewEthStorageWriter(this.EthWorldState, -1, (&StorageProxy{}).EthOnly),
	}
}
