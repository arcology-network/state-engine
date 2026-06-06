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

	"github.com/arcology-network/common-lib/crdt/noncommutative"
)

func TestBigintStorageCodec(t *testing.T) {
	v := noncommutative.NewBigint(111).(*noncommutative.Bigint)
	buffer, _ := (&BigintRLP{Bigint: *v}).Encode()
	decodedAny, err := (&BigintRLP{}).Decode(buffer)
	if err != nil {
		t.Fatal("decode error:", err)
	}
	decoded := decodedAny.(*BigintRLP)

	if !decoded.Equal(v) {
		t.Fatal("decode mismatch")
	}
}
