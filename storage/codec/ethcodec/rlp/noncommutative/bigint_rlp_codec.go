/*
 *   Copyright (c) 2025 Arcology Network

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

	noncommutative "github.com/arcology-network/common-lib/crdt/noncommutative"
	"github.com/ethereum/go-ethereum/rlp"
)

// For RLP encoding/decoding of Bigint type in non-commutative CRDTs only.
type BigintRLP struct{ noncommutative.Bigint }

func (this *BigintRLP) Encode() ([]byte, error) {
	return rlp.EncodeToBytes((*big.Int)(&this.Bigint))
}

func (this *BigintRLP) Decode(buffer []byte) any {
	rlp.DecodeBytes(buffer, (*big.Int)(&this.Bigint))
	return this
}
