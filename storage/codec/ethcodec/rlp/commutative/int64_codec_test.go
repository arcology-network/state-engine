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
)

func TestBoundedInt64StorageCodec(t *testing.T) {
	c := commutative.NewBoundedInt64(1, 100).(*commutative.Int64)
	c.SetValue(int64(50))

	buf, err := (&Int64RLP{*c}).Encode()
	if err != nil {
		t.Fatal("encode error:", err)
	}

	decoded, err := (Int64RLP{}).Decode(buf)
	if err != nil {
		t.Fatal("decode error:", err)
	}
	if decoded.(*commutative.Int64).Value() != int64(50) {
		t.Fatal("decode mismatch")
	}
}

func TestUnboundedInt64StorageCodec(t *testing.T) {
	c := commutative.NewUnboundedInt64().(*commutative.Int64)
	c.SetValue(int64(150))

	iv := &Int64RLP{*c}

	buf, err := iv.Encode()
	if err != nil {
		t.Fatal("encode error:", err)
	}

	decoded, err := (Int64RLP{}).Decode(buf)
	if err != nil {
		t.Fatal("decode error:", err)
	}
	decodedInt64 := decoded.(*commutative.Int64)
	if decodedInt64.Value() != int64(150) {
		t.Fatal("decode mismatch")
	}

	min, max := decodedInt64.Limits()
	if min.(int64) != math.MinInt64 || max.(int64) != math.MaxInt64 {
		t.Fatal("limits mismatch")
	}
}
