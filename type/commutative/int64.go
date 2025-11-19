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
	"math"

	"github.com/arcology-network/common-lib/common"
	stgcommon "github.com/arcology-network/state-engine/common"
)

type Int64 struct {
	value int64
	delta int64
	min   int64
	max   int64
}

func NewInt64(min, max int64) any {
	if min > max {
		return nil
	}

	return &Int64{
		0,
		0,
		min,
		max,
	}
}

func NewInt64Delta(delta int64) any {
	return &Int64{delta: delta}
}

func (this *Int64) New(value, delta, sign, min, max any) any {
	return &Int64{
		common.IfThenDo1st(value != nil, func() int64 { return value.(int64) }, 0),
		common.IfThenDo1st(delta != nil, func() int64 { return delta.(int64) }, 0),
		common.IfThenDo1st(min != nil, func() int64 { return min.(int64) }, math.MinInt64),
		common.IfThenDo1st(max != nil, func() int64 { return max.(int64) }, math.MaxInt64),
	}
}

func (this *Int64) Clone() any {
	return &Int64{
		value: this.value,
		delta: this.delta,
		min:   this.min,
		max:   this.max,
	}
}

func (this *Int64) Equal(other any) bool { return *this == *other.(*Int64) }
func (this *Int64) IsNumeric() bool      { return true }
func (this *Int64) IsCommutative() bool  { return true }
func (this *Int64) HasLimits() bool      { return this.min != math.MinInt64 || this.max != math.MaxInt64 }

func (this *Int64) Value() any         { return this.value }
func (this *Int64) Delta() (any, bool) { return this.delta, this.delta >= 0 }
func (this *Int64) DeltaSign() bool    { return this.delta >= 0 }
func (this *Int64) Limits() (any, any) { return this.min, this.max }

func (this *Int64) CloneDelta() (any, bool) { return (this.delta), this.delta >= 0 }
func (this *Int64) SetValue(v any)          { this.value = v.(int64) }
func (this *Int64) Preload(_ string, _ any) {}

func (this *Int64) IsDeltaApplied() bool   { return this.delta == 0 }
func (this *Int64) ResetDelta()            { this.delta = 0 }
func (this *Int64) SetDelta(v any, _ bool) { this.delta = (v.(int64)) }
func (this *Int64) SetDeltaSign(v any)     {}

func (this *Int64) MemSize() uint64                            { return 5 * 8 }
func (this *Int64) TypeID() uint8                              { return INT64 }
func (this *Int64) IsDeletable(key, path any) bool             { return true }
func (this *Int64) CopyTo(v any) (any, uint32, uint32, uint32) { return v, 0, 1, 0 }
func (this *Int64) Reset() {
	this.value = 0
	this.delta = 0
}

func (*Int64) GetCascadeSub(_ string, _ any) []string { return nil } // The entries to delete when this is deleted.

func (this *Int64) Get() (any, uint32, uint32) {
	return int64(this.value + this.delta), 1, common.IfThen(this.delta == 0, uint32(0), uint32(1))
}

func (this *Int64) Set(v any, source any) (any, uint32, uint32, uint32, error) {
	if v == nil {
		return this, 0, 1, 0, nil
	}

	if this.isUnderflow(int64(v.(*Int64).delta)) || this.isOverflow(int64(v.(*Int64).delta)) {
		return this, 0, 1, 0, errors.New("Error: Value out of range!!")
	}

	if (this.min > this.value+this.delta+v.(*Int64).delta) ||
		(this.max < this.value+this.delta+v.(*Int64).delta) {
		return this, 0, 1, 0, errors.New("Error: Value out of range!!")
	}

	this.delta += v.(*Int64).delta
	return this, 0, 1, 0, nil
}

func (this *Int64) isOverflow(delta int64) bool {
	flag := this.max-delta < this.value+this.delta
	return (delta >= 0 && this.delta >= 0) &&
		(this.max < delta || flag)
}

func (this *Int64) isUnderflow(delta int64) bool {
	flag := this.min-delta > this.value+this.delta
	return (delta < 0 && this.delta < 0) &&
		(this.min > delta || flag)
}

func (this *Int64) ApplyDelta(typedVals []stgcommon.Type) (stgcommon.Type, int, error) {
	for i, v := range typedVals {
		if this == nil && v != nil { // New value
			this = v.(*Int64)
		}

		if this != nil && v != nil {
			if _, _, _, _, err := this.Set(v, nil); err != nil {
				return nil, i, err
			}
		}

		if this != nil && v == nil {
			this = nil
		}
	}

	if this == nil {
		return nil, 0, errors.New("Error: A commutative int64 can't be nil")
	}

	this.value += this.delta
	this.delta = 0
	return this, len(typedVals), nil
}

func (this *Int64) ShortHash() (uint64, bool) {
	return uint64(this.value) ^ (1 << 63), true
}

func (this *Int64) Hash() [32]byte {
	return sha256.Sum256(this.Encode())
}
