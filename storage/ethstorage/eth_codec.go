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

package ethstorage

import (

	// "github.com/arcology-network/concurrenturl/commutative"
	commcodec "github.com/arcology-network/state-engine/storage/ethcodec/rlp/commutative"
	noncommcodec "github.com/arcology-network/state-engine/storage/ethcodec/rlp/noncommutative"

	commutative "github.com/arcology-network/common-lib/crdt/commutative"
	noncommutative "github.com/arcology-network/common-lib/crdt/noncommutative"
)

type Rlp struct{}

func (Rlp) Encode(key string, v any) ([]byte, error) {
	if v == nil {
		return []byte{}, nil // Deletion
	}

	switch v.(type) {
	case commutative.Int64:
		return (&commcodec.Int64{Int64: v.(commutative.Int64)}).Encode()
	case noncommutative.Bigint:
		return (&noncommcodec.Bigint{Bigint: v.(noncommutative.Bigint)}).Encode()
	case noncommutative.Bytes:
		return (&noncommcodec.Bytes{Bytes: v.(noncommutative.Bytes)}).Encode()
	case noncommutative.String:
		return (&noncommcodec.String{String: v.(noncommutative.String)}).Encode()
	default:
		panic("unsupported type for Rlp codec")
	}
}

func (Rlp) Decode(key string, buffer []byte, T any) any {
	switch T.(type) {
	case commutative.Int64:
		return new(commcodec.Int64).Decode(buffer)
	case noncommutative.Bigint:
		return new(noncommcodec.Bigint).Decode(buffer)
	case noncommutative.Bytes:
		return new(noncommcodec.Bytes).Decode(buffer)
	case noncommutative.String:
		return new(noncommcodec.String).Decode(buffer)
	default:
		panic("unsupported type for Rlp codec")
	}
}
