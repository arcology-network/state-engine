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

package noncommutativecodec

import (
	"testing"

	noncommutative "github.com/arcology-network/common-lib/crdt/noncommutative"
)

func TestUint64StorageCodec(t *testing.T) {
	v := noncommutative.NewUint64(12345)
	encoded, err := (&Uint64RLP{Uint64: *v}).Encode()
	if err != nil {
		t.Error("Encoding failed:", err)
	}

	decodedAny, err := new(Uint64RLP).Decode(encoded)
	if err != nil {
		t.Fatal("decode error:", err)
	}
	decoded := decodedAny.(*noncommutative.Uint64)
	if !decoded.Equal(v) {
		t.Fatal("decode mismatch")
	}
}
