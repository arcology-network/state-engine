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
	"github.com/arcology-network/common-lib/codec"
	"github.com/arcology-network/common-lib/crdt/commutative"
	"github.com/ethereum/go-ethereum/rlp"
)

type Int64RLP struct{ commutative.Int64 }

func (i *Int64RLP) Encode() ([]byte, error) {
	v := codec.Int64(i.Value().(int64)).Encode()
	if i.HasLimits() {
		min, max := i.Limits()
		minBytes := codec.Int64(min.(int64)).Encode()
		maxBytes := codec.Int64(max.(int64)).Encode()
		return rlp.EncodeToBytes([]any{v, minBytes, maxBytes})
	}

	return rlp.EncodeToBytes(v)
}

func (Int64RLP) Decode(buffer []byte) (any, error) {
	if len(buffer) == 9 {
		var vBuf []byte
		if err := rlp.DecodeBytes(buffer, &vBuf); err != nil {
			return nil, err
		}

		v := new(codec.Int64).Decode(vBuf).(codec.Int64)
		this := commutative.NewUnboundedInt64().(*commutative.Int64)
		this.SetValue(int64(v))
		return this, nil
	}

	var arr []any
	if err := rlp.DecodeBytes(buffer, &arr); err != nil {
		return nil, err
	}

	var this *commutative.Int64
	v := new(codec.Int64).Decode(arr[0].([]byte)).(codec.Int64)
	min := new(codec.Int64).Decode(arr[1].([]byte)).(codec.Int64)
	max := new(codec.Int64).Decode(arr[2].([]byte)).(codec.Int64)
	this = commutative.NewBoundedInt64(int64(min), int64(max)).(*commutative.Int64)
	this.SetValue(int64(v))
	return this, nil
}
