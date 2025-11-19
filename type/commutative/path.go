/*
 *   Copyright (c) 2023 Arcology Network

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

package commutative

import (
	"crypto/sha256"
	"errors"

	codec "github.com/arcology-network/common-lib/codec"
	"github.com/arcology-network/common-lib/common"
	"github.com/arcology-network/common-lib/exp/orderedset"
	"github.com/arcology-network/common-lib/exp/slice"
	softdeltaset "github.com/arcology-network/common-lib/exp/softdeltaset"
	stgcommon "github.com/arcology-network/state-engine/common"
)

// The Path type is a special commutative type that represents a path in the concurrent storage.
// It keeps track of the all the sub paths that are added, removed or updated.
type Path struct {
	*softdeltaset.DeltaSet[string]
	ElemType     uint8  // System paths may hold different types, when ElemType == 0, it's up to the developers to decide.
	Expression   string // type id
	IsSysPath    bool   // If true, it is a system path, which may hold different types of elements.
	isBlockBound bool   // If true, it is not persisted to the storage after each block

	preloaded *orderedset.OrderedSet[string]
	TotalSize uint64 // The size of the elements under the path in bytes.
}

func NewPath(newPaths ...string) stgcommon.Type {
	this := &Path{
		ElemType: 0,
		DeltaSet: softdeltaset.NewDeltaSet("", 10, codec.Sizer, codec.EncodeTo, new(codec.String).DecodeTo, nil, newPaths...),
	}
	return this
}

// The entries that need to be deleted when the path is deleted.
func (this *Path) GetCascadeSub(prefix string, source any) []string {
	store := source.(interface {
		Retrieve(string, any) (any, error)
	})

	subElem := this.DeltaSet.Elements()
	elements := slice.Transform(subElem, func(_ int, k string) string { return prefix + k })
	pathStrs := []string{}
	for {
		subPaths := slice.MoveIf(&elements, func(_ int, k string) bool {
			return common.IsPath(k)
		})

		// Loop through the sub paths and check if they exist in the store.
		if len(subPaths) == 0 {
			pathStrs = append(pathStrs, elements...)
			break
		}

		for i := range subPaths {
			if path, _ := store.Retrieve(subPaths[i], new(Path)); path != nil {
				underSubPaths := path.(*Path).GetCascadeSub(subPaths[i], store)
				pathStrs = append(pathStrs, underSubPaths...)
			}
		}
	}
	return pathStrs
}

func (this *Path) Length() int                          { return int(this.DeltaSet.NonNilCount()) }
func (this *Path) View() *softdeltaset.DeltaSet[string] { return this.DeltaSet }
func (this *Path) MemSize() uint64                      { return uint64(this.DeltaSet.NonNilCount()) * 32 * 2 } // Just an estimate, need to update on fly instead of calculating everytime
func (this *Path) TypeID() uint8                        { return PATH }

func (this *Path) SetBlockBound(v bool) { this.isBlockBound = v }
func (this *Path) IsBlockBound() bool   { return this.isBlockBound } // If true, it is not persisted to the storage after each block
func (this *Path) IsDeletable(key, path any) bool {
	return common.IsPath(key.(string)) && key.(string) == path.(string)
}

func (this *Path) CopyTo(v any) (any, uint32, uint32, uint32) { return v, 0, 1, 0 }

func (this *Path) IsNumeric() bool         { return false }
func (this *Path) IsCommutative() bool     { return true }
func (this *Path) IsIdempotent(v any) bool { return false } // To expensive to check, so we just return false.
func (this *Path) HasLimits() bool         { return true }

func (this *Path) Value() any { return this.DeltaSet.Committed() }
func (this *Path) Delta() (any, bool) {
	return this.DeltaSet.Delta(), (this.DeltaSet.SizeAdded() - this.DeltaSet.SizeRemoved()) > 0
}

func (this *Path) Limits() (any, any) { return nil, nil }

func (this *Path) CloneDelta() (any, bool) { return this.DeltaSet.CloneDelta(), true }

func (this *Path) IsDeltaApplied() bool   { return this.IsDirty() }
func (this *Path) SetValue(v any)         { this.DeltaSet = v.(*softdeltaset.DeltaSet[string]) }
func (this *Path) ResetDelta()            { this.DeltaSet.ResetDelta() }
func (this *Path) SetDelta(v any, _ bool) { this.DeltaSet.SetDelta(v.(*softdeltaset.DeltaSet[string])) }
func (this *Path) SetDeltaSign(v any)     {}

func (this *Path) Preload(k string, source any) {
	if this.preloaded != nil { // Already preloaded
		return
	}

	store := source.(interface {
		Retrieve(string, any) (any, error)
	})

	if v, err := store.Retrieve(k, new(Path)); v != nil && err == nil && v.(*Path).Committed().Length() > 0 {
		this.preloaded = v.(*Path).Committed()
	}
}

// Clone creates a new Path object with the same isCommitted and delta sets.
// the delta set is a deep copy of the original delta set, but the isCommitted set is a shallow copy.
// This is because the isCommitted set should never be modified until the commit time.
func (this *Path) Clone() any {
	return &Path{
		DeltaSet:     this.DeltaSet.Clone(),
		Expression:   this.Expression,
		preloaded:    this.preloaded,
		ElemType:     this.ElemType,
		isBlockBound: this.isBlockBound,
		TotalSize:    this.TotalSize,
	}
}

func (this *Path) Equal(other any) bool { return this.DeltaSet.Equal(other.(*Path).DeltaSet) }

func (this *Path) Get() (any, uint32, uint32) {
	return this.DeltaSet, 1, common.IfThen(this.DeltaSet.IsDirty(), uint32(1), uint32(0))
}

// For the codec only
func (this *Path) New(_, _, _, _, _ any) any {
	deltaSet := &Path{
		DeltaSet:     this.DeltaSet.CloneDelta(),
		ElemType:     this.ElemType,
		Expression:   this.Expression,
		isBlockBound: this.isBlockBound,
		TotalSize:    this.TotalSize,
	}
	return deltaSet
}

// Swap swaps two values.
func Swap[T any](lhv, rhv *T) {
	v := *lhv
	*lhv = *rhv
	*rhv = v
}

// ApplyDelta applies all the deltas from the non-conflicting transitions to the original value and returns the new value.
func (this *Path) ApplyDelta(typedVals []stgcommon.Type) (stgcommon.Type, int, error) {
	if idx, _ := slice.FindFirst(typedVals, nil); idx >= 0 {
		return nil, 1, nil //This is a deletion and when this is true, the number of write operations is 1.
	}

	// Due to the async nature of the importing process, the preloaded value may not be in the first element of the slice.
	// If this is the case, we need to find the preloaded value and set it to the preloaded field of the first element.
	if this.preloaded != nil {
		this.DeltaSet.SetCommitted(this.preloaded) // Set the preloaded value to the isCommitted value so the delta set can be applied on.
	} else {
		if idx, v := slice.FindFirstIf(typedVals, func(_ int, v stgcommon.Type) bool { return v.(*Path).preloaded != nil }); idx >= 0 {
			common.Swap(&this.preloaded, &(*v).(*Path).preloaded)
			this.DeltaSet.SetCommitted(this.preloaded)
		}
		// If no ones has the preloaded value, then this is a new path, no preloaded value
	}

	deltaSets := slice.Transform(typedVals, func(_ int, v stgcommon.Type) *softdeltaset.DeltaSet[string] { return v.(*Path).DeltaSet })
	this.Commit(deltaSets) // Apply the delta sets to the isCommitted value，including its own delta set.
	return this, len(typedVals), nil
}

// Set sets the value of the key to the given value and returns the new value, the number of keys added, the number of
// keys removed and the number of keys updated.
func (this *Path) Set(value any, source any) (any, uint32, uint32, uint32, error) {
	fullPath := source.([]any)[0].(string)
	containerRoot := source.([]any)[1].(string)
	tx := source.([]any)[2].(uint64)
	cache := source.([]any)[3].(interface {
		Write(uint64, string, any, ...any) (int64, error)
		IfExists(string) bool
		GetIfCached(string) (any, bool)
	})

	// Delete or rewrite the path. The path is the root of the container.
	// A rewrite is not allowed. But it can be deleted.
	// When that happens, all the sub paths are also deleted.
	clearPath, subPath := common.TrimWildcardSuffix(fullPath) // Remove the trailing wildcard suffix if it exists.

	// The clear path could be a sub path rather than a regular value.
	if common.IsPath(clearPath) && len(clearPath) == len(containerRoot) {
		// Either Delete the path and all its elements OR just the elements under the path with a wildcard suffix.
		if value != nil {
			return this, 0, 1, 0, errors.New("Error: Cannot rewrite a path!")
		}

		// Delete all elements under the path.
		if subPath == "*" {
			elems := this.DeltaSet.Elements() // Get all the elements under the path
			return this.deleteInPath(tx, clearPath, elems, func() { this.DeleteAll() }, cache)
		}

		// Delete the committed elements only
		if subPath == "[:]" {
			elems := this.DeltaSet.Committed().Elements() // Get all the elements under the path
			return this.deleteInPath(tx, clearPath, elems, func() { this.DeleteCommitted() }, cache)
		}
	}

	subkey := clearPath[len(clearPath)-(len(clearPath)-len(containerRoot)):] // Extract the sub key from the path
	if ok, _ := this.DeltaSet.Exists(subkey); (ok && value != nil) || (!ok && value == nil) {
		// Update an existing or delete a non-existent one won't change the set itself. So we return 0, 0, 0.
		// Otherwise it will cause a lot of conflicts, when multiple transactions are trying
		// to access the path. It is also logically correct.
		return this, 0, 0, 0, nil
	}

	if value == nil {
		// Delete an existing key
		this.DeltaSet.Delete(subkey)
	} else {
		// Insert a new key
		this.DeltaSet.Insert(subkey)
	}
	return this, 0, 0, 1, nil
}

func (this *Path) deleteInPath(tx uint64, parentPath string, elems []string, do func(), source any) (any, uint32, uint32, uint32, error) {
	writeCache := source.(interface {
		Write(uint64, string, any, ...any) (int64, error)
		IfExists(string) bool
		GetIfCached(string) (any, bool)
	})

	// Separate the sub paths from other elements.
	subPaths := slice.MoveIf(&elems, func(_ int, subpath string) bool {
		return common.IsPath(subpath)
	})

	// Only mark the elements already in the cache as deleted.
	// No need to touch those in the storage.
	for _, elem := range elems {
		if _, ok := writeCache.GetIfCached(parentPath + elem); ok {
			writeCache.Write(tx, parentPath+elem, nil)
		}
	}

	// Remove all from the path meta
	// this.DeleteAll()
	do()

	// Remove all the sub paths and their elements recursively.
	// This is NOT fully supported yet. For it to work, The write cache
	// needs to be able to handle recursive path meta checks.
	for _, subpath := range subPaths {
		writeCache.Write(tx, parentPath+subpath, nil)
	}

	// Delete isCommitted items only result in 1 delta write. Since no other thread may alter the path at the same time.
	if this.DeltaSet.CommittedOnly() {
		return this, 0, 0, 1, nil
	}

	// Need to delete the elements that are not isCommitted yet. This is a full delete.
	// For example, if Thread A adds some elements and Thread B deletes the path at the same time.
	// depending on the order of execution, the final result may be different. If A is executed first, then the elements
	// added by A will be deleted by B. If B is executed first, then the elements added by A will remain. They aren't commutative.
	return this, 0, 1, 0, nil
}

// data cleaning before saving to storage
func (this *Path) Reset() {
	// this.DeltaSet.Reset()
	this.DeltaSet.ResetDelta() // The isCommitted keys are not reset.
}

func (this *Path) Hash() [32]byte {
	return sha256.Sum256(this.Encode())
}

// This function is mainly for fast comparison.
// The second return value is to tell if the first return value is 100% accurate. If it is not, then the first return value
// is just an estimate. you need to use other function to do comparison
func (this *Path) ShortHash() (uint64, bool) {
	return 0, this.DeltaSet.IsEmpty()
}

// For Debug
func (this *Path) SetSubPaths(keys []string)   { this.DeltaSet.InsertCommitted(keys) }
func (this *Path) SetAdded(keys []string)      { this.DeltaSet.InsertAdded(keys) }
func (this *Path) InsertRemoved(keys []string) { this.DeltaSet.InsertRemoved(keys) }

func (this *Path) Keys() []string { // Committed keys
	return common.IfThenDo1st(this.DeltaSet.Committed() != nil, func() []string { return this.DeltaSet.Committed().Elements() }, []string{})
}

func (this *Path) Added() []string {
	return common.IfThenDo1st(this.DeltaSet.Added() != nil, func() []string { return this.DeltaSet.Added().Elements() }, []string{})
}

func (this *Path) Removed() []string {
	return common.IfThenDo1st(this.DeltaSet.Removed() != nil, func() []string { return this.DeltaSet.Removed().Elements() }, []string{})
}
