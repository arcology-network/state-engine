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

func (this *U256) Encode() ([]byte, error) {
	if this.HasLimits() {
		min, max := this.Limits()
		return rlp.EncodeToBytes([]any{this.Value(), min, max})
	}
	v := this.U256.Value().(uint256.Int)
	return rlp.EncodeToBytes(v.ToBig())
}

func (*U256) Decode(buffer []byte) any {
	this := commutative.NewUnboundedU256().(*commutative.U256)

	var arr []any
	err := rlp.DecodeBytes(buffer, &arr)

	// U256 can be encoded as big.Int directly to save space, or with limits
	// So when error occurs, we try to decode as a single big.Int value, because
	// the error is likely due to that.
	if err != nil {
		var v2 big.Int
		if err = rlp.DecodeBytes(buffer, &v2); err == nil {
			v, min, max := uint256.NewInt(0), uint256.NewInt(0), uint256.NewInt(0)
			v.SetFromBig(&v2)
			this.SetValue(*v)

			max.SetAllOne() // Set the max to default
			this.SetLimits(*min, *max)
		}
	} else {
		v, min, max := uint256.NewInt(0), uint256.NewInt(0), uint256.NewInt(0)
		v.SetBytes(arr[0].([]byte))
		min.SetBytes(arr[1].([]byte))
		max.SetBytes(arr[2].([]byte))
		this.SetLimits(*min, *max)
		this.SetValue(*v)
	}
	return this
}
