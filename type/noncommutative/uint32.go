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
	intf "github.com/arcology-network/storage-committer/common"
)

type Uint32 uint32

func NewUint32(v uint32) *Uint32 {
	var this Uint32 = Uint32(v)
	return &this
}

func (this *Uint32) MemSize() uint64                            { return 8 }
func (this *Uint32) IsDeletable(key, path any) bool             { return true }
func (this *Uint32) TypeID() uint8                              { return UINT32 }
func (this *Uint32) CopyTo(v any) (any, uint32, uint32, uint32) { return v, 0, 1, 0 }
func (this *Uint32) Clone() any                                 { return common.New(*this) }
func (this *Uint32) Equal(other any) bool                       { return *this == *(other.(*Uint32)) }
func (this *Uint32) Get() (any, uint32, uint32)                 { return int64(*this), 1, 0 }

func (this *Uint32) IsNumeric() bool     { return true }
func (this *Uint32) IsCommutative() bool { return false }
func (this *Uint32) HasLimits() bool     { return false }

func (this *Uint32) Value() any         { return this }
func (this *Uint32) Delta() (any, bool) { return this, true }
func (this *Uint32) DeltaSign() bool    { return true } // delta sign
func (this *Uint32) Limits() (any, any) {
	min, max := 0, uint64(math.MaxUint32)
	return &min, &max
}

func (this *Uint32) CloneDelta() (any, bool) { return this.Clone(), true }
func (this *Uint32) SetValue(v any)          { this.SetDelta(v, true) } // The sign is only a placeholder, the value carries the sign by itself.
func (this *Uint32) Preload(_ string, _ any) {}

func (this *Uint32) IsDeltaApplied() bool              { return true }
func (this *Uint32) ResetDelta()                       { this.SetDelta(common.New[Uint32](0), true) }
func (this *Uint32) SetDelta(v any, _ bool)            { (*this) = (*v.(*Uint32)) }
func (*Uint32) GetCascadeSub(_ string, _ any) []string { return nil } // The entries to delete when this is deleted.

func (this *Uint32) New(_, delta, _, _, _ any) any {
	return common.IfThenDo1st(delta != nil && delta.(*Uint32) != nil, func() any { return delta.(*Uint32).Clone() }, any(this))
}

func (this *Uint32) Set(value any, source any) (any, uint32, uint32, uint32, error) {
	if value != nil {
		*this = Uint32(*(value.(*Uint32)))
	}
	return this, 0, 1, 0, nil
}

func (this *Uint32) ApplyDelta(typedVals []intf.Type) (intf.Type, int, error) {
	for _, v := range typedVals {
		if this == nil && v != nil { // New value
			this = v.(*Uint32)
		}

		if this != nil && v != nil {
			this.Set(v.(*Uint32), nil)
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
