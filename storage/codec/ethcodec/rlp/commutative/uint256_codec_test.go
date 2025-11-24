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

package commutativecodec

import (
	"math"
	"reflect"
	"testing"

	"github.com/arcology-network/common-lib/crdt/commutative"
	"github.com/arcology-network/common-lib/exp/slice"
	"github.com/holiman/uint256"
)

func TestBoundedUint25StorageCodec(t *testing.T) {
	c := commutative.NewBoundedU256FromU64(1, 100).(*commutative.U256)
	c.SetValue(*uint256.NewInt(50))

	iv := &U256{*c}
	buf, err := iv.Encode()
	if err != nil {
		t.Fatal("encode error:", err)
	}

	decoded := (&U256{}).Decode(buf).(*commutative.U256)

	min, max := decoded.Limits()
	if decoded.Value().(uint256.Int) != *uint256.NewInt(50) ||
		min.(uint256.Int) != *uint256.NewInt(1) ||
		max.(uint256.Int) != *uint256.NewInt(100) {
		t.Fatal("decode mismatch")
	}
}

func TestUnboundedUint25StorageCodec(t *testing.T) {
	c := commutative.NewUnboundedU256().(*commutative.U256)
	c.SetValue(*uint256.NewInt(150))

	iv := &U256{*c}

	buf, err := iv.Encode()
	if err != nil {
		t.Fatal("encode error:", err)
	}

	decoded := (&U256{}).Decode(buf).(*commutative.U256)
	if decoded.Value() != *uint256.NewInt(150) {
		t.Fatal("decode mismatch")
	}

	min, max := decoded.Limits()
	// if min.(int64) != math.MinInt64 || max.(int64) != math.MaxInt64 {
	// 	t.Fatal("limits mismatch")
	// }

	max1 := uint256.NewInt(0)
	max1.SetBytes(slice.Fill(make([]byte, 32), math.MaxUint8))

	if decoded.Value().(uint256.Int) != *uint256.NewInt(150) ||
		min.(uint256.Int) != *uint256.NewInt(0) ||
		!reflect.DeepEqual(max.(uint256.Int), *max1) {
		t.Fatal("decode mismatch")
	}
}
