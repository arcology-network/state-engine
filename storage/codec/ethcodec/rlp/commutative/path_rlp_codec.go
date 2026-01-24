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
	"github.com/arcology-network/common-lib/crdt/commutative"
	"github.com/ethereum/go-ethereum/rlp"
	// performance "github.com/arcology-network/common-lib/mhasher"
)

type PathRLP struct{ commutative.Path }

func (this *PathRLP) Encode() ([]byte, error) {
	return rlp.EncodeToBytes(this.Path.Encode())
}

func (this *PathRLP) Decode(buffer []byte) any {
	var decoded []byte
	if err := rlp.DecodeBytes(buffer, &decoded); err != nil {
		panic(err)
	}
	return this.Path.Decode(decoded)
}
