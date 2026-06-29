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

// write_cache.go provides the implementation of ExecutionStateStore, a read-only data committedStore
// designed for caching key-value pairs in the Arcology Network storage committer module.
// It supports efficient retrieval, insertion, and management of cached data, including
// wildcard deletions, memory pooling, and integration with a committedStore store. The ExecutionStateStore
// is optimized for use in concurrent and multi-processor environments.
//
// Note: The ExecutionStateStore itself is read-only; all updates are performed by the committer.
//

package cache

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	libcommon "github.com/arcology-network/common-lib/common"
	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
	"github.com/arcology-network/common-lib/crdt/commutative"
	"github.com/arcology-network/common-lib/crdt/noncommutative"
	statecell "github.com/arcology-network/common-lib/crdt/statecell"
	"github.com/arcology-network/common-lib/exp/associative"
	mapi "github.com/arcology-network/common-lib/exp/map"
	mempool "github.com/arcology-network/common-lib/exp/mempool"
	slice "github.com/arcology-network/common-lib/exp/slice"
	stgintf "github.com/arcology-network/common-lib/storage/interface"
	statecommon "github.com/arcology-network/state-engine/common"
	ethrlp "github.com/arcology-network/state-engine/storage/codec/ethcodec/rlp"
	"github.com/arcology-network/state-engine/storage/proxy"
	"github.com/cespare/xxhash"
)

// ExecutionStateStore stores the *per-execution* working set of StateCells for a single
// transaction or execution context.
//
// This layer sits between:
//   - the global committed storage (committedStore), and
//   - the per-transaction read/write operations performed during execution.
//
// Responsibilities of ExecutionStateStore:
//   - lazy materialization of StateCells (only create when accessed)
//   - wildcard/container-based inherited state resolution
//   - deletion propagation (parent wildcard delete → child logical delete)
//   - tracking local reads/writes for conflict detection
//   - maintaining deterministic execution order
//
// IMPORTANT:
//
//	ExecutionStateStore is NOT global state. It is NOT the MPT. It is the *local
//	execution view* used to ensure correctness, determinism, and CRDT/container
//	semantics. Modifying its logic incorrectly can break replay determinism,
//	wildcard deletion behavior, lazy materialization rules, and conflict
//	detection invariants.
type ExecutionStateStore struct {
	// committedStore is the read-only storage interface for accessing *committed*
	// global state (MPT / flattened DB). Reads fall back to committedStore when
	// a path is not found locally. committedStore must never be mutated here.
	committedStore stgintf.ReadOnlyStore[string, crdtcommon.CRDT]

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

	// platform identifies the execution platform (e.g. ETH_PATH). ExecutionStateStore
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

	// The serial ID of the state cache, it is the same as the execution unit ID, since
	// each execution unit has its own state cache.
	id uint64

	// *committer.StateCommitter
}

// ExecutionStateStore holds the per-execution working set of StateCells.
// This is the local view of state used during transaction execution.
// It supports lazy materialization, wildcard delete inheritance,
// and correct conflict detection.
func NewExecutionStateStore(
	committedStore stgintf.ReadOnlyStore[string, crdtcommon.CRDT],
	perPage int,
	numPages int,
	args ...any,
) *ExecutionStateStore {
	return &ExecutionStateStore{
		id:                     0,
		stateVersion:           statecommon.LATEST_STATE_VERSION, // Default to the latest state version
		committedStore:         committedStore,
		localCells:             make(map[string]*statecell.StateCell),
		pendingWildcardDeletes: make([]*associative.Pair[uint64, string], 0),
		platform:               *statecommon.NewPlatform(),
		pool: mempool.NewMempool(perPage, numPages, func() *statecell.StateCell {
			return new(statecell.StateCell)
		}, (&statecell.StateCell{}).Reset),
	}
}

func NewDefaultExecutionStateStore(backend proxy.VersionedStore) *ExecutionStateStore {
	return NewExecutionStateStore(
		backend,
		16,
		1,
		func(k string) uint64 {
			return xxhash.Sum64String(k)
		},
	)
}

func (this *ExecutionStateStore) SetCommittedStore(backend stgintf.ReadOnlyStore[string, crdtcommon.CRDT]) *ExecutionStateStore {
	this.committedStore = backend
	return this
}

func (this *ExecutionStateStore) GetID() uint64   { return this.id }
func (this *ExecutionStateStore) SetID(id uint64) { this.id = id }

func (this *ExecutionStateStore) GetVersion() uint64        { return this.stateVersion }
func (this *ExecutionStateStore) SetVersion(version uint64) { this.stateVersion = version }

func (this *ExecutionStateStore) addToLocalCache(v *statecell.StateCell) {
	this.localCells[*v.GetPath()] = v
}

func (this *ExecutionStateStore) CommittedStore() stgintf.ReadOnlyStore[string, crdtcommon.CRDT] {
	return this.committedStore
}

func (this *ExecutionStateStore) Cache() *map[string]*statecell.StateCell { return &this.localCells }

// func (this *ExecutionStateStore) Preload(key []byte) any {
// 	return this.committedStore.Preload(key)
// }

// Placeholder
func (this *ExecutionStateStore) NewStateCell() *statecell.StateCell { return this.pool.New() }

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
func (this *ExecutionStateStore) ExistsInParent(tx uint64, path string, do func(*statecell.StateCell)) bool {
	// No metadata for immediate children of system paths.
	if this.platform.IsImmediateChildOfSysPath(path) {
		return true
	}

	typeHint := new(commutative.Path)
	parentPath, _ := statecommon.GetParentPath(path) // Get the parent path
	if meta, _, _ := this.ResolveCellForRead(tx, parentPath, typeHint, do, true); meta != nil {
		if libcommon.IsType[*noncommutative.Bytes](meta) {
			meta, _ = ethrlp.RlpCodec{}.Decode("", *(meta.(*noncommutative.Bytes)), typeHint)
		}

		childKey := path[len(parentPath):]
		if ok, _ := meta.(*commutative.Path).Exists(childKey); ok { // Add the path to the parent path
			return ok
		}
	}
	return false
}

// Get the raw value directly, put it in an empty cell without recording
// the access. `Won't` update the localCells.
func (this *ExecutionStateStore) ReadCell(
	tx uint64,
	path string,
	T crdtcommon.CRDT,
	do func(*statecell.StateCell),
) (any, *statecell.StateCell, error) {
	// If the path doesn't exist in the parent snapshot at all (no committed
	// value, no wildcard/container inheritance), just return an empty,
	// non-cached cell.
	if !this.ExistsInParent(tx, path, do) {
		return nil, this.NewStateCell().Init(tx, path, 0, 0, 0, nil, false), stgintf.ErrNotInParent
	}

	// For existing paths (including those affected by wildcard/container
	// semantics), reuse the full resolution pipeline. This may materialize
	// a StateCell into localCells, but the caller is still "read-only" from
	// a semantics point of view.
	return this.ResolveCellForRead(tx, path, T, do, true) // Find the value in the cache
}

// FindForWrite resolves the StateCell for `path` through a 3-step lookup:
//
//  1. localCells (already materialized in this cache)
//  2. wildcard-inherited deletion (lazy materialization)
//  3. committed storage committedStore
//
// It may insert a new StateCell into localCells when a wildcard/container
// delete implies a *logical* deleted state for this path.
func (this *ExecutionStateStore) ResolveCellForRead(
	tx uint64,
	path string,
	T any,
	do func(*statecell.StateCell),
	cached bool,
) (any, *statecell.StateCell, error) {
	if cell, ok := this.localCells[path]; ok {
		return cell.Value(), cell, nil // From cache
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
	if ok, cell, err := this.ResolveWildcardDeletion(path, T); ok && cached {
		this.localCells[path] = cell   // Add to the cache because should have logically been there already.
		return cell.Value(), cell, err // From wildcard delete, not found in the committed store
	}

	// Fallback: load from the committed store.
	cell, err := this.getFromCommitted(tx, path, T)
	if do != nil {
		do(cell) // Call the callback function if provided
	}
	return cell.Value(), cell, err
}

// Read the value from the committedStore. This function is used for
// GetCommittedState() in Eth interface for gas refund related code.
func (this *ExecutionStateStore) ReadCommitted(tx uint64, path string, T any) (any, uint64) {
	// Just to leave a record for conflict detection. This is different from the original Ethereum implementation.
	// In Ethereum, there is no such concept as the multiprocessor，so the isCommitted state can only come from the
	// previous block or the transactions before the current one. But in the multiprocessor, the isCommitted state
	// may also come from the parent thread. So we need to leave a record for the conflict detection in case that
	// threads spawned by multiple parent are trying to access the same path.
	dataSize := statecommon.MIN_READ_SIZE

	stcell, err := this.getFromCommitted(tx, path, T) // Check to see if the path exists in the committedStore.
	if err != nil {
		return nil, dataSize
	}

	if typedv := stcell.Value(); typedv != nil {
		dataSize += typedv.(crdtcommon.CRDT).MemSize()
	}
	return stcell.Get(tx, path, nil), dataSize
}

// The load the data from the committedStore. Since the state is already isCommitted, it is read-only.
// No need to add it to the localCells or keep track of the access.
func (this *ExecutionStateStore) getFromCommitted(tx uint64, path string, T any) (*statecell.StateCell, error) {
	var typedv any
	var err error
	if committedStore := this.CommittedStore(); committedStore != nil {
		typedv, err = this.CommittedStore().GetAs(path, T)
	}
	return this.NewStateCell().Init(tx, path, 0, 0, 0, typedv, typedv != nil), err
}

// Write applies newVal to path, tracks size delta, and invokes optional callback.
func (this *ExecutionStateStore) Write(tx uint64, path string, newVal crdtcommon.CRDT, setter any) (int64, error) {
	if newVal != nil && newVal.TypeID() == uint8(reflect.Invalid) { // Neither a valid replacement nor a delete operation.
		return 0, errors.New("Error: Unknown data type !")
	}

	cell, err := this.write(tx, path, newVal)
	sizeDif := this.DiffSize(tx, path, newVal) // Update the size difference
	if setter != nil {
		setter.(func(*statecell.StateCell, int64))(cell, sizeDif) // Call the callback function if provided
	}
	return sizeDif, err
}

// Inject forcibly sets the value at path to value, bypassing normal
// lookup logic. This is used for initializing new paths or special
// system operations.
//
// Use it with SPECIAL care!!!
func (this *ExecutionStateStore) Inject(tx uint64, path string, v crdtcommon.CRDT) (*statecell.StateCell, error) {
	_, cell, _ := this.ResolveCellForRead(tx, path, v, this.addToLocalCache, true) // Get a cell wrapper
	err := cell.Set(tx, path, v, this)                                             // set the new value
	return cell, err
}

// write stores a value in the state cache at the specified path, creating or materializing the associated state cell,
// ensuring parent metadata is updated when necessary, and propagating transient status from the parent path; it fails if
// the parent path is missing and the transaction is not SYSTEM.
func (this *ExecutionStateStore) write(tx uint64, path string, value crdtcommon.CRDT) (*statecell.StateCell, error) {
	parentPath, _ := statecommon.GetParentPath(path)
	cell := statecell.NewStateCell(tx, path, 0, 1, 0, value, nil) // Default cell wrapper
	if this.Has(parentPath) || tx == statecommon.SYSTEM {         // The parent path exists or to inject the path directly
		var err error
		// var inCache bool

		// Not a special expression, just a value update.
		if !strings.HasSuffix(path, "*") && !strings.HasSuffix(path, "[:]") {
			_, cell, _ = this.ResolveCellForRead(tx, path, value, this.addToLocalCache, true) // Get a cell wrapper
			err = cell.Set(tx, path, value, this)                                             // set the new value
		}

		// Update the parent path meta
		if err == nil {
			// IMPORTANT: Track child membership only for CONCURRENT paths.
			//
			// blcc://eth1.0/account/0x123456790/container/ all the sub paths under container will need to check if
			// their parent path exists. The parent path missing issue actually only happens beyond the context of
			// the crdt domain. For a crdt container initated using the library api, its parent path always exists.
			if this.platform.IsContainerPath(parentPath) ||
				!this.platform.IsSysPath(parentPath) && tx != statecommon.SYSTEM {
				_, parentMeta, _ := this.ResolveCellForRead(
					tx,
					parentPath,
					new(commutative.Path),
					this.addToLocalCache,
					true,
				)
				err = parentMeta.Set(tx, path, cell.Value(), this)
			}

			// Set Transient Status based on its parent path settings. A transient path will not be persisted after
			// the a generation or a block is committed. This makes it different from either a normal state updates or
			// a memory variable.
			if this.platform.IsContainerPath(parentPath) { // Only applies to the container paths.
				var pathMeta any
				pathMeta, _, err = this.ReadCell(
					tx,
					parentPath,
					new(commutative.Path),
					nil,
				)

				if pathMeta != nil { // Get the parent path meta
					// Use the parent path transient status to set the current path.
					// A block bound path will only live in the current block.
					// Nothing beyond the block can see it. So it is a special type of TRANSIENT path.
					cell.SetBlockBound(pathMeta.(*commutative.Path).IsBlockBound()) //
				}
			}
		}
		return cell, err
	}
	return cell, errors.New("Error: The parent path " + parentPath + " doesn't exist for " + path)
}

// ExecutionStateStore can be used as a readonly backend in a cascading set, so we need
// this interface to stay compatible.
func (this *ExecutionStateStore) Get(path string) (any, error) {
	// The data backend may return a raw value instead of a CRDT. This is a normal
	// Behavior for the eth storage.
	typedv, _, _ := this.ReadCell(statecommon.SYSTEM, path, nil, nil)

	if !libcommon.IsType[crdtcommon.CRDT](typedv) || // can be a nil, or a raw []byte depending on the backend type.
		typedv.(crdtcommon.CRDT).IsDeltaApplied() {
		return typedv, nil
	}

	// Special treatment for the commutative.Path.
	// In general, value types need to be fully cloned as well, so they be
	// manipulated without affecting the original value. But this doesn't apply
	// to the commutative.Path, which has its own change tracking mechanism.
	// The clone() here isn't going to clone everything inside the Path, but just
	// the delta part. The committed part is still shared to save memory.
	if libcommon.IsType[*commutative.Path](typedv) {
		return typedv.(*commutative.Path).Clone(), nil
	}

	// Make a Deep copy of the original value.
	rawv, _, _ := typedv.(crdtcommon.CRDT).Get()
	min, max := typedv.(crdtcommon.CRDT).Limits()
	return typedv.(crdtcommon.CRDT).New(rawv, nil, nil, min, max), nil // Clone the value
}

func (this *ExecutionStateStore) GetAs(path string, typeHint any) (any, error) {
	var typedHint crdtcommon.CRDT
	if hint, ok := typeHint.(crdtcommon.CRDT); ok {
		typedHint = hint
	}

	typedv, _, _ := this.ReadCell(statecommon.SYSTEM, path, typedHint, nil)

	if !libcommon.IsType[crdtcommon.CRDT](typedv) ||
		typedv.(crdtcommon.CRDT).IsDeltaApplied() {
		return typedv, nil
	}

	if libcommon.IsType[*commutative.Path](typedv) {
		return typedv.(*commutative.Path).Clone(), nil
	}

	rawv, _, _ := typedv.(crdtcommon.CRDT).Get()
	min, max := typedv.(crdtcommon.CRDT).Limits()
	return typedv.(crdtcommon.CRDT).New(rawv, nil, nil, min, max), nil
}

func (this *ExecutionStateStore) Read(tx uint64, path string, T crdtcommon.CRDT) (any, any, uint64) {
	_, stcell, _ := this.ReadCell(tx, path, T, this.addToLocalCache) // Get the cell wrapper

	// need to check if it is in the memory. If so gas price should be 3 instead.
	dataSize := statecommon.MIN_READ_SIZE
	if typedv := stcell.Value(); typedv != nil {
		dataSize += typedv.(crdtcommon.CRDT).MemSize()
	}
	return stcell.Get(tx, path, nil), stcell, dataSize
}

// DiffSize returns the memory size delta between the current cell value and newVal.
// This is used for tracking memory usage changes in the ExecutionStateStore and calculating fees.
func (this *ExecutionStateStore) DiffSize(tx uint64, path string, newVal crdtcommon.CRDT) int64 {
	oldSize := int64(0)
	if oldVal, _, _ := this.ReadCell(tx, path, newVal, nil); oldVal != nil {
		oldSize += int64(oldVal.(crdtcommon.CRDT).MemSize())
	}

	newSize := int64(0)
	if newVal != nil {
		newSize = int64(newVal.MemSize())
	}

	return newSize - oldSize
}

// Get the raw value directly, skip the access counting at the cell level
func (this *ExecutionStateStore) GetIfCached(path string) (any, bool) {
	cell, ok := this.localCells[path]
	return cell, ok
}

// Check if the path exists in the writecache or the committedStore.
// No access count is recorded. Only for internal use. Not exposed to the public API.
func (this *ExecutionStateStore) Has(path string) bool {
	// Any path shorter than the ETH_ACCOUNT_PREFIX is a system path.
	if statecommon.ETH_ACCOUNT_PREFIX_LENGTH >= len(path) {
		return true
	}

	if v := this.localCells[path]; v != nil {
		return v.Value() != nil // If value == nil means either it's been deleted or never existed.
	}

	if this.committedStore == nil {
		return false
	}

	flag := this.committedStore.Has(path) //this.GetAsShallow(path, nil) != nil
	return flag
}

// The function is used to add the transitions to the writecache. It assumes that the transition's
// parent path has been added to the writecache already. Otherwise, it won't succeed.
func (this *ExecutionStateStore) set(v *statecell.StateCell) *ExecutionStateStore {
	if v == nil {
		return this
	}

	if libcommon.IsPath(*v.GetPath()) && v.IsCommitted() {
		return this
	}

	(*v).CopyTo(this)
	return this
}

// The function is used to add the transitions to the writecache, which usually comes from
// the child writecaches. It usually happens with the sub processeses are completed.
func (this *ExecutionStateStore) Insert(transitions []*statecell.StateCell) *ExecutionStateStore {
	if len(transitions) == 0 {
		return this
	}

	// Filter out the path creations transitions as they will be treated differently.
	newPathCreations := slice.MoveIf(&transitions, func(_ int, v *statecell.StateCell) bool {
		return libcommon.IsPath(*v.GetPath()) && !v.IsCommitted()
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
		return libcommon.IsPath(*v.GetPath())
	})

	// Write back to the parent writecache
	slice.Foreach(transitions, func(_ int, v **statecell.StateCell) {
		(*v).CopyTo(this)
	})
	return this
}

// Reset the writecache to the initial state for the next round of processing.
func (this *ExecutionStateStore) Clear() *ExecutionStateStore {
	this.pool.Reset()
	clear(this.localCells)
	return this
}

func (this *ExecutionStateStore) Equal(other *ExecutionStateStore) bool {
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
func (this *ExecutionStateStore) Export(preprocs ...func([]*statecell.StateCell) []*statecell.StateCell) []*statecell.StateCell {
	buffer := mapi.Values(this.localCells)
	for _, proc := range preprocs {
		buffer = libcommon.IfThenDo1st(proc != nil, func() []*statecell.StateCell {
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
func (this *ExecutionStateStore) ExportAll(preprocs ...func([]*statecell.StateCell) []*statecell.StateCell) ([]*statecell.StateCell, []*statecell.StateCell) {
	all := this.Export()
	accesses := statecell.StateCells(slice.Clone(all)).To(statecell.InterThreadAccess{})
	transitions := statecell.StateCells(slice.Clone(all)).To(statecell.InterThreadTransition{})
	return accesses, transitions
}

func (this *ExecutionStateStore) KVs() ([]string, []crdtcommon.CRDT) {
	transitions := statecell.StateCells(slice.Clone(this.Export(statecell.Sorter))).To(statecell.InterThreadTransition{})

	values := make([]crdtcommon.CRDT, len(transitions))
	keys := slice.ParallelTransform(transitions, 4, func(i int, v *statecell.StateCell) string {
		values[i] = v.Value().(crdtcommon.CRDT)
		return *v.GetPath()
	})
	return keys, values
}

func (this *ExecutionStateStore) GetWriters() []stgintf.StoreWriter[*statecell.StateCell] {
	selfWriter := []stgintf.StoreWriter[*statecell.StateCell]{
		&ExecutionCacheWriter{
			ExecutionCacheIndexer: NewExecutionCacheIndexer(nil, int64(this.stateVersion), nil),
			ExecutionStateStore:   this,
		}}

	if this.committedStore == nil {
		return selfWriter
	}
	return append(selfWriter, this.committedStore.(proxy.VersionedStore).GetWriters()...)
}

// This function is used to write the cache to the data source directly to bypass all the intermediate steps,
// including the conflict detection.
func (this *ExecutionStateStore) Print() {
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
func (this *ExecutionStateStore) Checksum() [32]byte {
	values := mapi.Values(this.localCells)
	sort.SliceStable(values, func(i, j int) bool {
		return *values[i].GetPath() < *values[j].GetPath()
	})
	return statecell.StateCells(values).Checksum()
}
