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
	"fmt"
	"math/big"

	codec "github.com/arcology-network/common-lib/codec"
	"github.com/arcology-network/common-lib/common"
	"github.com/arcology-network/common-lib/exp/slice"
	stgcommon "github.com/arcology-network/state-engine/common"
	uint256 "github.com/holiman/uint256"
)

var (
	U256_ZERO = (*uint256.NewInt(0))
	U256_ONE  = (codec.Uint256)(*uint256.NewInt(1))

	U256_MIN = (*uint256.NewInt(0)) // default limits
	U256_MAX = (*uint256.NewInt(0).SetAllOne())
)

type U256 struct {
	value         uint256.Int
	delta         uint256.Int
	min           uint256.Int
	max           uint256.Int
	deltaPositive bool
}

func NewBoundedU256(lower, upper *uint256.Int) stgcommon.Type {
	v := NewUnboundedU256().(*U256)
	if upper.Cmp(lower) >= 0 { // The upper limit has to be greater than the lower one
		v.min = *lower
		v.max = *upper
	}
	return v
}

func NewBoundedU256FromU64(lower, upper uint64) stgcommon.Type {
	v := NewUnboundedU256().(*U256)
	if upper >= lower { // The upper limit has to be greater than the lower one
		v.min = *uint256.NewInt(lower)
		v.max = *uint256.NewInt(upper)
	}
	return v
}

func NewUnboundedU256() stgcommon.Type {
	return &U256{
		value:         *uint256.NewInt(0),
		delta:         *uint256.NewInt(0),
		deltaPositive: true,
		min:           U256_MIN,
		max:           U256_MAX,
	}
}

func NewU256Delta(delta *uint256.Int, deltaPositive bool) stgcommon.Type {
	return &U256{
		delta:         (*delta),
		deltaPositive: deltaPositive,
	}
}

func NewBoundedU256Delta(min, max *uint256.Int, delta *uint256.Int, deltaPositive bool) stgcommon.Type {
	v := &U256{
		delta:         (*delta),
		deltaPositive: deltaPositive,
	}

	v.min = *min
	v.max = *max
	return v
}

func NewU256DeltaFromU64(delta uint64, deltaPositive bool) stgcommon.Type {
	return &U256{
		delta:         *uint256.NewInt(delta),
		deltaPositive: deltaPositive,
	}
}

func NewU256DeltaFromBigInt(delta *big.Int) (any, bool) {
	sign := delta.Sign()
	deltaV, overflowed := uint256.FromBig(delta.Abs(delta))
	if overflowed {
		return nil, false
	}

	return &U256{
		delta:         *deltaV,
		deltaPositive: sign != -1, // >= 0
	}, true
}

// A commutative type shouldn't have the initial value other than the type default value.
// This function is mainly for debugging purpose.
func (*U256) NewBoundedU256(value, delta, min, max *uint256.Int, sign bool) *U256 {
	return &U256{
		value:         *value,
		delta:         *delta,
		min:           *min,
		max:           *max,
		deltaPositive: sign, // positive delta by default
	}
}

func (*U256) NewBoundedU256FromUint64(value, delta, min, max int64, sign bool) *U256 {
	u256Value := &U256{}
	u256Value.value.SetFromBig(big.NewInt(value))
	u256Value.delta.SetFromBig(big.NewInt(delta))
	u256Value.min.SetFromBig(big.NewInt(min))
	u256Value.max.SetFromBig(big.NewInt(max))
	u256Value.deltaPositive = sign
	return u256Value
}

func (this *U256) New(value, delta, sign, min, max any) any {
	return &U256{
		value:         common.IfThenDo1st(value != nil, func() uint256.Int { return value.(uint256.Int) }, *U256_ZERO.Clone()),
		delta:         common.IfThenDo1st(delta != nil, func() uint256.Int { return delta.(uint256.Int) }, *U256_ZERO.Clone()),
		deltaPositive: common.IfThenDo1st(sign != nil, func() bool { return sign.(bool) }, true),
		min:           common.IfThenDo1st(min != nil, func() uint256.Int { return min.(uint256.Int) }, U256_ZERO),
		max:           common.IfThenDo1st(max != nil, func() uint256.Int { return max.(uint256.Int) }, U256_MAX),
	}
}

func (this *U256) IsNumeric() bool     { return true }
func (this *U256) IsCommutative() bool { return true }
func (this *U256) HasLimits() bool     { return !this.min.Eq(&U256_ZERO) || !this.max.Eq(&U256_MAX) }

func (this *U256) Value() any         { return this.value }
func (this *U256) Delta() (any, bool) { return this.delta, this.deltaPositive }

func (this *U256) Limits() (any, any)     { return this.min, this.max }
func (this *U256) SetLimits(min, max any) { this.min, this.max = min.(uint256.Int), max.(uint256.Int) } // Debugging functions

func (this *U256) CloneDelta() (any, bool) { return *this.delta.Clone(), this.deltaPositive }
func (this *U256) ToAbsolute() any         { return this.value }
func (this *U256) SetValue(v any)          { this.value = (v.(uint256.Int)) }
func (this *U256) Preload(_ string, _ any) {}

func (this *U256) IsDeltaApplied() bool { return this.delta.Eq(&U256_ZERO) }
func (this *U256) ResetDelta()          { this.SetDelta(*U256_ZERO.Clone(), true) }

func (this *U256) SetDelta(v any, sign bool) {
	this.delta = (v.(uint256.Int))
	this.deltaPositive = sign
}

func (this *U256) MemSize() uint64                            { return 16 + 1 } // in bytes
func (this *U256) IsDeletable(key, path any) bool             { return true }
func (this *U256) TypeID() uint8                              { return UINT256 }
func (this *U256) CopyTo(v any) (any, uint32, uint32, uint32) { return v, 0, 1, 0 }
func (*U256) GetCascadeSub(_ string, _ any) []string          { return nil }

func (this *U256) Reset() { // Reset to its default value
	slice.Fill(this.delta[:], 0)
	slice.Fill(this.value[:], 0)
}

func (this *U256) Hash() [32]byte            { return sha256.Sum256(this.Encode()) }
func (this *U256) ShortHash() (uint64, bool) { return 0, false }

func (this *U256) Clone() any {
	return &U256{
		value:         *this.value.Clone(),
		delta:         *this.delta.Clone(),
		min:           *this.min.Clone(),
		max:           *this.max.Clone(),
		deltaPositive: this.deltaPositive,
	}
}

func (this *U256) Equal(other any) bool {
	return this.value.Eq(&other.(*U256).value) &&
		this.delta.Eq(&other.(*U256).delta) &&
		this.deltaPositive == other.(*U256).deltaPositive &&
		this.min.Eq(&other.(*U256).min) &&
		this.max.Eq(&other.(*U256).max)
}

func (this *U256) Get() (any, uint32, uint32) {
	if U256_ZERO.Eq(&this.delta) {
		return *((*uint256.Int)(&this.value)), 1, 0 // delta is zero
	}

	original := this.value.Clone()
	if this.deltaPositive {
		return *((&uint256.Int{}).Add(original, &this.delta)), 1, 1
	}
	return *((&uint256.Int{}).Sub(original, &this.delta)), 1, 1
}

func (this *U256) isOverflowed(lhv *uint256.Int, lhvSign bool, rhv *uint256.Int, rhvSign bool) (*uint256.Int, bool) {
	if lhvSign == rhvSign { // Both positive or negative
		summed, overflowed := (*uint256.Int)(lhv).AddOverflow(lhv, (*uint256.Int)(rhv))
		if overflowed {
			return nil, true
		}
		return summed, lhvSign
	}

	if lhv.Cmp(rhv) < 1 { // v0 <= rhv
		return uint256.NewInt(0).Sub(rhv, lhv),
			common.IfThen((*uint256.Int)(rhv).Eq(lhv), true, rhvSign) // sign is positive when delta values cancel out each other

	}
	return uint256.NewInt(0).Sub(lhv, rhv), lhvSign
}

// Set delta
func (this *U256) Set(newDelta any, source any) (any, uint32, uint32, uint32, error) {
	if newDelta == nil {
		return this, 0, 1, 0, nil
	}

	if newDelta.(*U256).delta.Eq(&U256_ZERO) {
		return this, 0, 0, 0, nil
	}

	accumDelta, isDeltaPositive := this.isOverflowed(this.delta.Clone(), this.deltaPositive, &newDelta.(*U256).delta, newDelta.(*U256).deltaPositive)
	if accumDelta == nil {
		return this, 0, 0, 1, errors.New("Error: The value is underflowed")
	}

	accumVal, isPossitive := this.isOverflowed(this.value.Clone(), true, accumDelta.Clone(), isDeltaPositive)
	if accumVal == nil || !isPossitive { // Result must be possitive
		return this, 0, 0, 1, errors.New("Error: The value is overflowed")
	}

	if this.min.Cmp(accumVal) < 1 && accumVal.Cmp(&this.max) < 1 {
		this.delta = *accumDelta
		this.deltaPositive = isDeltaPositive
		return this, 0, 0, 1, nil
	}
	return this, 0, 0, 1, errors.New("Error: Value out of range")
}

func (this *U256) ApplyDelta(typedVals []stgcommon.Type) (stgcommon.Type, int, error) {
	for i, v := range typedVals {

		if this == nil && v != nil { // New value
			this = v.(*U256)
		}

		if this == nil && v == nil { // Delete a non-existent
			this = nil
		}

		if this != nil && v != nil { // Update an existent
			if _, _, _, _, err := this.Set(v.(*U256), nil); err != nil {
				return nil, i, err
			}
		}

		if this != nil && v == nil { // Delete an existent
			this = nil
		}
	}

	newValue, _, _ := this.Get()
	this.value = (newValue.(uint256.Int))
	this.delta.Clear()
	return this, len(typedVals), nil
}

func (this *U256) Print() {
	fmt.Println(" Value: ", this.value, " Delta: ", this.delta, "Delta Sign: ", this.deltaPositive)
}
