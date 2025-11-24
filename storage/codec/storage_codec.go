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

package ethcodec

import (

	// "github.com/arcology-network/concurrenturl/commutative"

	statecommon "github.com/arcology-network/state-engine/common"
	arcocodec "github.com/arcology-network/state-engine/storage/codec/arcocodec"
	rlp "github.com/arcology-network/state-engine/storage/codec/ethcodec/rlp"
)

// Encoding and decoding for storage
// Righ now there are two codecs, one for Arcology native storage and one for Ethereum RLP encoding.
// This selector chooses the right codec based on the key string.
type StorageCodec struct {
	ethCodec    rlp.RlpCodec
	arcoCodec   arcocodec.Codec
	pathBuilder statecommon.PathBuilder
}

func NewStorageCodec() *StorageCodec {
	return &StorageCodec{
		pathBuilder: *statecommon.NewPathBuilder(statecommon.ETH_PATH),
	}
}

// The Encode function chooses the right codec based on the the key string.
func (this *StorageCodec) Encode(key string, v any) ([]byte, error) {
	if statecommon.ShouldPersistToEth(key) {
		return this.ethCodec.Encode(key, v)
	}
	return this.arcoCodec.Encode(key, v)
}

// The Decode function chooses the right codec based on the the key string.
func (this *StorageCodec) Decode(key string, buffer []byte, T any) any {
	if statecommon.ShouldPersistToEth(key) {
		return this.ethCodec.Decode(key, buffer, T)
	}
	return this.arcoCodec.Decode(key, buffer, T)
}
