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
	"fmt"
	"sort"

	"github.com/arcology-network/common-lib/codec"
	"github.com/arcology-network/common-lib/common"
	"github.com/arcology-network/common-lib/exp/slice"
)

func (this StateCells) Size() uint64 {
	size := (len(this) + 1) * codec.UINT64_LEN
	for _, v := range this {
		size += int(v.Size())
	}
	return uint64(size)
}

func (this StateCells) Sizes() []int {
	sizes := make([]int, len(this))
	for i, v := range this {
		sizes[i] = common.IfThenDo1st(v != nil, func() int { return int(v.Size()) }, 0)
	}
	return sizes
}

func (this StateCells) Encode(selector ...any) []byte {
	lengths := make([]uint64, len(this))
	if len(lengths) == 0 {
		return []byte{}
	}

	slice.ParallelForeach(this, 6, func(i int, _ **StateCell) {
		if this[i] != nil {
			lengths[i] = this[i].Size()
		}
	})

	offsets := make([]uint64, len(this)+1)
	for i := 0; i < len(lengths); i++ {
		offsets[i+1] = offsets[i] + lengths[i]
	}

	headerLen := uint64((len(this) + 1) * codec.UINT64_LEN)
	buffer := make([]byte, headerLen+offsets[len(offsets)-1])
	codec.Uint32(len(this)).EncodeTo(buffer)

	slice.ParallelForeach(this, 6, func(i int, _ **StateCell) {
		codec.Uint32(offsets[i]).EncodeTo(buffer[(i+1)*codec.UINT64_LEN:])
		this[i].EncodeTo(buffer[headerLen+offsets[i]:])
	})
	return buffer
}

func (StateCells) Decode(bytes []byte) any {
	if len(bytes) == 0 {
		return StateCells{}
	}

	buffers := [][]byte(codec.Byteset{}.Decode(bytes).(codec.Byteset))
	cells := make([]*StateCell, len(buffers))

	slice.ParallelForeach(buffers, 6, func(i int, _ *[]byte) {
		v := (&StateCell{}).Decode(buffers[i])
		cells[i] = v.(*StateCell)
	})
	return StateCells(cells)
}

func (StateCells) DecodeWithMempool(bytes []byte, get func() *StateCell, put func(any)) any {
	if len(bytes) == 0 {
		return nil
	}

	buffers := [][]byte(codec.Byteset{}.Decode(bytes).(codec.Byteset))
	univalues := make([]*StateCell, len(buffers))

	slice.ParallelForeach(buffers, 6, func(i int, _ *[]byte) {
		v := get()
		v.reclaimFunc = put
		univalues[i] = v.Decode(buffers[i]).(*StateCell)
	})
	return StateCells(univalues)
}

// func (StateCells) DecodeV2(bytesset [][]byte, get func() any, put func(any)) StateCells {
// 	univalues := make([]*StateCells, len(bytesset))
// 	for i := range bytesset {
// 		v := get().(*StateCells)
// 		v.reclaimFunc = put
// 		v.Decode(bytesset[i])
// 		univalues[i] = v
// 	}
// 	return StateCells(univalues)
// }

func (this StateCells) GobEncode() ([]byte, error) {
	return this.Encode(), nil
}

func (this *StateCells) GobDecode(data []byte) error {
	v := this.Decode(data)
	*this = v.(StateCells)
	return nil
}

// Print the univalues if the satisfied the existing condition
func (this StateCells) Print(condition ...func(v *StateCell) bool) {
	sorted := slice.Clone(this)
	sort.Slice(sorted, func(i, j int) bool {
		return (*sorted[i].GetPath()) < (*sorted[j].GetPath())
	})

	for i, v := range sorted {
		if len(condition) > 0 && !condition[0](v) {
			continue
		}

		fmt.Print(i, ": ")
		v.Print()
	}
	fmt.Println(" --------------------  ")
}

// Print the univalues if the satisfied the existing condition
func (this StateCells) PrintUnsorted() {
	for i, v := range this {
		fmt.Print(i, ": ")
		v.Print()
	}
	fmt.Println(" --------------------  ")
}
