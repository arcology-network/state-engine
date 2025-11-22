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

package commutativecodec

import (
	"math/big"

	"github.com/arcology-network/common-lib/crdt/commutative"
	"github.com/ethereum/go-ethereum/rlp"
)

type Uint64 struct{ commutative.Uint64 }

func (this *Uint64) Encode(_ string) []byte {
	var buffer []byte
	if this.HasLimits() {
		min, max := this.Limits()
		v := []*big.Int{new(big.Int).SetUint64(this.Value().(uint64)), new(big.Int).SetUint64(min.(uint64)), new(big.Int).SetUint64(max.(uint64))}
		buffer, _ = rlp.EncodeToBytes(v)
	} else {
		buffer, _ = rlp.EncodeToBytes(this.Value())
	}
	return buffer
}

func (*Uint64) Decode(buffer []byte) any {
	this := commutative.NewUnboundedUint64().(*commutative.Uint64)

	arr := make([]*big.Int, 3)
	err := rlp.DecodeBytes(buffer, &arr)
	if err != nil {
		var value big.Int
		if err = rlp.DecodeBytes(buffer, &value); err == nil {
			// this.value = value.Uint64()
			this.SetValue(value.Uint64())
		}
	} else {
		min, max := arr[1].Uint64(), arr[2].Uint64()
		this = commutative.NewBoundedUint64(min, max).(*commutative.Uint64)
		this.SetValue(arr[0].Uint64())
		// this.value = arr[0].Uint64()
		// this.min = arr[1].Uint64()
		// this.max = arr[2].Uint64()
	}
	return this
}
