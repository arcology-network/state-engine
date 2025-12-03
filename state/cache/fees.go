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

package cache

// const (
// 	READ_NONEXIST          = uint64(3)
// 	READ_COMMITTED_FROM_DB = uint64(1000) // Read for the state db
// )

// type Fee struct{}

// func (Fee) Reader(v interface{}) uint64 { // Call this before setting the value attribute to nil
// 	if v == nil {
// 		return READ_NONEXIST
// 	}

// 	typedv := v.(*statecell.StateCell).Value()
// 	dataSize := common.IfThenDo1st(typedv != nil, func() uint64 { return uint64(typedv.(crdtcommon.CRDT).MemSize()) }, 0)
// 	return common.IfThen(
// 		v.(*statecell.StateCell).Reads() > 1, // Is hot loaded
// 		common.Max(dataSize/32, 1)*3,
// 		(dataSize/32)*5000,
// 	)
// }

// func (Fee) Writer(key string, v interface{}, writecache *StateCache) int64 { // May get refunds sometimes
// 	committedSize := uint64(0)
// 	committedv, _ := writecache.ReadOnlyStore().Retrieve(key, v)

// 	if data, ok := committedv.([]byte); ok {
// 		committedSize = uint64(len(data))
// 	} else {
// 		committedSize = common.IfThenDo1st(committedv != nil, func() uint64 { return uint64(committedv.(crdtcommon.CRDT).MemSize()) }, 0)
// 	}

// 	dataSize := common.IfThenDo1st(v != nil, func() uint64 { return uint64(v.(crdtcommon.CRDT).Size()) }, 0)
// 	return common.IfThen(
// 		committedSize > 0,
// 		common.Max(int64(dataSize/32), 1)*20000,
// 		common.IfThen(
// 			int64(dataSize) > int64(committedSize),
// 			int64(dataSize),
// 			((int64(committedSize)-int64(dataSize))/32)*20000,
// 		),
// 	)
// }
