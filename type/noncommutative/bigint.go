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
	"bytes"
	"math/big"

	// "github.com/arcology-network/common-lib/common"
	"github.com/arcology-network/common-lib/common"
	stgcommon "github.com/arcology-network/state-engine/common"
)

// type Bigint codec.Bigint

type Bigint big.Int

func NewBigint(v int64) any {
	var value big.Int
	value.SetInt64(v)
	this := Bigint(value)
	return &this
}

func (this *Bigint) MemSize() uint64                            { return uint64((*big.Int)(this).BitLen()) }
func (this *Bigint) IsDeletable(key, path any) bool             { return true }
func (this *Bigint) TypeID() uint8                              { return BIGINT }
func (this *Bigint) CopyTo(v any) (any, uint32, uint32, uint32) { return v, 0, 1, 0 }

func (this *Bigint) Equal(other any) bool {
	return bytes.Equal((*big.Int)(this).Bytes(), (*big.Int)(other.(*Bigint)).Bytes())
}

func (this *Bigint) Clone() any {
	v := big.Int(*this)
	return (*Bigint)(new(big.Int).Set(&v))
}

func (this *Bigint) IsNumeric() bool     { return true }
func (this *Bigint) IsCommutative() bool { return false }
func (this *Bigint) HasLimits() bool     { return false }

func (this *Bigint) Value() any         { return (this) }
func (this *Bigint) Delta() (any, bool) { return this, (*big.Int)(this).Sign() > 0 }
func (this *Bigint) Limits() (any, any) { return nil, nil }

func (this *Bigint) CloneDelta() (any, bool) { return this.Clone(), (*big.Int)(this).Sign() > 0 }

func (this *Bigint) SetValue(v any)          { this.SetDelta(v, true) } // The sign is only a placeholder, the value carries the sign by itself.
func (this *Bigint) Preload(_ string, _ any) {}

func (this *Bigint) IsDeltaApplied() bool   { return true }
func (this *Bigint) ResetDelta()            { this.SetDelta(big.NewInt(0), true) }
func (this *Bigint) SetDelta(v any, _ bool) { (*big.Int)(this).Set((*big.Int)(v.(*Bigint))) }

func (this *Bigint) Get() (any, uint32, uint32)        { return *((*big.Int)(this)), 1, 0 }
func (*Bigint) GetCascadeSub(_ string, _ any) []string { return nil } // // The entries to delete when this is deleted.

func (this *Bigint) New(_, delta, _, _, _ any) any {
	return common.IfThenDo1st(delta != nil && delta.(*Bigint) != nil, func() any { return delta.(*Bigint).Clone() }, any(this))
}

func (this *Bigint) Set(value any, _ any) (any, uint32, uint32, uint32, error) {
	if value != nil {
		*this = *(value.(*Bigint))
	}
	return this, 0, 1, 0, nil
}

func (this *Bigint) ApplyDelta(typedVals []stgcommon.Type) (stgcommon.Type, int, error) {
	for _, v := range typedVals {
		if this == nil && v != nil { // New value
			this = v.(*Bigint)
		}

		if this != nil && v != nil {
			this.Set(v.(*Bigint), nil)
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
