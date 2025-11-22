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

func TestBytesStorageCodec(t *testing.T) {
	v := noncommutative.NewBytes([]byte{0x01, 0x02, 0x03, 0x04}).(*noncommutative.Bytes)
	encoded, _ := (&Bytes{Bytes: *v}).Encode()
	decoded := new(Bytes).Decode(encoded).(*noncommutative.Bytes)

	if !decoded.Equal(v) {
		t.Fatal("decode mismatch")
	}
}
