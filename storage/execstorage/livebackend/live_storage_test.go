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

package ccstorage

import (
	"errors"
	"os"
	"path"
	"testing"

	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
	noncommutative "github.com/arcology-network/common-lib/crdt/noncommutative"
	cachedstore "github.com/arcology-network/common-lib/storage/cachedstore"
	stgcodec "github.com/arcology-network/common-lib/storage/codec"
	filedb "github.com/arcology-network/common-lib/storage/filedb"
	commonintf "github.com/arcology-network/common-lib/storage/interface"
	stgintf "github.com/arcology-network/common-lib/storage/interface"
	arcocodec "github.com/arcology-network/state-engine/storage/codec/arcocodec"
)

var (
	TEST_ROOT_PATH = path.Join(os.TempDir(), "/filedb/")
)

func newTestStore(db commonintf.BackendStore[string, []byte]) *cachedstore.CachedStore[string, crdtcommon.CRDT, string, []byte] {
	codec := stgcodec.NewStorageCodec(
		func(key string, value crdtcommon.CRDT) (string, []byte, error) {
			encoded, err := arcocodec.Codec{}.Encode(key, value)
			return key, encoded, err
		},
		func(key string, data []byte) (string, crdtcommon.CRDT, error) {
			decoded := arcocodec.Codec{}.Decode(key, data, nil)
			if decoded == nil {
				return key, nil, errors.New("failed to decode data")
			}
			return key, decoded.(crdtcommon.CRDT), nil
		},
	)

	return NewLiveStorage(
		func(v crdtcommon.CRDT) uint64 { return v.MemSize() },
		db,
		codec,
	)
}

func newStringValues(values ...string) []crdtcommon.CRDT {
	out := make([]crdtcommon.CRDT, len(values))
	for i, value := range values {
		out[i] = noncommutative.NewString(value)
	}
	return out
}

func encodeValues(t *testing.T, keys []string, values []crdtcommon.CRDT) [][]byte {
	t.Helper()

	encoded := make([][]byte, len(keys))
	for i := range keys {
		var err error
		encoded[i], err = arcocodec.Codec{}.Encode(keys[i], values[i])
		if err != nil {
			t.Fatalf("encode %s: %v", keys[i], err)
		}
	}
	return encoded
}

func requireNoBatchErrors(t *testing.T, errs []error) {
	t.Helper()

	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func requireEqualCRDT(t *testing.T, got any, want crdtcommon.CRDT) {
	t.Helper()

	typed, ok := got.(crdtcommon.CRDT)
	if !ok {
		t.Fatalf("expected crdtcommon.CRDT, got %T", got)
	}
	if !typed.Equal(want) {
		t.Fatalf("unexpected value: got %v want %v", typed.Value(), want.Value())
	}
}

func TestDatastoreBasic(t *testing.T) {
	fileDB, err := filedb.NewFileDB(TEST_ROOT_PATH, 8, 2)
	if err != nil {
		t.Error(err)
	}

	keys := []string{"123", "456", "789"}
	values := newStringValues("alpha", "beta", "gamma")
	store := newTestStore(fileDB)

	requireNoBatchErrors(t, store.SetBatch(keys, values))

	read, err := store.Get(keys[0])
	if err != nil {
		t.Fatal(err)
	}
	requireEqualCRDT(t, read, values[0])

	read, err = store.Get(keys[1])
	if err != nil {
		t.Fatal(err)
	}
	requireEqualCRDT(t, read, values[1])
}

func TestDatastorePersistentStorage(t *testing.T) {
	fileDB, err := filedb.NewFileDB(TEST_ROOT_PATH, 8, 2)
	if err != nil {
		t.Error(err)
	}

	keys := []string{"123", "456"}
	values := newStringValues("persist-a", "persist-b")
	requireNoBatchErrors(t, fileDB.SetBatch(keys, encodeValues(t, keys, values)))

	store := newTestStore(fileDB)

	read, err := store.Get(keys[0])
	if err != nil {
		t.Fatal(err)
	}
	requireEqualCRDT(t, read, values[0])

	read, err = store.Get(keys[1])
	if err != nil {
		t.Fatal(err)
	}
	requireEqualCRDT(t, read, values[1])
}

func TestDatastorePrefetch(t *testing.T) {
	fileDB, err := filedb.NewFileDB(TEST_ROOT_PATH, 8, 2)
	if err != nil {
		t.Error(err)
	}

	keys := []string{
		"blcc:/eth1.0/account/0x12345/abc",
		"blcc:/eth1.0/account/0x98765/bcd",
		"blcc:/eth1.0/account/0x12345/efg",
		"blcc:/eth1.0/account/0x98765/hyq"}
	values := newStringValues("one", "two", "three", "four")
	requireNoBatchErrors(t, fileDB.SetBatch(keys, encodeValues(t, keys, values)))

	store := newTestStore(fileDB)
	reads, errs := store.GetBatch(keys)
	requireNoBatchErrors(t, errs)

	for i := range keys {
		requireEqualCRDT(t, reads[i], values[i])
	}
}

func TestAsyncCommitter(t *testing.T) {
	fileDB, err := filedb.NewFileDB(TEST_ROOT_PATH, 8, 2)
	if err != nil {
		t.Error(err)
	}

	keys := []string{
		"blcc:/eth1.0/account/0x12345/abc",
		"blcc:/eth1.0/account/0x98765/bcd",
		"blcc:/eth1.0/account/0x12345/efg",
		"blcc:/eth1.0/account/0x98765/hyq"}
	values := newStringValues("one", "two", "three", "four")
	store := newTestStore(fileDB)
	requireNoBatchErrors(t, store.SetBatch(keys, values))

	freshStore := newTestStore(fileDB)
	read, err := freshStore.Get(keys[0])
	if err != nil {
		t.Fatal(err)
	}
	requireEqualCRDT(t, read, values[0])

	requireNoBatchErrors(t, store.DeleteBatch([]string{keys[0], keys[1]}))
	freshStore = newTestStore(fileDB)
	if _, err := freshStore.Get(keys[0]); !errors.Is(err, stgintf.ErrNotFound) {
		t.Fatalf("expected deleted key to be missing, got %v", err)
	}
	read, err = freshStore.Get(keys[2])
	if err != nil {
		t.Fatal(err)
	}
	requireEqualCRDT(t, read, values[2])
}

func TestCRDTCodecRoundTrip(t *testing.T) {
	store := newTestStore(nil)
	value := noncommutative.NewString("codec-value")

	key, encoded, err := store.Codec().ForwardConvert("k", value)
	if err != nil {
		t.Fatal(err)
	}
	if key != "k" {
		t.Fatalf("unexpected key: %s", key)
	}

	decodedKey, decodedValue, err := store.Codec().BackwardConvert(key, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decodedKey != key {
		t.Fatalf("unexpected decoded key: %s", decodedKey)
	}
	if !decodedValue.Equal(value) {
		t.Fatalf("decoded value mismatch: got %v want %v", decodedValue.Value(), value.Value())
	}
}
