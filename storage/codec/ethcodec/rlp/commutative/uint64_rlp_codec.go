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

type Uint64RLP struct{ commutative.Uint64 }

func (this *Uint64RLP) Encode() ([]byte, error) {
	if this.HasLimits() {
		min, max := this.Limits()
		v := []*big.Int{new(big.Int).SetUint64(
			this.Value().(uint64)),
			new(big.Int).SetUint64(min.(uint64)),
			new(big.Int).SetUint64(max.(uint64)),
		}
		return rlp.EncodeToBytes(v)
	}
	return rlp.EncodeToBytes(this.Value())
}

func (*Uint64RLP) Decode(buffer []byte) (any, error) {
	this := commutative.NewUnboundedUint64().(*commutative.Uint64)

	arr := make([]*big.Int, 3)
	err := rlp.DecodeBytes(buffer, &arr)

	// It can be encoded directly to save space, or with limits
	// So when error occurs, we try to decode as a single value, because
	// the error is likely due to that.
	if err != nil {
		var value big.Int
		if err = rlp.DecodeBytes(buffer, &value); err == nil {
			this.SetValue(value.Uint64())
		}
	} else {
		min, max := arr[1].Uint64(), arr[2].Uint64()
		this = commutative.NewBoundedUint64(min, max).(*commutative.Uint64)
		this.SetValue(arr[0].Uint64())
	}
	return this, err
}
