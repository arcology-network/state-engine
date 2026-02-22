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
package harness

import (
	"errors"

	"github.com/arcology-network/common-lib/crdt/statecell"
	"github.com/arcology-network/common-lib/exp/slice"
	"github.com/arcology-network/state-engine/storage/proxy"
	"github.com/ethereum/go-ethereum/common/hexutil"

	// callee "github.com/arcology-network/scheduler/callee"
	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
	stateengine "github.com/arcology-network/state-engine"
	statecommon "github.com/arcology-network/state-engine/common"
	statecommitter "github.com/arcology-network/state-engine/state/committer"
)

func CreateAccountInStore(accts ...[20]byte) (*stateengine.StateStore, error) {
	sstore := stateengine.NewStateStore(proxy.NewMemDBStoreProxy())

	writeCache := sstore.ExecutionStateCache

	for _, sender := range accts {
		if _, err := statecommon.CreateDefaultPaths(1, hexutil.Encode(sender[:]), writeCache); err != nil { // NewAccount account structure {
			return nil, errors.New("Failed to create default paths for " + hexutil.Encode(sender[:]))
		}
	}

	raw := writeCache.Export(statecell.Sorter)
	acctTrans := statecell.StateCells(slice.Clone(raw)).To(statecell.InterProcTransition{})

	buffer := statecell.StateCells(acctTrans).Encode()
	statecell.StateCells{}.Decode(buffer)

	committer := statecommitter.NewStateCommitter(sstore, sstore.GetWriters())
	committer.Import(acctTrans)
	committer.DebugPrecommit([]uint64{1})
	committer.DebugCommit(10)
	return sstore, nil
}

// InjectTransitions creates accounts in the state store by injecting state transitions.
func InjectTransitions(sstore *stateengine.StateStore, keys []string, vals []crdtcommon.CRDT) error {
	writeCache := sstore.ExecutionStateCache
	var aggregatedErr error
	for i := range keys {
		_, err := writeCache.Inject(statecommon.SYSTEM, keys[i], vals[i])
		aggregatedErr = errors.Join(aggregatedErr, err)
	}

	acctTrans := writeCache.Export(statecell.Sorter)
	committer := statecommitter.NewStateCommitter(sstore, sstore.GetWriters())
	committer.Import(acctTrans)
	committer.DebugPrecommit([]uint64{1})
	committer.DebugCommit(10) //block height 10
	return aggregatedErr
}
