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
	"testing"

	"github.com/arcology-network/common-lib/crdt/commutative"
	"github.com/arcology-network/common-lib/exp/slice"
	"github.com/holiman/uint256"
)

func TestBoundedUint64StorageCodec(t *testing.T) {
	c := commutative.NewBoundedUint64(1, 100).(*commutative.Uint64)
	c.SetValue(uint64(50))

	iv := &Uint64RLP{*c}
	buf, err := iv.Encode()
	if err != nil {
		t.Fatal("encode error:", err)
	}

	decodedAny, err := (&Uint64RLP{}).Decode(buf)
	if err != nil {
		t.Fatal("decode error:", err)
	}
	decoded := decodedAny.(*commutative.Uint64)

	min, max := decoded.Limits()
	if decoded.Value().(uint64) != uint64(50) ||
		min.(uint64) != uint64(1) ||
		max.(uint64) != uint64(100) {
		t.Fatal("decode mismatch")
	}
}

func TestUnboundedUint64StorageCodec(t *testing.T) {
	c := commutative.NewUnboundedUint64().(*commutative.Uint64)
	c.SetValue(uint64(150))

	buf, err := (&Uint64RLP{*c}).Encode()
	if err != nil {
		t.Fatal("encode error:", err)
	}

	decodedAny, err := (&Uint64RLP{}).Decode(buf)
	if err != nil {
		t.Fatal("decode error:", err)
	}
	decoded := decodedAny.(*commutative.Uint64)
	if decoded.Value() != uint64(150) {
		t.Fatal("decode mismatch")
	}

	min, max := decoded.Limits()
	// if min.(int64) != math.MinInt64 || max.(int64) != math.MaxInt64 {
	// 	t.Fatal("limits mismatch")
	// }

	max1 := uint256.NewInt(0)
	max1.SetBytes(slice.Fill(make([]byte, 32), math.MaxUint8))

	if decoded.Value().(uint64) != uint64(150) ||
		min.(uint64) != uint64(0) ||
		max.(uint64) != math.MaxUint64 {
		t.Fatal("decode mismatch")
	}
}
