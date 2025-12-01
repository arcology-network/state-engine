/*
 *   Copyright (c) 2023 Arcology Network
 *   All rights reserved.

 *   Licensed under the Apache License, Version 2.0 (the "License");
 *   you may not use this file except in compliance with the License.
 *   You may obtain a copy of the License at

 *   http://www.apache.org/licenses/LICENSE-2.0

 *   Unless required by applicable law or agreed to in writing, software
 *   distributed under the License is distributed on an "AS IS" BASIS,
 *   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *   See the License for the specific language governing permissions and
 *   limitations under the License.
 */

// write_cache.go provides the implementation of StateCache, a read-only data readonlyBackend
// designed for caching key-value pairs in the Arcology Network storage committer module.
// It supports efficient retrieval, insertion, and management of cached data, including
// wildcard deletions, memory pooling, and integration with a readonlyBackend store. The StateCache
// is optimized for use in concurrent and multi-processor environments.
//
// Note: The StateCache itself is read-only; all updates are performed by the committer.
//

package cache

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	common "github.com/arcology-network/common-lib/common"
	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
	"github.com/arcology-network/common-lib/crdt/commutative"
	statecell "github.com/arcology-network/common-lib/crdt/statecell"
	"github.com/arcology-network/common-lib/exp/associative"
	mapi "github.com/arcology-network/common-lib/exp/map"
	mempool "github.com/arcology-network/common-lib/exp/mempool"
	slice "github.com/arcology-network/common-lib/exp/slice"
	statecommon "github.com/arcology-network/state-engine/common"
)

// StateCache stores the *per-execution* working set of StateCells for a single
// transaction or execution context.
//
// This layer sits between:
//   - the global committed storage (readonlyBackend), and
//   - the per-transaction read/write operations performed during execution.
//
// Responsibilities of StateCache:
//   - lazy materialization of StateCells (only create when accessed)
//   - wildcard/container-based inherited state resolution
//   - deletion propagation (parent wildcard delete → child logical delete)
//   - tracking local reads/writes for conflict detection
//   - maintaining deterministic execution order
//
// IMPORTANT:
//
//	StateCache is NOT global state. It is NOT the MPT. It is the *local
//	execution view* used to ensure correctness, determinism, and CRDT/container
//	semantics. Modifying its logic incorrectly can break replay determinism,
//	wildcard deletion behavior, lazy materialization rules, and conflict
//	detection invariants.
type StateCache struct {
	// readonlyBackend is the read-only storage interface for accessing *committed*
	// global state (MPT / flattened DB). Reads fall back to readonlyBackend when
	// a path is not found locally. readonlyBackend must never be mutated here.
	readonlyBackend crdtcommon.ReadOnlyStore

	// localCells stores lazily-materialized StateCells for the current
	// execution. A cell is added when:
	//   - it is accessed directly,
	//   - or ResolveWildcardDeletion determines that it inherits state from a
	//     wildcard-deleted parent container.
	//
	// localCells is the authoritative view for:
	//   - local reads/writes,
	//   - conflict detection,
	//   - determinism of state expansion,
	//   - deletion propagation to children.
	//
	// NOTE:
	//   Children are *not* materialized on parent delete. They appear here
	//   only when accessed ("lazy materialization"). This is critical for
	//   performance and deterministic replay.
	localCells map[string]*statecell.StateCell // Local KV lookup

	// pendingWildcardDeletes records wildcard delete operations encountered
	// during execution. Each entry contains:
	//   (generationID, wildcardPath)
	//
	// These deletes must be committed to global storage only after execution
	// completes, otherwise timing differences would break determinism.
	//
	// This queue ensures:
	//   - correct ordering of wildcard deletes,
	//   - no premature state mutation,
	//   - deferred deletion behavior for MPT/flattened storage commit.
	pendingWildcardDeletes []*associative.Pair[uint64, string] // Paths delete by wildcard

	// platform identifies the execution platform (e.g. ETH_PATH). StateCache
	// may apply platform-specific rules for storage layout, path normalization,
	// or encoding behavior.
	platform statecommon.Platform

	// pool is an object pool for allocating StateCell instances efficiently.
	// StateCells are created frequently—using an object pool minimizes GC
	// overhead and reduces allocation churn during execution.
	pool *mempool.Mempool[*statecell.StateCell]

	// stateVersion tracks the version of the state being accessed or modified.
	// It is the block number for Ethereum storage.
	stateVersion uint64
}

// StateCache holds the per-execution working set of StateCells.
// This is the local view of state used during transaction execution.
// It supports lazy materialization, wildcard delete inheritance,
// and correct conflict detection.
func NewStateCache(readonlyBackend crdtcommon.ReadOnlyStore, perPage int, numPages int, args ...any) *StateCache {
	return &StateCache{
		stateVersion:           statecommon.LATEST_STATE_VERSION, // Default to the latest state version
		readonlyBackend:        readonlyBackend,
		localCells:             make(map[string]*statecell.StateCell),
		pendingWildcardDeletes: make([]*associative.Pair[uint64, string], 0),
		platform:               *statecommon.NewPlatform(),
		pool: mempool.NewMempool(perPage, numPages, func() *statecell.StateCell {
			return new(statecell.StateCell)
		}, (&statecell.StateCell{}).Reset),
	}
}

func (this *StateCache) SetReadOnlyBackend(backend crdtcommon.ReadOnlyStore) *StateCache {
	this.readonlyBackend = backend
	return this
}

func (this *StateCache) GetStateVersion() uint64        { return this.stateVersion }
func (this *StateCache) SetStateVersion(version uint64) { this.stateVersion = version }

func (this *StateCache) AddToDict(v *statecell.StateCell)        { this.localCells[*v.GetPath()] = v }
func (this *StateCache) ReadOnlyStore() crdtcommon.ReadOnlyStore { return this.readonlyBackend }
func (this *StateCache) Cache() *map[string]*statecell.StateCell { return &this.localCells }
func (this *StateCache) Preload([]byte, uint64) any {
	panic("Not implemented yet")
} // Preload from the underlying storage

// Placeholder
func (this *StateCache) NewStateCell() *statecell.StateCell { return this.pool.New() }

// Check if the current entry is in its parents' records. This is used when
// the entry is deleted through a wildcard deletion, in this case, if the
// entry is not in the write cache,
// it won't be touched, but it is not in the parent records to mark it as deleted.

// Recursively check IS NOT supported yet. It is not fully implemented for multi-level containers.
// But the single level is fine to use.

// Imagine a path like /a/b/c/d. We delete all the sub paths of /a/*
// and because c and d are not in the write cache, they are not touched, only marked
// as deleted in the parent path which is a's child list. But they may still be in the storage.
// So when we check if they still exist and if we only query by their paths directly we can
// still find them and their immediate parent path also exists, although their grandparent path
// are gone. Unless we recursively check the parent paths, we can't tell if they are truly gone.
// This requires a lot of queries and decoding.
func (this *StateCache) ExistsInParent(path string) bool {
	// No metadata for immediate children of system paths.
	if this.platform.IsImmediateChildOfSysPath(path) {
		return true
	}

	parentPath, _ := statecommon.GetParentPath(path) // Get the parent path
	if meta, _, _ := this.LookupOrMaterialize(0, parentPath, new(commutative.Path), nil); meta != nil {
		childKey := path[len(parentPath):]
		if ok, _ := meta.(*commutative.Path).Exists(childKey); ok { // Add the path to the parent path
			return ok
		}
	}
	return false
}

// Get the raw value directly, put it in an empty cell without recording
// the access. `Won't` update the localCells.
func (this *StateCache) LookupForRead(tx uint64, path string, T any, do func(*statecell.StateCell)) (any, *statecell.StateCell, bool) {
	// If the path doesn't exist in the parent snapshot at all (no committed
	// value, no wildcard/container inheritance), just return an empty,
	// non-cached cell.
	if !this.ExistsInParent(path) {
		return nil, this.NewStateCell().Init(tx, path, 0, 0, 0, nil, false), false
	}

	// For existing paths (including those affected by wildcard/container
	// semantics), reuse the full resolution pipeline. This may materialize
	// a StateCell into localCells, but the caller is still "read-only" from
	// a semantics point of view.
	return this.LookupOrMaterialize(tx, path, T, do) // Find the value in the cache
}

// FindForWrite resolves the StateCell for `path` through a 3-step lookup:
//
//  1. localCells (already materialized in this cache)
//  2. wildcard-inherited deletion (lazy materialization)
//  3. committed storage readonlyBackend
//
// It may insert a new StateCell into localCells when a wildcard/container
// delete implies a *logical* deleted state for this path.
func (this *StateCache) LookupOrMaterialize(tx uint64, path string, T any, do func(*statecell.StateCell)) (any, *statecell.StateCell, bool) {
	if cell, ok := this.localCells[path]; ok {
		return cell.Value(), cell, true // From cache
	}

	//  This is a LAZY materialization step, mainly for performance.

	// Handles inherited state for a path that is NOT present in localCells,
	// but whose parent matches a wildcard container that has been deleted.
	//
	// When a container path (e.g. "x/*") is deleted, all of its children become
	// *logically* deleted. However, these children are **not** materialized in any
	// execution cache at delete time. If we expanded all children eagerly, every
	// Execution Unit (each running its own EVM + cache) would end up materializing
	// the same millions of child paths.
	//
	// This would overwhelm the entire state engine:
	//   - massive duplicated deletions sent to conflict detection,
	//   - huge repeated expansion work in each EU,
	//   - unnecessary state growth across all execution contexts,
	//
	// To avoid this, Arcology uses **lazy materialization**:
	//
	//   - A child StateCell is created only when that path is actually accessed.
	//   - If the parent container was deleted, the child inherits the deleted state.
	//   - The derived StateCell is inserted into localCellCache.
	//
	// This prevents duplicated work across parallel executors, keeps state expansion
	// deterministic, and avoids saturating the conflict-detection and storage engine.
	if ok, cell := this.ResolveWildcardDeletion(path, T); ok {
		this.localCells[path] = cell // Add to the cache because should logically already be there.
		return cell.Value(), cell, false
	}

	// Fallback: load from the committed store.
	cell := this.LoadFromCommitted(tx, path, T)
	if do != nil {
		do(cell) // Call the callback function if provided
	}
	return cell.Value(), cell, false
}

// Write applies newVal to path, tracks size delta, and invokes optional callback.
func (this *StateCache) Write(tx uint64, path string, newVal any, args ...any) (int64, error) {
	if newVal != nil && newVal.(crdtcommon.Type).TypeID() == uint8(reflect.Invalid) { // Neither a valid replacement nor a delete operation.
		return 0, errors.New("Error: Unknown data type !")
	}

	cell, err := this.write(tx, path, newVal)
	sizeDif := this.DiffSize(tx, path, newVal) // Update the size difference
	if len(args) > 0 && args[0] != nil {
		args[0].(func(*statecell.StateCell, int64))(cell, sizeDif) // Call the callback function if provided
	}
	return sizeDif, err
}

// write stores a value in the state cache at the specified path, creating or materializing the associated state cell,
// ensuring parent metadata is updated when necessary, and propagating transient status from the parent path; it fails if
// the parent path is missing and the transaction is not SYSTEM.
func (this *StateCache) write(tx uint64, path string, value any) (*statecell.StateCell, error) {
	parentPath, _ := statecommon.GetParentPath(path)
	cell := statecell.NewStateCell(tx, path, 0, 1, 0, value, nil)                                // Default cell wrapper
	if this.IfExists(parentPath, statecommon.LATEST_STATE_VERSION) || tx == statecommon.SYSTEM { // The parent path exists or to inject the path directly
		var err error
		var inCache bool

		// Not a special expression, just a value update.
		if !strings.HasSuffix(path, "*") && !strings.HasSuffix(path, "[:]") {
			_, cell, inCache = this.LookupOrMaterialize(tx, path, value, this.AddToDict) // Get a cell wrapper
			err = cell.Set(tx, path, value, inCache, this)                               // set the new value
		}

		// Update the parent path meta
		if err == nil {
			// Only track of the children of concurrent paths.
			// blcc://eth1.0/account/0x123456790/container/ all the sub paths under container will need to check if
			// their parent path exists. The parent path missing issue actually only happens beyond the context of
			// the crdt domain. For a crdt container initated using the library api, its parent path always exists.
			if strings.HasSuffix(parentPath, "/container/") || !this.platform.IsSysPath(parentPath) && tx != statecommon.SYSTEM {
				_, parentMeta, inCache := this.LookupOrMaterialize(tx, parentPath, new(commutative.Path), this.AddToDict)
				err = parentMeta.Set(tx, path, cell.Value(), inCache, this)
			}

			// Set Transient Status based on its parent path settings. A transient path will not be persisted after
			// the a generation or a block is committed. This makes it different from either a normal state updates or
			// a memory variable.
			if pathMeta, _, _ := this.LookupForRead(tx, parentPath, new(commutative.Path), nil); pathMeta != nil { // Get the parent path meta
				cell.SetBlockBound(pathMeta.(*commutative.Path).IsBlockBound()) // Use the parent path transient status to set the current path
			}
		}
		return cell, err
	}
	return cell, errors.New("Error: The parent path " + parentPath + " doesn't exist for " + path)
}

// Get the raw value directly WITHOUT tracking the accessing record.
// Users need to count access themselves.
func (this *StateCache) Retrieve(path string, T any, _ uint64) (any, error) {
	typedv, _, _ := this.LookupForRead(statecommon.SYSTEM, path, T, nil)
	if typedv == nil || typedv.(crdtcommon.Type).IsDeltaApplied() {
		return typedv, nil
	}

	// Special treatment for the commutative.Path.
	// In general, value types need to be fully cloned as well, so they be
	// manipulated without affecting the original value. But this doesn't apply
	// to the commutative.Path, which has its own change tracking mechanism.
	// The clone() here isn't going to clone everything inside the Path, but just
	// the delta part. The committed part is still shared to save memory.
	if common.IsType[*commutative.Path](typedv) {
		return typedv.(*commutative.Path).Clone(), nil
	}

	// Make a Deep copy of the original value.
	rawv, _, _ := typedv.(crdtcommon.Type).Get()
	min, max := typedv.(crdtcommon.Type).Limits()
	return typedv.(crdtcommon.Type).New(rawv, nil, nil, min, max), nil // Clone the value
}

// The load the data from the readonlyBackend. Since the state is already isCommitted, it is read-only.
// No need to add it to the localCells or keep track of the access.
func (this *StateCache) LoadFromCommitted(tx uint64, path string, T any) *statecell.StateCell {
	var typedv any
	if readonlyBackend := this.ReadOnlyStore(); readonlyBackend != nil {
		typedv, _ = readonlyBackend.Retrieve(path, T, this.stateVersion) // The readonlyBackend could also be another instance of StateCache.
	}
	return this.NewStateCell().Init(tx, path, 0, 0, 0, typedv, typedv != nil)
}

// This function specifically retrieves the value from the readonlyBackend without any tracking.
func (this *StateCache) ReadStorage(key string, T any, version uint64) (any, error) {
	if this.readonlyBackend != nil {
		return this.readonlyBackend.ReadStorage(key, T, version)
	}
	return nil, errors.New("Error: The readonlyBackend is nil")
}

func (this *StateCache) Read(tx uint64, path string, T any) (any, any, uint64) {
	_, stcell, _ := this.LookupForRead(tx, path, T, this.AddToDict) // Get the cell wrapper

	// need to check if it is in the memory. If so gas price should be 3 instead.
	dataSize := statecommon.MIN_READ_SIZE
	if typedv := stcell.Value(); typedv != nil {
		dataSize = typedv.(crdtcommon.Type).MemSize()
	}
	return stcell.Get(tx, path, nil), stcell, dataSize
}

// DiffSize returns the memory size delta between the current cell value and newVal.
// This is used for tracking memory usage changes in the StateCache and calculating fees.
func (this *StateCache) DiffSize(tx uint64, path string, newVal any) int64 {
	oldSize := int64(0)
	if oldVal, _, _ := this.LookupForRead(tx, path, newVal, nil); oldVal != nil {
		oldSize += int64(oldVal.(crdtcommon.Type).MemSize())
	}

	newSize := int64(0)
	if newVal != nil {
		newSize = int64(newVal.(crdtcommon.Type).MemSize())
	}

	return newSize - oldSize
}

// Get the raw value directly, skip the access counting at the cell level
func (this *StateCache) GetIfCached(path string) (any, bool) {
	cell, ok := this.localCells[path]
	return cell, ok
}

// Check if the path exists in the writecache or the readonlyBackend.
// No access count is recorded. Only for internal use. Not exposed to the public API.
func (this *StateCache) IfExists(path string, _ uint64) bool {
	// Any path shorter than the ETH_ACCOUNT_PREFIX is a system path.
	if statecommon.ETH_ACCOUNT_PREFIX_LENGTH >= len(path) {
		return true
	}

	if v := this.localCells[path]; v != nil {
		return v.Value() != nil // If value == nil means either it's been deleted or never existed.
	}

	if this.readonlyBackend == nil {
		return false
	}

	flag := this.readonlyBackend.IfExists(path, statecommon.LATEST_STATE_VERSION) //this.RetrieveShallow(path, nil) != nil
	return flag
}

// The function is used to add the transitions to the writecache. It assumes that the transition's
// parent path has been added to the writecache already. Otherwise, it won't succeed.
func (this *StateCache) set(v *statecell.StateCell) *StateCache {
	if v == nil {
		return this
	}

	if common.IsPath(*v.GetPath()) && v.IsCommitted() {
		return this
	}

	(*v).CopyTo(this)
	return this
}

// The function is used to add the transitions to the writecache, which usually comes from
// the child writecaches. It usually happens with the sub processeses are completed.
func (this *StateCache) Insert(transitions []*statecell.StateCell) *StateCache {
	if len(transitions) == 0 {
		return this
	}

	// Filter out the path creations transitions as they will be treated differently.
	newPathCreations := slice.MoveIf(&transitions, func(_ int, v *statecell.StateCell) bool {
		return common.IsPath(*v.GetPath()) && !v.IsCommitted()
	})

	// Not necessary to sort the path creations at the moment,
	// but it is good for the future if multiple level containers are available
	newPathCreations = statecell.StateCells(statecell.Sorter(newPathCreations))
	slice.Foreach(newPathCreations, func(_ int, v **statecell.StateCell) {
		(*v).CopyTo(this) // Write back to the parent writecache
	})

	// Remove the changes to the existing path meta, as they will be updated automatically
	// when inserting or deleting sub elements. This is just simpler and more straightforward
	// than to keep track of the meta changes and merge them back the meta changes.
	transitions = slice.RemoveIf(&transitions, func(_ int, v *statecell.StateCell) bool {
		return common.IsPath(*v.GetPath())
	})

	// Write back to the parent writecache
	slice.Foreach(transitions, func(_ int, v **statecell.StateCell) {
		(*v).CopyTo(this)
	})
	return this
}

// Reset the writecache to the initial state for the next round of processing.
func (this *StateCache) Clear() *StateCache {
	this.pool.Reset()
	clear(this.localCells)
	return this
}

func (this *StateCache) Equal(other *StateCache) bool {
	thisBuffer := mapi.Values(this.localCells)
	sort.SliceStable(thisBuffer, func(i, j int) bool {
		return *thisBuffer[i].GetPath() < *thisBuffer[j].GetPath()
	})

	otherBuffer := mapi.Values(other.localCells)
	sort.SliceStable(otherBuffer, func(i, j int) bool {
		return *otherBuffer[i].GetPath() < *otherBuffer[j].GetPath()
	})

	cacheFlag := reflect.DeepEqual(thisBuffer, otherBuffer)
	return cacheFlag
}

// Export the content of the writecache to two arrays of cells.
// One for the accesses and the other for the transitions.
func (this *StateCache) Export(preprocs ...func([]*statecell.StateCell) []*statecell.StateCell) []*statecell.StateCell {
	buffer := mapi.Values(this.localCells)
	for _, proc := range preprocs {
		buffer = common.IfThenDo1st(proc != nil, func() []*statecell.StateCell {
			return proc(buffer)
		}, buffer)
	}
	slice.RemoveIf(&buffer, func(_ int, v *statecell.StateCell) bool {
		return v.PathLookupOnly() // Remove peeks and local values
	})

	// statecell.StateCell(buffer).PrintUnsorted() // For debugging purpose
	buffer = append(buffer, this.WildcardsToStateCell()...)
	return buffer
}

// For the testing purpose, export the content of the writecache to two arrays of cells and filter.
func (this *StateCache) ExportAll(preprocs ...func([]*statecell.StateCell) []*statecell.StateCell) ([]*statecell.StateCell, []*statecell.StateCell) {
	all := this.Export()
	accesses := statecell.StateCells(slice.Clone(all)).To(statecell.ITAccess{})
	transitions := statecell.StateCells(slice.Clone(all)).To(statecell.ITTransition{})
	return accesses, transitions
}

func (this *StateCache) KVs() ([]string, []crdtcommon.Type) {
	transitions := statecell.StateCells(slice.Clone(this.Export(statecell.Sorter))).To(statecell.ITTransition{})

	values := make([]crdtcommon.Type, len(transitions))
	keys := slice.ParallelTransform(transitions, 4, func(i int, v *statecell.StateCell) string {
		values[i] = v.Value().(crdtcommon.Type)
		return *v.GetPath()
	})
	return keys, values
}

// This function is used to write the cache to the data source directly to bypass all the intermediate steps,
// including the conflict detection.
func (this *StateCache) Print() {
	values := mapi.Values(this.localCells)
	sort.SliceStable(values, func(i, j int) bool {
		return *values[i].GetPath() < *values[j].GetPath()
	})

	for i, elem := range values {
		fmt.Println("Level : ", i)
		elem.Print()
	}
}

// Calculate the checksum of the writecache for integrity check.
func (this *StateCache) Checksum() [32]byte {
	values := mapi.Values(this.localCells)
	sort.SliceStable(values, func(i, j int) bool {
		return *values[i].GetPath() < *values[j].GetPath()
	})
	return statecell.StateCells(values).Checksum()
}

// Read the value from the readonlyBackend. This function is used for
// GetCommittedState() in Eth interface for gas refund related code.
func (this *StateCache) ReadCommitted(tx uint64, key string, T any) (any, uint64) {
	// Just to leave a record for conflict detection. This is different from the original Ethereum implementation.
	// In Ethereum, there is no such concept as the multiprocessor，so the isCommitted state can only come from the
	// previous block or the transactions before the current one. But in the multiprocessor, the isCommitted state
	// may also come from the parent thread. So we need to leave a record for the conflict detection in case that
	// threads spawned by multiple parent are trying to access the same path.
	if v := this.LoadFromCommitted(tx, key, this); v != nil { // Check to see if the path exists in the readonlyBackend.
		return v.Get(tx, key, nil), 0
	}
	return nil, 0
}
