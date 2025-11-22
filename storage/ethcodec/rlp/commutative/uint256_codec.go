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
	"github.com/holiman/uint256"
)

type U256 struct{ commutative.U256 }

func (this *U256) StorageEncode(_ string) []byte {
	var buffer []byte
	if this.HasLimits() {
		min, max := this.Limits()
		buffer, _ = rlp.EncodeToBytes([]any{this.Value(), min, max})
	} else {
		v := this.U256.Value().(uint256.Int)
		buffer, _ = rlp.EncodeToBytes(v.ToBig())
	}
	return buffer
}

func (*U256) StorageDecode(_ string, buffer []byte) any {
	this := commutative.NewUnboundedU256().(*U256)

	var arr []any
	err := rlp.DecodeBytes(buffer, &arr)
	if err != nil {
		var v2 big.Int
		if err = rlp.DecodeBytes(buffer, &v2); err == nil {
			// this.value.SetFromBig(&v2)
			v := this.Value().(uint256.Int)
			v.SetFromBig(&v2)
		}
	} else {
		min, max := uint256.NewInt(0), uint256.NewInt(0)
		min.SetFromBig(arr[1].(*big.Int))
		max.SetFromBig(arr[2].(*big.Int))

		commutative.NewBoundedU256(min, max)
		v := this.Value().(uint256.Int)
		v.SetBytes(arr[0].([]byte))
		// this.min.SetBytes(arr[1].([]byte))
		// this.max.SetBytes(arr[2].([]byte))
	}
	return this
}
