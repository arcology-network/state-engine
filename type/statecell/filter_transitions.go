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
	"strings"

	common "github.com/arcology-network/common-lib/common"
	stgcommon "github.com/arcology-network/storage-committer/common"
	"github.com/arcology-network/storage-committer/type/commutative"
)

// IPTransition stands for intra-process transition. It is used to filter out the fields that are not needed in inter-thread transitions to save
// time spent on encoding and decoding.
type IPTransition struct {
	*StateCell
	Err error
}

func (this IPTransition) From(v *StateCell) *StateCell {
	if v == nil ||
		v.IsReadOnly() ||
		(v.Value() == nil && !v.IsCommitted()) { // Deletion of an non-existing entry or a read-only entry
		return nil
	}

	if v.Value() == nil { // Entry deletion
		return v
	}

	if this.Err != nil && !v.IfSkipConflictCheck() { // Keep balance and nonce transitions for failed ones.
		return nil
	}

	typed := v.Value().(stgcommon.Type)
	delta, sign := typed.Delta()

	min, max := typed.Limits()
	vtyped := typed.New(
		common.IfThen(!v.Value().(stgcommon.Type).IsCommutative() || common.IsType[*commutative.Path](v.Value()),
			nil,
			v.Value().(stgcommon.Type).Value()), // Keep Non-path commutative variables (u256, u64) only
		delta,
		sign,
		min,
		max,
	)

	vt := vtyped.(stgcommon.Type)
	return v.New(
		&v.Property,
		vt,
		[]byte{},
	)
}

// ITT stands for inter-thread transition. It is used to filter out the fields that are not needed in inter-thread transitions to save
// time spent on encoding and decoding, which is only needed in inter-process scenarios.
type ITTransition struct {
	IPTransition
	Err error
}

func (this ITTransition) From(v *StateCell) *StateCell {
	unival := IPTransition{Err: this.Err}.From(v)

	// if unival == nil { // Entry deletion
	// 	return unival
	// }

	// converted := common.IfThenDo1st(value != nil, func() *StateCell { return value.(*StateCell) }, nil)
	// if converted == nil {
	// 	return nil
	// }

	// The unival is nil when either of the following conditions is met:
	// 1. The unival represents a read-only entry.
	// 2. The unival represents an attempt to delete a non-existing entry.
	if unival == nil || unival.Value() == nil { // Entry deletion
		return unival
	}

	typed := unival.Value().(stgcommon.Type) // Get the typed value from the unival
	delta, sign := typed.CloneDelta()
	typed.SetDelta(delta, sign)
	// typedNew := typed.New(
	// 	nil,
	// 	typed.CloneDelta(),
	// 	typed.DeltaSign(),
	// 	typed.Min(),
	// 	typed.Max(),
	// ).(stgcommon.Type)

	// typedNew.SetDelta(codec.Clone(typedNew.Delta()))
	// converted.SetValue(typed) // Reuse the univalue wrapper
	return unival
}

// Get property univalue entries
type RuntimeProperty struct {
	*StateCell
	Err error
}

func (this RuntimeProperty) From(unival *StateCell) *StateCell {
	if unival == nil || unival.Value() == nil { // Entry deletion
		return unival
	}

	path := *unival.GetPath()
	if strings.Contains(path[stgcommon.ETH_ACCOUNT_FULL_LENGTH:], stgcommon.FUNC_PROFILE_PATH) {
		return unival
	}
	return nil
}
