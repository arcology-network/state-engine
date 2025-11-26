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
	"os"
	"path"
	"testing"

	"github.com/arcology-network/common-lib/codec"
	filedb "github.com/arcology-network/common-lib/storage/filedb"
)

var (
	TEST_ROOT_PATH = path.Join(os.TempDir(), "/filedb/")
)

func TestDatastoreBasic(t *testing.T) {
	fileDB, err := filedb.NewFileDB(TEST_ROOT_PATH, 8, 2)
	if err != nil {
		t.Error(err)
	}

	keys := []string{"123", "456", "789"}
	values := [][]byte{{1, 2, 3}, {4, 5, 6}, {5, 5, 5}}

	encoder := func(k string, v any) ([]byte, error) {
		return codec.Bytes(v.([]byte)).Encode(), nil
	}

	decoder := func(_ string, data []byte, _ any) any {
		return []byte(codec.Bytes("").Decode(data).(codec.Bytes))
	}

	// fileDB.BatchSet(keys, values)

	store := NewLiveStorage(fileDB, encoder, decoder)

	vs := make([]any, len(values))
	for i := 0; i < len(values); i++ {
		vs[i] = values[i]
	}

	if err := store.BatchInject(keys, vs); err != nil {
		t.Error(err)
	}

	v, _ := store.Retrieve(keys[0], nil)
	if string(v.([]byte)) != string(values[0]) {
		t.Error("Error: Values mismatched !")
	}

	v, _ = store.Retrieve(keys[1], nil)
	if string(v.([]byte)) != string(values[1]) {
		t.Error("Error: Values mismatched !")
	}
}

func TestDatastorePersistentStorage(t *testing.T) {
	fileDB, err := filedb.NewFileDB(TEST_ROOT_PATH, 8, 2)
	if err != nil {
		t.Error(err)
	}

	keys := []string{"123", "456"}
	values := [][]byte{{1, 2, 3}, {4, 5, 6}}

	//policy := policy.(1234, 1.0)
	encoder := func(_ string, v any) ([]byte, error) { return codec.Bytes(v.([]byte)).Encode(), nil }
	decoder := func(_ string, data []byte, _ any) any { return codec.Bytes("").Decode(data) }

	// fileDB.BatchSet(keys, values)

	store := NewLiveStorage(fileDB, encoder, decoder)

	vs := make([]any, len(values))
	for i := 0; i < len(values); i++ {
		vs[i] = values[i]
	}

	if err := store.db.BatchSet(keys, values); err != nil {
		t.Error(err)
	}

	if err := store.BatchInject(keys, vs); err != nil {
		t.Error(err)
	}

	v, _ := store.Retrieve(keys[0], nil)
	if string(v.([]byte)) != string(values[0]) {
		t.Error("Error: Values mismatched !")
	}

	v, _ = store.Retrieve(keys[1], nil)
	if string(v.([]byte)) != string(values[1]) {
		t.Error("Error: Values mismatched !")
	}
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
	values := make([][]byte, 4)
	values[0] = []byte{1, 2, 3}
	values[1] = []byte{4, 5, 6}
	values[2] = []byte{6, 7, 8}
	values[3] = []byte{8, 9, 0}

	//policy := policy.(1234, 1.0)
	encoder := func(_ string, v any) ([]byte, error) { return codec.Bytes(v.([]byte)).Encode(), nil }
	decoder := func(_ string, data []byte, _ any) any { return codec.Bytes("").Decode(data) }

	// if err := fileDB.BatchSet(keys, values); err != nil {
	// 	t.Error(err)
	// }

	store := NewLiveStorage(fileDB, encoder, decoder)

	vs := make([]any, len(values))
	for i := 0; i < len(values); i++ {
		vs[i] = values[i]
	}
	store.db.BatchSet(keys, values)
	store.BatchInject(keys, vs)

	v, _ := store.Retrieve(keys[0], nil)

	if string(v.([]byte)) != string(values[0]) {
		t.Error("Error: Values mismatched !")
	}

	v, _ = store.Retrieve(keys[1], nil)
	if string(v.([]byte)) != string(values[1]) {
		t.Error("Error: Values mismatched !")
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
	values := make([][]byte, 4)
	values[0] = []byte{1, 2, 3}
	values[1] = []byte{4, 5, 6}
	values[2] = []byte{6, 7, 8}
	values[3] = []byte{8, 9, 0}

	//policy := policy.(1234, 1.0)
	encoder := func(_ string, v any) ([]byte, error) { return codec.Bytes(v.([]byte)).Encode(), nil }
	decoder := func(_ string, data []byte, _ any) any { return codec.Bytes("").Decode(data) }

	// if err := fileDB.BatchSet(keys, values); err != nil {
	// 	t.Error(err)
	// }

	store := NewLiveStorage(fileDB, encoder, decoder)

	vs := make([]any, len(values))
	for i := 0; i < len(values); i++ {
		vs[i] = values[i]
	}
	store.db.BatchSet(keys, values)
	store.BatchInject(keys, vs)

	v, _ := store.Retrieve(keys[0], nil)

	if string(v.([]byte)) != string(values[0]) {
		t.Error("Error: Values mismatched !")
	}

	v, _ = store.Retrieve(keys[1], nil)
	if string(v.([]byte)) != string(values[1]) {
		t.Error("Error: Values mismatched !")
	}
}
