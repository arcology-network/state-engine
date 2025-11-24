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

type Uint64 struct{ noncommutative.Uint64 }

// func (this *Uint64) Encode() ([]byte, error) {
// 	return codec.Uint64(this.Uint64).Encode()
// }
// func (this *Uint64) Decode(buffer []byte) any {
// 	return new(noncommutative.Uint64).Decode(buffer).(*noncommutative.Uint64)
// }

func (this *Uint64) Encode() ([]byte, error) {
	return rlp.EncodeToBytes(*this)
	// return codec.Uint64(this.Uint64).Encode()
}
func (this *Uint64) Decode(buffer []byte) any {
	var v Uint64
	if err := rlp.DecodeBytes(buffer, &v); err != nil {
		panic("Failed to decode uint64")
	}
	return &v.Uint64
}
