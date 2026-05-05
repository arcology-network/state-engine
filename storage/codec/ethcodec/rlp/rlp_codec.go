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

package ethrlp

import (
	"reflect"

	crdtc "github.com/arcology-network/common-lib/crdt/commutative"
	crdtnc "github.com/arcology-network/common-lib/crdt/noncommutative"
	commcodec "github.com/arcology-network/state-engine/storage/codec/ethcodec/rlp/commutative"
	noncommcodec "github.com/arcology-network/state-engine/storage/codec/ethcodec/rlp/noncommutative"
)

type RlpCodec struct{}

func (RlpCodec) Encode(key string, v any) ([]byte, error) {
	if v == nil {
		return []byte{}, nil // Deletion
	}

	switch v := v.(type) {
	// Commutative types
	case *crdtc.Uint64:
		return (&commcodec.Uint64RLP{Uint64: *(v)}).Encode() // For nonce.
	case *crdtc.U256:
		return (&commcodec.U256RLP{U256: *(v)}).Encode()
	case *crdtc.Int64:
		return (&commcodec.Int64RLP{Int64: *(v)}).Encode()
	case *crdtc.Path:
		return (&commcodec.PathRLP{Path: *(v)}).Encode()

	// Non-commutative types
	case *crdtnc.Int64:
		return (&noncommcodec.Int64RLP{Int64: *(v)}).Encode()
	case *crdtnc.Uint64:
		return (&noncommcodec.Uint64RLP{Uint64: *(v)}).Encode()
	case *crdtnc.Uint32:
		return (&noncommcodec.Uint32RLP{Uint32: *(v)}).Encode()
	case *crdtnc.Bigint:
		return (&noncommcodec.BigintRLP{Bigint: *(v)}).Encode()
	case *crdtnc.Bytes:
		return (&noncommcodec.BytesRLP{Bytes: *(v)}).Encode()
	case *crdtnc.String:
		return (&noncommcodec.StringRLP{String: *(v)}).Encode()

	default:
		panic("RlpCodec Encoder: unsupported type for RlpCodec encoder")
	}
}

func (RlpCodec) Decode(_ string, buffer []byte, T any) any {
	if T == nil {
		return buffer
	}

	switch T.(type) {
	// Commutative types
	case *crdtc.Uint64:
		return new(commcodec.Uint64RLP).Decode(buffer)
	case *crdtc.U256:
		return new(commcodec.U256RLP).Decode(buffer)
	case *crdtc.Int64:
		return new(commcodec.Int64RLP).Decode(buffer)
	case *crdtc.Path:
		return new(commcodec.PathRLP).Decode(buffer)

	// Non-commutative types
	case *crdtnc.Int64:
		return new(noncommcodec.Int64RLP).Decode(buffer)
	case *crdtnc.Uint64:
		return new(noncommcodec.Uint64RLP).Decode(buffer)
	case *crdtnc.Uint32:
		return new(noncommcodec.Uint32RLP).Decode(buffer)
	case *crdtnc.Bigint:
		return new(noncommcodec.BigintRLP).Decode(buffer)
	case *crdtnc.Bytes:
		return new(noncommcodec.BytesRLP).Decode(buffer)
	case *crdtnc.String:
		return new(noncommcodec.StringRLP).Decode(buffer)

	default:
		panic("RlpCodec Decoder: unsupported type for RlpCodec decoder: " + reflect.TypeOf(T).String())
	}
}
