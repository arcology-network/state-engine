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

package noncommutative

import (
	"math"

	"github.com/arcology-network/common-lib/common"
	intf "github.com/arcology-network/state-engine/common"
)

type Uint64 uint64

func NewUint64(v uint64) *Uint64 {
	var this Uint64 = Uint64(v)
	return &this
}

func (this *Uint64) MemSize() uint64                            { return 8 }
func (this *Uint64) IsDeletable(key, path any) bool             { return true }
func (this *Uint64) TypeID() uint8                              { return UINT64 }
func (this *Uint64) CopyTo(v any) (any, uint32, uint32, uint32) { return v, 0, 1, 0 }
func (this *Uint64) Clone() any                                 { return common.New(*this) }
func (this *Uint64) Equal(other any) bool                       { return *this == *(other.(*Uint64)) }
func (this *Uint64) Get() (any, uint32, uint32)                 { return uint64(*this), 1, 0 }

func (this *Uint64) IsNumeric() bool     { return true }
func (this *Uint64) IsCommutative() bool { return false }
func (this *Uint64) HasLimits() bool     { return false }

func (this *Uint64) Value() any         { return this }
func (this *Uint64) Delta() (any, bool) { return this, true }
func (this *Uint64) DeltaSign() bool    { return true } // delta sign
func (this *Uint64) Limits() (any, any) {
	min, max := 0, uint64(math.MaxUint64)
	return &min, &max
}

func (this *Uint64) CloneDelta() (any, bool) { return this.Clone(), *this >= 0 }
func (this *Uint64) SetValue(v any)          { this.SetDelta(v, true) } // The sign is only a placeholder, the value carries the sign by itself.
func (this *Uint64) Preload(_ string, _ any) {}

func (this *Uint64) IsDeltaApplied() bool              { return true }
func (this *Uint64) ResetDelta()                       { this.SetDelta(common.New[Uint64](0), true) }
func (this *Uint64) SetDelta(v any, _ bool)            { (*this) = (*v.(*Uint64)) }
func (*Uint64) GetCascadeSub(_ string, _ any) []string { return nil } // The entries to delete when this is deleted.

func (this *Uint64) New(_, delta, _, _, _ any) any {
	return common.IfThenDo1st(delta != nil && delta.(*Uint64) != nil, func() any { return delta.(*Uint64).Clone() }, any(this))
}

func (this *Uint64) Set(value any, source any) (any, uint32, uint32, uint32, error) {
	if value != nil {
		*this = Uint64(*(value.(*Uint64)))
	}
	return this, 0, 1, 0, nil
}

func (this *Uint64) ApplyDelta(typedVals []intf.Type) (intf.Type, int, error) {
	for _, v := range typedVals {
		if this == nil && v != nil { // New value
			this = v.(*Uint64)
		}

		if this != nil && v != nil {
			this.Set(v.(*Uint64), nil)
		}

		if this != nil && v == nil {
			this = nil
		}
	}

	if this == nil {
		return nil, 0, nil
	}
	return this, len(typedVals), nil
}
