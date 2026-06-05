/*
 *   Copyright (c) 2026 Arcology Network

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

package ccstorage

import (
	cachedstore "github.com/arcology-network/common-lib/storage/cachedstore"
	stgcodec "github.com/arcology-network/common-lib/storage/codec"
	commonintf "github.com/arcology-network/common-lib/storage/interface"
)

func NewLiveStorage[V0, V1 any](
	sizeOf func(V0) uint64,
	db commonintf.ReadWriteStore[string, V1],
	codec *stgcodec.StorageCodec[string, V0, string, V1],
) *cachedstore.CachedStore[string, V0, string, V1] {
	return cachedstore.NewCachedStore(
		db,    // Backend storage
		codec, // Codec for encoding/decoding
		1024,  // Cache capacity
		sizeOf,
	)
}
