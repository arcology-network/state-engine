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

package statecell

import (
	codec "github.com/arcology-network/common-lib/codec"
	"github.com/arcology-network/common-lib/common"

	stgcommon "github.com/arcology-network/storage-committer/common"
	stgcodec "github.com/arcology-network/storage-committer/type/common"
)

func (this *StateCell) Encode() []byte {
	buffer := make([]byte, this.Size())
	this.EncodeTo(buffer)
	return buffer
}

func (this *StateCell) HeaderSize() uint64 {
	return uint64(3 * codec.UINT64_LEN)
}

func (this *StateCell) Sizes() []uint64 {
	return []uint64{
		this.HeaderSize(),
		this.Property.Size(),
		this.value.(stgcommon.Type).Size(),
	}
}

func (this *StateCell) Size() uint64 {
	return this.HeaderSize() +
		this.Property.Size() +
		common.IfThenDo1st(this.value != nil, func() uint64 { return this.value.(stgcommon.Type).Size() }, 0)
}

func (this *StateCell) FillHeader(buffer []byte) int {
	return codec.Encoder{}.FillHeader(
		buffer,
		[]uint64{
			this.Property.Size(),
			common.IfThenDo1st(this.value != nil, func() uint64 { return this.value.(stgcommon.Type).Size() }, 0),
		},
	)
}

func (this *StateCell) EncodeTo(buffer []byte) int {
	offset := this.FillHeader(buffer)

	offset += this.Property.EncodeTo(buffer[offset:])
	offset += common.IfThenDo1st(this.value != nil, func() int {
		return codec.Bytes(this.value.(stgcommon.Type).Encode()).EncodeTo(buffer[offset:])
	}, 0)

	return offset
}

func (this *StateCell) Decode(buffer []byte) any {
	fields := codec.Byteset{}.Decode(buffer).(codec.Byteset)
	property := (&Property{}).Decode(fields[0]).(*Property)

	return &StateCell{
		*property,
		(&stgcodec.Codec{ID: property.vType}).Decode(*property.path, fields[1], this.value),
		fields[1], // Keep copy, should expire as soon as the value is updated
	}
}

func (this *StateCell) GetEncoded() []byte {
	if this.value == nil {
		return []byte{}
	}

	if this.Value().(stgcommon.Type).IsCommutative() {
		return this.value.(stgcommon.Type).Value().(codec.Encodable).Encode()
	}

	if len(this.buf) > 0 {
		return this.value.(stgcommon.Type).Value().(codec.Encodable).Encode()
	}
	return this.buf
}

func (this *StateCell) GobEncode() ([]byte, error) {
	return this.Encode(), nil
}

func (this *StateCell) GobDecode(buffer []byte) error {
	*this = *(&StateCell{}).Decode(buffer).(*StateCell)
	return nil
}
