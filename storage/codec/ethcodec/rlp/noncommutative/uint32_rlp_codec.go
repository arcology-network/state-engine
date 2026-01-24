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
	noncommutative "github.com/arcology-network/common-lib/crdt/noncommutative"
	"github.com/ethereum/go-ethereum/rlp"
)

type Uint32RLP struct{ noncommutative.Uint32 }

func (this *Uint32RLP) Encode() ([]byte, error) {
	return rlp.EncodeToBytes(*this)
	// return codec.Uint32(this.Uint32).Encode()
}
func (this *Uint32RLP) Decode(buffer []byte) any {
	var v Uint32RLP
	if err := rlp.DecodeBytes(buffer, &v); err != nil {
		panic("Failed to decode uint32")
	}
	return &v.Uint32
}
