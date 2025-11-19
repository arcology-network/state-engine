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

	codec "github.com/arcology-network/common-lib/codec"
	common "github.com/arcology-network/common-lib/common"
	stgcommon "github.com/arcology-network/state-engine/common"
)

// type Selector []bool

type Uint64 struct {
	value uint64
	delta uint64
	min   uint64
	max   uint64
}

func NewUnboundedUint64() stgcommon.Type         { return &Uint64{min: 0, max: math.MaxUint64} }
func NewUint64Delta(delta uint64) stgcommon.Type { return &Uint64{delta: delta} }

func NewBoundedUint64(min, max uint64) stgcommon.Type {
	if max >= min {
		return &Uint64{min: min, max: max}
	}
	return NewUnboundedUint64()
}

func (this *Uint64) Clone() any { return common.New(*this) }

// For the codec only, don't use it for other purposes
func (this *Uint64) New(value, delta, _, min, max any) any {
	return &Uint64{
		common.IfThenDo1st(value != nil, func() uint64 { return value.(uint64) }, 0),
		common.IfThenDo1st(delta != nil, func() uint64 { return delta.(uint64) }, 0),
		common.IfThenDo1st(min != nil, func() uint64 { return min.(uint64) }, 0),
		common.IfThenDo1st(max != nil, func() uint64 { return max.(uint64) }, math.MaxUint64),
	}
}

func (this *Uint64) Equal(other any) bool {
	return this.value == other.(*Uint64).value &&
		this.delta == other.(*Uint64).delta &&
		this.min == other.(*Uint64).min &&
		this.max == other.(*Uint64).max
}

func (this *Uint64) MemSize() uint64 { return 5 * 8 }

func (this *Uint64) IsNumeric() bool     { return true }
func (this *Uint64) IsCommutative() bool { return true }
func (this *Uint64) HasLimits() bool     { return this.min != 0 || this.max != math.MaxInt64 }

func (this *Uint64) Value() any         { return this.value }
func (this *Uint64) Delta() (any, bool) { return this.delta, true }
func (this *Uint64) DeltaSign() bool    { return true }
func (this *Uint64) Limits() (any, any) { return this.min, this.max }

func (this *Uint64) Reset()                  { this.delta = 0 }
func (this *Uint64) IsDeltaApplied() bool    { return this.delta == 0 }
func (this *Uint64) CloneDelta() (any, bool) { return this.delta, true }
func (this *Uint64) ResetDelta()             { this.SetDelta(common.New[codec.Uint64](0), true) }
func (this *Uint64) Preload(_ string, _ any) {}

func (this *Uint64) SetValue(v any)            { this.value = v.(uint64) }
func (this *Uint64) SetDelta(v any, sign bool) { this.delta = v.(uint64) }

func (this *Uint64) TypeID() uint8                              { return UINT64 }
func (this *Uint64) IsDeletable(key, path any) bool             { return true }
func (this *Uint64) CopyTo(v any) (any, uint32, uint32, uint32) { return v, 0, 1, 0 }
func (*Uint64) GetCascadeSub(_ string, _ any) []string          { return nil }

func (this *Uint64) Get() (any, uint32, uint32) {
	return this.value + this.delta, 1, common.IfThen(this.delta == 0, uint32(0), uint32(1))
}

func (this *Uint64) Set(v any, source any) (any, uint32, uint32, uint32, error) {
	if v == nil {
		return this, 0, 1, 0, nil
	}

	if (this.max < v.(*Uint64).delta) || (this.max-v.(*Uint64).delta < this.value+this.delta) {
		return this, 0, 1, 0, errors.New("Error: Value out of range!!")
	}

	this.delta += v.(*Uint64).delta
	return this, 0, 0, 1, nil
}

func (this *Uint64) ApplyDelta(typedVals []stgcommon.Type) (stgcommon.Type, int, error) {
	for i, v := range typedVals {

		if this == nil && v != nil { // New value
			this = v.(*Uint64)
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
		return nil, 0, errors.New("Error: Nil value")
	}

	this.value += this.delta
	this.delta = 0
	return this, len(typedVals), nil
}

func (this *Uint64) Hash() [32]byte            { return sha256.Sum256(this.Encode()) }
func (this *Uint64) ShortHash() (uint64, bool) { return this.value, false }
