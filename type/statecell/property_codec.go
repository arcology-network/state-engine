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
	"bytes"
	"reflect"
	"unsafe"

	codec "github.com/arcology-network/common-lib/codec"
)

func (this *Property) Encode() []byte {
	buffer := make([]byte, this.Size())
	this.EncodeTo(buffer)
	return buffer
}

func (this *Property) HeaderSize() uint64 {
	return uint64(16 * codec.UINT64_LEN)
}

func (this *Property) Size() uint64 {
	return this.HeaderSize() + // uint64(9*codec.UINT64_LEN) +
		uint64(1) + // codec.Uint8(this.vType).Size() +
		uint64(8) + // codec.Uint64(uint64(this.tx)).Size() +
		uint64(8) + // codec.Uint64(this.generation).Size() +
		uint64(8) + // codec.Uint64(this.sequence).Size() +
		uint64(len(*this.path)) + // codec.String(*this.path).Size() +
		uint64(8) + // codec.Uint64(this.keyHash).Size() +
		uint64(8) + // codec.Uint64(this.reads).Size() +
		uint64(8) + // codec.Uint64(this.writes).Size() +
		uint64(8) + // codec.Uint64(this.deltaWrites).Size() +
		uint64(8) + // codec.Uint64(this.gasUsed).Size() +
		uint64(1) + //+  codec.Bool(this.isCommitted).Size() + // isDeleted
		uint64(1) + //+  codec.Bool(this.isCommitted).Size() +
		uint64(1) + //+  codec.Bool(this.ifSkipConflictCheck).Size() +
		uint64(8) + //+  sizeInStorage
		uint64(len(this.msg))
}

func (this *Property) FillHeader(buffer []byte) int {
	return codec.Encoder{}.FillHeader(
		buffer,
		[]uint64{
			uint64(codec.Uint8(this.vType).Size()),
			codec.Uint64(this.tx).Size(),
			codec.Uint64(this.generation).Size(),
			codec.Uint64(this.sequence).Size(),
			codec.String(*this.path).Size(),
			codec.Uint64(this.keyHash).Size(),
			codec.Uint64(this.reads).Size(),
			codec.Uint64(this.writes).Size(),
			codec.Uint64(this.deltaWrites).Size(),
			codec.Uint64(this.gasUsed).Size(),
			codec.Bool(this.isDeleted).Size(),
			codec.Bool(this.isCommitted).Size(),
			codec.Bool(this.ifSkipConflictCheck).Size(),
			codec.Uint64(this.sizeInStorage).Size(),
			codec.String(this.msg).Size(),
		},
	)
}

func (this *Property) EncodeTo(buffer []byte) int {
	offset := this.FillHeader(buffer)
	offset += codec.Uint8(this.vType).EncodeTo(buffer[offset:])
	offset += codec.Uint64(this.tx).EncodeTo(buffer[offset:])
	offset += codec.Uint64(this.generation).EncodeTo(buffer[offset:])
	offset += codec.Uint64(this.sequence).EncodeTo(buffer[offset:])
	offset += codec.String(*this.path).EncodeTo(buffer[offset:])
	offset += codec.Uint64(this.keyHash).EncodeTo(buffer[offset:])
	offset += codec.Uint64(this.reads).EncodeTo(buffer[offset:])
	offset += codec.Uint64(this.writes).EncodeTo(buffer[offset:])
	offset += codec.Uint64(this.deltaWrites).EncodeTo(buffer[offset:])
	offset += codec.Uint64(this.gasUsed).EncodeTo(buffer[offset:])
	offset += codec.Bool(this.isDeleted).EncodeTo(buffer[offset:])
	offset += codec.Bool(this.isCommitted).EncodeTo(buffer[offset:])
	offset += codec.Bool(this.ifSkipConflictCheck).EncodeTo(buffer[offset:])
	offset += codec.Uint64(this.sizeInStorage).EncodeTo(buffer[offset:])
	offset += codec.String(this.msg).EncodeTo(buffer[offset:])

	return offset
}

func (this *Property) Decode(buffer []byte) any {
	fields := codec.Byteset{}.Decode(buffer).(codec.Byteset)
	if len(fields) == 1 {
		return this
	}

	this.vType = uint8(reflect.Kind(codec.Uint8(1).Decode(fields[0]).(codec.Uint8)))
	this.tx = uint64(codec.Uint64(0).Decode(fields[1]).(codec.Uint64))
	this.generation = uint64(codec.Uint64(0).Decode(fields[2]).(codec.Uint64))
	this.sequence = uint64(codec.Uint64(0).Decode(fields[3]).(codec.Uint64))
	key := string(codec.String("").Decode(bytes.Clone(fields[4])).(codec.String))
	this.path = &key
	this.keyHash = uint64(codec.Uint64(0).Decode(fields[5]).(codec.Uint64))
	this.reads = uint32(codec.Uint64(1).Decode(fields[6]).(codec.Uint64))
	this.writes = uint32(codec.Uint64(1).Decode(fields[7]).(codec.Uint64))
	this.deltaWrites = uint32(new(codec.Uint64).Decode(fields[8]).(codec.Uint64))
	this.gasUsed = uint64(new(codec.Uint64).Decode(fields[9]).(codec.Uint64))
	this.isDeleted = bool(codec.Bool(false).Decode(fields[10]).(codec.Bool))
	this.isCommitted = bool(codec.Bool(false).Decode(fields[11]).(codec.Bool))
	this.ifSkipConflictCheck = bool(codec.Bool(true).Decode(fields[12]).(codec.Bool))
	this.sizeInStorage = uint64(new(codec.Uint64).Decode(fields[13]).(codec.Uint64))
	this.msg = string(codec.String("").Decode(bytes.Clone(fields[14])).(codec.String))

	this.pathBytes = unsafe.Slice(unsafe.StringData(*this.path), len(*this.path))
	return this
}

func (this *Property) GobEncode() ([]byte, error) {
	return this.Encode(), nil
}

func (this *Property) GobDecode(data []byte) error {
	v := this.Decode(data).(*Property)
	this.vType = v.vType
	this.path = v.path
	this.pathBytes = v.pathBytes
	this.keyHash = v.keyHash
	this.isCommitted = v.isCommitted
	this.ifSkipConflictCheck = v.ifSkipConflictCheck
	this.tx = v.tx
	this.generation = v.generation
	this.sequence = v.sequence
	this.reads = v.reads
	this.writes = v.writes
	this.deltaWrites = v.deltaWrites
	this.gasUsed = v.gasUsed
	this.isDeleted = v.isDeleted
	this.sizeInStorage = v.sizeInStorage
	this.msg = v.msg
	return nil
}
