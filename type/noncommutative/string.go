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
	"strings"

	"github.com/arcology-network/common-lib/common"
	intf "github.com/arcology-network/state-engine/common"
)

type String string

func NewString(v string) intf.Type {
	var this String = String(v)
	return &this
}

func (this *String) MemSize() uint64                            { return uint64(len(*this)) }
func (this *String) IsDeletable(key, path any) bool             { return true }
func (this *String) TypeID() uint8                              { return STRING }
func (this *String) Equal(other any) bool                       { return *this == *(other.(*String)) }
func (this *String) CopyTo(v any) (any, uint32, uint32, uint32) { return v, 0, 1, 0 }
func (this *String) Get() (any, uint32, uint32)                 { return string(*this), 1, 0 }

func (this *String) IsNumeric() bool     { return false }
func (this *String) IsCommutative() bool { return false }
func (this *String) HasLimits() bool     { return false }

func (this *String) Value() any         { return this }
func (this *String) Delta() (any, bool) { return this, true }
func (this *String) DeltaSign() bool    { return true } // delta sign
func (this *String) Limits() (any, any) { return nil, nil }

func (this *String) CloneDelta() (any, bool) { return this.Clone(), true }
func (this *String) SetValue(v any)          { this.SetDelta(v, true) }
func (this *String) Preload(_ string, _ any) {}

func (this *String) IsDeltaApplied() bool              { return true }
func (this *String) ResetDelta()                       { this.SetDelta(common.New[String](""), true) }
func (this *String) SetDelta(v any, _ bool)            { *this = (*v.(*String)) }
func (*String) GetCascadeSub(_ string, _ any) []string { return nil } // The entries to delete when this is deleted.

func (this *String) New(_, delta, _, _, _ any) any {
	return common.IfThenDo1st(delta != nil && delta.(*String) != nil, func() any { return delta.(*String).Clone() }, any(this))
}

func (this *String) Clone() any {
	value := strings.Clone(string(*this))
	return (*String)(&value)
}

func (this *String) Set(value any, source any) (any, uint32, uint32, uint32, error) {
	if value != nil {
		*this = String(*(value.(*String)))
	}
	return this, 0, 1, 0, nil
}

func (this *String) ApplyDelta(typedVals []intf.Type) (intf.Type, int, error) {
	for _, v := range typedVals {
		if this == nil && v != nil { // New value
			this = v.(*String)
		}

		if this != nil && v != nil {
			this.Set(v.(*String), nil)
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
