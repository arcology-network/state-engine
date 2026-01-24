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

func TestUint32StorageCodec(t *testing.T) {
	v := noncommutative.NewUint32(12345)
	encoded, err := (&Uint32RLP{Uint32: *v}).Encode()
	if err != nil {
		t.Error("Encoding failed:", err)
	}
	decoded := new(Uint32RLP).Decode(encoded).(*noncommutative.Uint32)

	if !decoded.Equal(v) {
		t.Fatal("decode mismatch")
	}
}
