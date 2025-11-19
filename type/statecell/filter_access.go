/*
 *   Copyright (c) 2023 Arcology Network
 *   All rights reserved.

 *   Licensed under the Apache License, Version 2.0 (the "License");
 *   you may not use this file except in compliance with the License.
 *   You may obtain a copy of the License at

 *   http://www.apache.org/licenses/LICENSE-2.0

 *   Unless required by applicable law or agreed to in writing, software
 *   distributed under the License is distributed on an "AS IS" BASIS,
 *   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *   See the License for the specific language governing permissions and
 *   limitations under the License.
 */

package statecell

import (
	"github.com/arcology-network/common-lib/common"
	stgcommon "github.com/arcology-network/state-engine/common"
)

// IPAccess is purely for inter-process communication, the valuee get copied in
// the process of serialization anyway.
type IPAccess struct {
	*StateCell
	Err error
}

func (this IPAccess) From(v *StateCell) *StateCell {
	if this.Err != nil || v.IfSkipConflictCheck() || v.PathLookupOnly() {
		return nil
	}

	if v.Value() == nil {
		return v
	}

	value := v.Value().(stgcommon.Type)

	return v.New(
		&v.Property,
		common.IfThen(value.IsCommutative() && value.IsNumeric(), value, nil), // commutative but not meta, for the accumulator
		[]byte{},
	)
}

// ITAccess is used to filter out the fields that are not needed in inter-thread
// transitions to save time spent on encoding and decoding.

// The biggest difference between ITAccess and IPAccess is that ITAccess needs to
// make a deep copy of the value, while IPAccess does not. Because IPAccess is purely
// for inter-process communication, the valu get copied in the process of serialization anyway.

type ITAccess struct{ IPAccess }

func (this ITAccess) From(v *StateCell) *StateCell {
	value := this.IPAccess.From(v)
	// converted := common.IfThenDo1st(value != nil, func() *StateCell { return value.(*StateCell) }, nil)
	if value == nil {
		return nil
	}

	if value.Value() == nil { // regular value or Entry deletion
		return value
	}

	typed := value.Value().(stgcommon.Type)
	delta, sign := typed.Delta()
	min, max := typed.Limits()
	newv := typed.New(
		nil,
		delta,
		sign,
		min,
		max,
	).(stgcommon.Type)

	if typed.IsCommutative() && typed.IsNumeric() { // For the accumulator, commutative u64 & U256
		newv.SetValue(typed.Value())
	}

	return value.New(
		&value.Property,
		typed,
		[]byte{},
	)
}
