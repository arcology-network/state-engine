package ethrlp

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

import (

	// "github.com/arcology-network/concurrenturl/commutative"

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
	case *crdtc.Uint64:
		return (&commcodec.Uint64{Uint64: *(v)}).Encode() // For nonce.
	case *crdtc.U256:
		return (&commcodec.U256{U256: *(v)}).Encode()
	case *crdtc.Int64:
		return (&commcodec.Int64{Int64: *(v)}).Encode()
	case *crdtnc.Int64:
		return (&noncommcodec.Int64{Int64: *(v)}).Encode()
	case *crdtnc.Bigint:
		return (&noncommcodec.Bigint{Bigint: *(v)}).Encode()
	case *crdtnc.Bytes:
		return (&noncommcodec.Bytes{Bytes: *(v)}).Encode()
	case *crdtnc.String:
		return (&noncommcodec.String{String: *(v)}).Encode()
	case *crdtc.Path:
		return (&commcodec.Path{Path: *(v)}).Encode()
	default:
		panic("RlpCodec Encoder: unsupported type for RlpCodec codec")
	}
}

func (RlpCodec) Decode(key string, buffer []byte, T any) any {
	switch T.(type) {
	case crdtc.Uint64:
		return new(commcodec.Uint64).Decode(buffer)
	case crdtc.U256:
		return new(commcodec.U256).Decode(buffer)
	case crdtc.Int64:
		return new(commcodec.Int64).Decode(buffer)
	case crdtnc.Int64:
		return new(noncommcodec.Int64).Decode(buffer)
	case crdtnc.Bigint:
		return new(noncommcodec.Bigint).Decode(buffer)
	case crdtnc.Bytes:
		return new(noncommcodec.Bytes).Decode(buffer)
	case crdtnc.String:
		return new(noncommcodec.String).Decode(buffer)
	case crdtc.Path:
		return new(commcodec.Path).Decode(buffer)
	default:
		panic("RlpCodec Decoder: unsupported type for RlpCodec codec")
	}
}
