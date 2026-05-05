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

	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
	noncommutative "github.com/arcology-network/common-lib/crdt/noncommutative"
	paged "github.com/arcology-network/common-lib/exp/pagedslice"
	stgcodec "github.com/arcology-network/common-lib/storage/codec"
	stgintf "github.com/arcology-network/common-lib/storage/interface"
	arcocodec "github.com/arcology-network/state-engine/storage/codec/arcocodec"
)

func testLiveCacheCodec() *stgcodec.StorageCodec[string, crdtcommon.CRDT, string, []byte] {
	return stgcodec.NewStorageCodec(
		func(k string, value crdtcommon.CRDT) (string, []byte, error) {
			encoded, err := arcocodec.Codec{}.Encode(k, value)
			return k, encoded, err
		},
		func(k string, data []byte) (string, crdtcommon.CRDT, error) {
			decoded := arcocodec.Codec{}.Decode(k, data, nil)
			if decoded == nil {
				return k, nil, nil
			}
			return k, decoded.(crdtcommon.CRDT), nil
		},
	)
}

func TestNewLiveCacheWithoutBackend(t *testing.T) {
	store := NewLiveCache(128, nil, testLiveCacheCodec())

	value := noncommutative.NewString("value")
	store.Set("alpha", value)

	read, err := store.Get("alpha")
	if err != nil || read != value {
		t.Fatal("expected local writes to be readable from the cache")
	}

	if ok := store.Has("missing"); ok {
		t.Fatal("expected missing key lookup to stay false in local-only mode")
	}

	if _, err := store.Get("missing"); err != stgintf.ErrNotFound {
		t.Fatal("expected missing key read to return ErrNotFound in local-only mode")
	}
}

func TestNewLiveCacheSizeFunctionHandlesNilAndCRDT(t *testing.T) {
	store := NewLiveCache(128, nil, testLiveCacheCodec())
	sizeOf := store.Cache().Policy().ValueSize

	if got := sizeOf(nil); got != 0 {
		t.Fatalf("expected nil values to report zero size, got %d", got)
	}

	value := noncommutative.NewString("hello")
	if got := sizeOf(value); got != value.MemSize() {
		t.Fatalf("expected MemSize-backed size accounting, got %d want %d", got, value.MemSize())
	}
}

func TestNewLiveCacheTracksCRDTMemSize(t *testing.T) {
	store := NewLiveCache(128, nil, testLiveCacheCodec())
	value := noncommutative.NewString("hello")

	store.Set("alpha", value)
	if got := store.Cache().Policy().Size(); got != value.MemSize() {
		t.Fatalf("expected cache size to match CRDT MemSize, got %d want %d", got, value.MemSize())
	}

	store.Delete("alpha")
	if got := store.Cache().Policy().Size(); got != 0 {
		t.Fatalf("expected deleting the entry to release tracked size, got %d", got)
	}
}

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
