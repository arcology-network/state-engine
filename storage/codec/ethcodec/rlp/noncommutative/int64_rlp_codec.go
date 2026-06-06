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
	"math/big"

	"github.com/arcology-network/common-lib/common"
	noncommutative "github.com/arcology-network/common-lib/crdt/noncommutative"
	"github.com/ethereum/go-ethereum/rlp"
)

type Int64RLP struct{ noncommutative.Int64 }

func (this *Int64RLP) Encode() ([]byte, error) {
	return rlp.EncodeToBytes(new(big.Int).SetInt64(int64(*this.Int64.Value().(*noncommutative.Int64))))
}

func (this *Int64RLP) Decode(buffer []byte) (any, error) {
	var v big.Int
	if err := rlp.DecodeBytes(buffer, &v); err != nil {
		return nil, err
	}
	return common.New(noncommutative.Int64(v.Int64())), nil
}
