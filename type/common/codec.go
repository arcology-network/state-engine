/*
 *   Copyright (c) 2023 Arcology Network

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

package common

import (
	stgcommon "github.com/arcology-network/storage-committer/common"
	commutative "github.com/arcology-network/storage-committer/type/commutative"
	noncommutative "github.com/arcology-network/storage-committer/type/noncommutative"
)

type Codec struct {
	ID uint8
}

func (Codec) Encode(_ string, value any) []byte {
	if value == nil {

		return []byte{} // Deletion
	}

	encoded := value.(stgcommon.Type).Encode()
	encoded = append(encoded, value.(stgcommon.Type).TypeID())
	return encoded
}

func (this Codec) Decode(_ string, buffer []byte, _ any) any {
	if len(buffer) == 0 {
		return nil
	}

	if len(buffer) == 0 || this.ID == 0 {
		this.ID = buffer[len(buffer)-1]
		buffer = buffer[0 : len(buffer)-1]
	}

	if this.ID == noncommutative.STRING { // delta big int
		stringer := noncommutative.String("")
		return stringer.Decode(buffer)
	}

	switch this.ID {
	case noncommutative.STRING: // delta big int
		stringer := noncommutative.String("")
		return stringer.Decode(buffer)

	case noncommutative.BIGINT: // big int pointer
		return (&noncommutative.Bigint{}).Decode(buffer)

	case noncommutative.BYTES: // big int pointer
		return (&noncommutative.Bytes{}).Decode(buffer)

	case commutative.PATH: // Path
		return (&commutative.Path{}).Decode(buffer)

	case commutative.INT64: // delta int 64
		return (&commutative.Int64{}).Decode(buffer)

	case commutative.UINT64: // delta int 64
		return (&commutative.Uint64{}).Decode(buffer)

	case commutative.UINT256: // delta big int
		return (&commutative.U256{}).Decode(buffer)

	case noncommutative.INT64:
		i64 := noncommutative.Int64(0)
		return i64.Decode(buffer)

		// case commutative.GROWONLY_SET: // GrowOnlySet
		// 	// panic("GrowOnlySet is not supported in this codec")
		// 	return (&commutative.GrowOnlySet[[]byte]{}).Decode(buffer)
	}

	// panic("Unknown type ID: " + string(this.ID))
	return nil
}

// func (Codec) Size(v interface{}) uint64 {
// 	switch v.(type) {
// 	case int64: // delta big int
// 		stringer := noncommutative.String("")
// 		return stringer.Decode(buffer)

// 	case noncommutative.INT64:
// 		i64 := noncommutative.Int64(0)
// 		return i64.Decode(buffer)

// 	case noncommutative.BIGINT: // big int pointer
// 		return (&noncommutative.Bigint{}).Decode(buffer)

// 	case noncommutative.BYTES: // big int pointer
// 		return (&noncommutative.Bytes{}).Decode(buffer)

// 	case commutative.PATH: // Path
// 		return (&commutative.Path{}).Decode(buffer)

// 	case commutative.UINT256: // delta big int
// 		return (&commutative.U256{}).Decode(buffer)

// 	case commutative.UINT64: // delta int 64
// 		return (&commutative.Uint64{}).Decode(buffer)

// 	case commutative.INT64: // delta int 64
// 		return (&commutative.Int64{}).Decode(buffer)
// 	}

// 	return nil
// }
