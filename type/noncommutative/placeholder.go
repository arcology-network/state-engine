/*
 *   Copyright (c) 2025 Arcology Network

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
	stgcommon "github.com/arcology-network/state-engine/common"
)

// Placeholder is a type that does not hold any value and is used as a placeholder in Archology's storage system.
type Placeholder struct{}

func NewPlaceholder(any) stgcommon.Type { return &Placeholder{} }

func (this *Placeholder) MemSize() uint64                            { return 0 }
func (this *Placeholder) IsDeletable(_, _ any) bool                  { return true } // If the input has the same type as this, return true
func (this *Placeholder) TypeID() uint8                              { return PLACEHOLDER }
func (this *Placeholder) CopyTo(v any) (any, uint32, uint32, uint32) { return v, 0, 1, 0 }

// create a new path
func (this *Placeholder) Clone() any           { return &Placeholder{} }
func (this *Placeholder) Equal(other any) bool { return true }

func (this *Placeholder) IsNumeric() bool     { return false }
func (this *Placeholder) IsCommutative() bool { return false }
func (this *Placeholder) HasLimits() bool     { return false }

func (this *Placeholder) Value() any              { return this }
func (this *Placeholder) Delta() (any, bool)      { return this, true }
func (this *Placeholder) DeltaSign() bool         { return true } // delta sign
func (this *Placeholder) Limits() (any, any)      { return nil, nil }
func (this *Placeholder) CloneDelta() (any, bool) { return this, true }
func (this *Placeholder) SetValue(v any)          { this.SetDelta(v, true) }
func (this *Placeholder) Preload(_ string, _ any) {}

func (this *Placeholder) IsDeltaApplied() bool              { return true }
func (this *Placeholder) ResetDelta()                       {}
func (this *Placeholder) SetDelta(v any, _ bool)            {}
func (this *Placeholder) Get() (any, uint32, uint32)        { return this, 1, 0 }
func (*Placeholder) GetCascadeSub(_ string, _ any) []string { return nil } // The entries to delete when this is deleted.
func (this *Placeholder) New(_, delta, _, _, _ any) any     { return &Placeholder{} }

func (this *Placeholder) Set(value any, _ any) (any, uint32, uint32, uint32, error) {
	return this, 0, 1, 0, nil
}

func (this *Placeholder) ApplyDelta(typedVals []stgcommon.Type) (stgcommon.Type, int, error) {
	return this, 0, nil
}

func (*Placeholder) Size() uint64          { return 0 }
func (*Placeholder) Encode() []byte        { return []byte{} }
func (*Placeholder) EncodeTo(_ []byte) int { return 0 }
func (*Placeholder) Decode([]byte) any     { return &Placeholder{} }

func (*Placeholder) Reset()                               {}
func (*Placeholder) Hash() [32]byte                       { return [32]byte{} }
func (*Placeholder) ShortHash() (uint64, bool)            { return 0, true }
func (*Placeholder) Print()                               {}
func (*Placeholder) StorageEncode(_ string) []byte        { return nil }
func (*Placeholder) StorageDecode(_ string, _ []byte) any { return &Placeholder{} }
