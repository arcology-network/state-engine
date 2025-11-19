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

package statecell

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/arcology-network/common-lib/common"
	"github.com/arcology-network/common-lib/exp/slice"
	intf "github.com/arcology-network/state-engine/common"
	"github.com/arcology-network/state-engine/type/noncommutative"
	"github.com/cespare/xxhash"
)

// THe univalue is a combination of a value and a property field that contains the access information about the value.
type StateCell struct {
	Property
	value any
	buf   []byte // The encoded value.
}

func NewStateCell(tx uint64, key string, reads, writes uint32, deltaWrites uint32, T any, source any) *StateCell {
	if source != nil {
		panic("Error: The source should be nil")
	}

	univ := &StateCell{
		Property{
			vType:         common.IfThenDo1st(T != nil, func() uint8 { return T.(intf.Type).TypeID() }, uint8(reflect.Invalid)),
			tx:            tx,
			path:          &key,
			keyHash:       xxhash.Sum64String(key),
			reads:         reads,
			writes:        writes,
			deltaWrites:   deltaWrites,
			sizeInStorage: 0,
			isCommitted:   common.IfThenDo1st(source != nil, func() bool { return (&Property{}).IsCommiitted(key, source) }, false),
		},
		T,
		[]byte{},
	}
	return univ
}

func (this *StateCell) Init(tx uint64, key string, reads, writes, deltaWrites uint32, v any, preExist bool) *StateCell {
	// Sometime when a value is read from the storage by a delete operation, the caller only provides the nil value as
	// the new value, the loader wouldn't be about to decode it to the correct type. The loader will return the raw byte value
	// in this case, so we convert it to a noncommutative bytes type to avoid the type mismatch happening down stream.
	if common.IsType[[]byte](v) {
		v = noncommutative.NewBytes(v.([]byte))
	}

	this.vType = common.IfThenDo1st(v != nil, func() uint8 { return v.(intf.Type).TypeID() }, uint8(reflect.Invalid))
	this.tx = tx
	this.path = &key
	this.keyHash = xxhash.Sum64String(key)
	this.reads = reads
	this.writes = writes
	this.deltaWrites = deltaWrites
	this.value = v
	this.isCommitted = preExist
	return this
}

func (*StateCell) New(meta, value, cache any) *StateCell {
	return &StateCell{
		*meta.(*Property),
		value,
		cache.([]byte),
	}
}

func (*StateCell) Reset(this *StateCell) {
	this.Property.Reset()
	this.ClearCache()
	this.value = nil
}

func (this *StateCell) From(v *StateCell) any { return v }

// func (this *StateCell) IsHotLoaded() bool             { return this.reads > 1 }
func (this *StateCell) SetTx(txId uint64) { this.tx = txId }
func (this *StateCell) ClearCache()       { this.buf = this.buf[:0] }
func (this *StateCell) Value() any        { return this.value }
func (this *StateCell) SetValue(newValue any) *StateCell {
	if this.value != nil && reflect.TypeOf(this.value) != reflect.TypeOf(newValue) && newValue != nil {
		panic("Wrong type")
	}

	this.value = newValue
	return this
}

func (this *StateCell) Reclaim() {
	if this.reclaimFunc != nil {
		this.reclaimFunc(this)
	}
}

// This performs the action on the value and returns the result.
// This function doesnn't make a deep copy of the original value.
// It should be used for read-only operations ONLY!!!.
func (this *StateCell) Do(tx uint64, path string, doer any) any {
	r, w, dw, ret := doer.(func(any) (uint32, uint32, uint32, any))(this)
	this.reads += r
	this.writes += w
	this.deltaWrites += dw
	return ret
}

func (this *StateCell) Get(tx uint64, path string, source any) any {
	if this.value != nil {
		tempV, r, w := this.value.(intf.Type).Get() //RW: Affiliated reads and writes
		this.reads += r

		// The whole copy mechansim is designed to avoid interference with maximum performance,
		// so deep copy is made only when necessary. The criteria for making a deep copy are:
		// 1. The value needs to be updated.
		// 2. The value is modified for the first time, when writes == 0 && deltaWrites == 0.
		// So in the following cases, if we record a write with a deep copy, it will effectivly stop making deep copy in the future.
		// This is problematic because it will change the value in the global object cache as well.
		if w > 0 {
			this.MakeDeepCopy(this.value)
		}

		this.writes += w
		return tempV
	}
	this.IncrementReads(1)
	return this.value
}

func (this *StateCell) CopyTo(writable any) {
	writeCache := writable.(interface {
		Read(uint64, string, any) (any, any, uint64)
		Write(uint64, string, any, ...any) (int64, error)
		FindForRead(uint64, string, any, func(*StateCell)) (any, *StateCell, bool)
	})

	if this.writes == 0 && this.deltaWrites == 0 {
		writeCache.Read(this.tx, *this.GetPath(), this.value)
	} else {
		writeCache.Write(this.tx, *this.GetPath(), this.value)
	}

	_, univ, _ := writeCache.FindForRead(this.tx, *this.GetPath(), nil, nil)
	if this == univ {
		return
	}

	univ.IncrementReads(this.Reads())
	univ.IncrementWrites(this.Writes())
	univ.IncrementDeltaWrites(this.DeltaWrites())
}

func (this *StateCell) Set(tx uint64, path string, newV any, inCache bool, importer any) error { // update the value
	this.tx = tx

	// Delete an non-existing value or deleting an entry that has been deleted already.
	if this.value == nil && newV == nil {
		this.writes++
		return errors.New("Error: The value doesn't exists")
	}

	// Write a new value
	if this.value == nil {
		this.vType = newV.(intf.Type).TypeID()
		v, r, w, dw := newV.(intf.Type).CopyTo(newV)
		this.value = v
		this.writes += w
		this.reads += r
		this.deltaWrites += dw
		return nil
	}

	this.MakeDeepCopy(newV) // To avoid interference with the value in the global object cache.

	//Update an existing value. For a path a delete operation will
	// remove all its sub paths, but not the path itself.
	oldV := this.value.(intf.Type)
	v, r, w, dw, err := oldV.Set(newV, []any{path, *this.path, tx, importer}) // Update the current value
	this.value = v
	this.writes += w
	this.reads += r
	this.deltaWrites += dw

	//Delete an existing value. Not Every value can be deleted by itself, some only can be delete by its parent
	// that is why we need to check first.
	if newV == nil && this.Value().(intf.Type).IsDeletable(*this.GetPath(), path) { // Delete the entry but keep the access record.
		this.vType = uint8(reflect.Invalid)
		this.value = newV // Delete the value
		this.writes++
		this.isDeleted = true // This is a delete operation
	}
	return err
}

// Making a deep copy may be necessary to avoid interference with
// the value in the global object cache.
func (this *StateCell) MakeDeepCopy(newV any) {
	// writes == 0 && deltaWrites == 0 means the value has been modified already.
	// this.value == nil, this is a new value assignment, so we don't need to make a deep copy.
	// typedV == nil, this is a delete operation, so we don't need to make a deep copy.
	// In cascading write cache, the values' access info will stripped off, so it wouldn't introduce interference.
	if this.writes == 0 && this.deltaWrites == 0 && this.value != nil { // Make a deep copy if has't done so
		this.value = this.value.(intf.Type).Clone()
	}
}

// Check & Merge attributes
func (this *StateCell) ApplyDelta(vec []*StateCell) error {

	/* Precheck & Merge attributes*/
	for i := range vec {
		this.PrecheckAttributes(vec[i])
		this.writes += vec[i].Writes()
		this.reads += vec[i].Reads()
		this.deltaWrites += vec[i].DeltaWrites()
	}

	// Apply transitions
	typedVals := slice.Transform(vec, func(_ int, v *StateCell) intf.Type {
		if v.Value() != nil {
			return v.Value().(intf.Type)
		}
		return nil
	})

	var err error
	if this.Value() != nil {
		if this.value, _, err = this.Value().(intf.Type).ApplyDelta(typedVals); err != nil {
			return err
		}
	}
	return nil
}

// when a key is looked up, it will be checked if its parent path exists and it will generate a path access record.
// This record will have zero reads, writes and deltaWrites because isn't a real access to the path itself.
// It may be a read access to the sub key, but it has been already recorded in the sub key record. In addition, it doesn't
// conflict with other path access like read/write/deltaWrite to the path.
func (this *StateCell) PathLookupOnly() bool {
	return this.reads == 0 && this.deltaWrites == 0 && this.writes == 0
}

// If all the entries in the isCommitted set have been removed.
// only work for Path type
// func (this *StateCell) IsCommittedDeleted() (bool, string) {
// 	if common.IsType[*commutative.Path](this.Value()) {
// 		return this.Value().(*commutative.Path).DeltaSet.Removed().AllDeleted, *this.path
// 	}

//		return IsCommittedPath(*this.path)
//	}
func (this *StateCell) IsReadOnly() bool       { return (this.writes == 0 && this.deltaWrites == 0) }
func (this *StateCell) IsWriteOnly() bool      { return (this.reads == 0 && this.deltaWrites == 0) }
func (this *StateCell) IsDeltaWriteOnly() bool { return (this.reads == 0 && this.writes == 0) }
func (this *StateCell) IsDeleteOnly() bool {
	return this.isDeleted && this.reads == 0 && this.deltaWrites == 0 // Cannot just use value == nil, because it may be a new value.
}

func (this *StateCell) IsNilInitOnly() bool {
	return this.reads == 0 &&
		!this.isDeleted &&
		this.Value() != nil &&
		this.Value().(intf.Type).IsCommutative() // Must be commutative
}

// Commutative write is no longer treated as a conflict with read.
// Write without read happens when a new value is created.
func (this *StateCell) IsCumulativeWriteOnly(other *StateCell) bool {
	if this.Value() == nil {
		return false
	}

	min, max := this.Value().(intf.Type).Limits()
	otherMin, otherMax := other.Value().(intf.Type).Limits()
	return this.reads == 0 &&
		this.Value().(intf.Type).IsCommutative() &&
		this.Value().(intf.Type).IsNumeric() &&
		min == otherMin &&
		max == otherMax &&
		this.Reads() == 0
}

func (this *StateCell) PrecheckAttributes(other *StateCell) {
	if other.reads == 0 && other.writes == 0 && other.deltaWrites == 0 {
		panic("Error: Read/Write/Deltawrite all zero!!")
	}

	if other.writes == 0 && other.deltaWrites == 0 {
		panic("Error: Value type mismatched!") // Read only variable should never be here.
	}

	if this.GetTx() != other.GetTx() &&
		this.isCommitted &&
		this.Value() != nil &&
		this.Value().(intf.Type).IsCommutative() &&
		this.Reads() > 0 &&
		this.IsDeltaWriteOnly() == other.IsDeltaWriteOnly() {
		this.Print()
		fmt.Println("================================================================")
		other.Print()
		panic("Error: The composite attribute must match in different transitions")
	}

	if this.Value() == nil && this.IsDeltaWriteOnly() {
		panic("Error: A deleted value cann't be composite")
	}

	if !this.isCommitted && this.IsDeltaWriteOnly() {
		panic("Error: A new value cann't be composite")
	}
}

func (this *StateCell) Clone() any {
	v := &StateCell{
		this.Property.Clone(),
		common.IfThenDo1st(this.value != nil, func() any { return this.value.(intf.Type).Clone() }, this.value),
		slice.Clone(this.buf),
	}
	return v
}

func LessByTx(this, other *StateCell) bool { return this.tx < other.tx }

func (this *StateCell) Less(other *StateCell) bool {
	if (this.value == nil || other.value == nil) && (this.value != other.value) {
		return this.value == nil
	}

	if this.writes != other.writes {
		return this.writes > other.writes
	}

	if this.reads != other.reads {
		return this.reads > other.reads
	}

	if this.deltaWrites != other.deltaWrites {
		return this.deltaWrites > other.deltaWrites
	}

	if (!this.isCommitted || !other.isCommitted) && (this.isCommitted != other.isCommitted) {
		return this.isCommitted
	}
	return true
}

func (this *StateCell) Checksum() [32]byte {
	return sha256.Sum256(this.Encode())
}

func (this *StateCell) Print() {
	spaces := " " //fmt.Sprintf("%"+strconv.Itoa(len(strings.Split(*this.path, "/"))*1)+"v", " ")
	fmt.Print(spaces+"tx: ", this.tx)
	fmt.Print(spaces+"sequence: ", this.sequence)
	fmt.Print(spaces+"reads: ", this.reads)
	fmt.Print(spaces+"writes: ", this.writes)
	fmt.Print(spaces+"DeltaWrites: ", this.deltaWrites)
	fmt.Print(spaces+"ifSkipConflictCheck: ", this.ifSkipConflictCheck)
	fmt.Print(spaces+"isBlockBound: ", this.isBlockBound)
	fmt.Print(spaces+"isCommitted: ", this.isCommitted)

	path := *this.path
	if index := strings.Index(path, "container/"); index != -1 {
		path = path[:index] + "container/" + hex.EncodeToString([]byte(path[index:]))
	}

	fmt.Print(spaces+"path: ", *this.path, "      ")
	common.IfThenDo(this.value != nil, func() { this.value.(intf.Type).Print() }, func() { fmt.Print("nil") })
	fmt.Println()
}

func (this *StateCell) Equal(other *StateCell) bool {
	if this.value == nil && other.Value() == nil {
		return true
	}

	if (this.value == nil && other.Value() != nil) || (this.value != nil && other.Value() == nil) {
		return false
	}

	vFlag := this.value.(intf.Type).Equal(other.Value().(intf.Type))
	return this.tx == other.GetTx() &&
		*this.path == *other.GetPath() &&
		this.reads == other.Reads() &&
		this.writes == other.Writes() &&
		vFlag &&
		this.isCommitted == other.IsCommitted()
}

// func (this *StateCell) GetCascadeSub(prefix string, source any) []string {
// 	elements := slice.Transform(this.DeltaSet.Elements(), func(_ int, k string) string { return prefix + k })

// 	store := source.(interface {
// 		Peek(string, any) (any, error)
// 	})

// 	pathStrs := []string{}
// 	for {
// 		subPaths := slice.MoveIf(&elements, func(_ int, k string) bool {
// 			return common.IsPath(k)
// 		})
// 		pathStrs = append(pathStrs, elements...)

// 		// Loop through the sub paths and check if they exist in the store.
// 		for i := range subPaths {
// 			if path, _ := store.Peek(prefix+subPaths[i], new(Path)); path != nil {
// 				path.(*Path).GetCascadeSub(prefix+subPaths[i], store)
// 			}
// 		}

// 		if len(subPaths) == 0 {
// 			pathStrs = append(pathStrs, subPaths...)
// 			break
// 		}
// 	}
// 	return pathStrs
// }
