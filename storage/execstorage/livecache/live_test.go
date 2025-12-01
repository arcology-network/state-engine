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

package livecache

import (
	"fmt"
	"testing"
	"time"

	paged "github.com/arcology-network/common-lib/exp/pagedslice"
)

func TestCompare(t *testing.T) {
	page := paged.NewPagedSlice[int](200000, 5, 0)
	t0 := time.Now()
	for i := 0; i < 1000*1000; i++ {
		page.PushBack(i)
		page.Get(i)
	}
	fmt.Println("paged slice pushback:", time.Since(t0))

	t0 = time.Now()
	for i := 0; i < 1000*1000; i++ {
		page.Get(i)
	}
	fmt.Println("paged slice get:", time.Since(t0))

	arr := make([]int, 0, 1000)
	t0 = time.Now()
	for i := 0; i < 1000*1000; i++ {
		arr = append(arr, i)
	}
	fmt.Println("plane append:", time.Since(t0))

	t0 = time.Now()
	v := 0
	for i := 0; i < len(arr); i++ {
		v = arr[i]
	}
	fmt.Println("plane get:", time.Since(t0), v, len(arr))

	lookup := make(map[int]int)
	t0 = time.Now()
	for i := 0; i < 1000*1000; i++ {
		lookup[i] = i
	}
	fmt.Println("map set:", time.Since(t0))

	t0 = time.Now()
	for i := 0; i < 1000*1000; i++ {
		_ = lookup[i]
	}
	fmt.Println("map get:", time.Since(t0))

}
