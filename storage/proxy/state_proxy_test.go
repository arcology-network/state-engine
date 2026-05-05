/*
 *   Copyright (c) 2026 Arcology Network

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

package proxy

import (
	"errors"
	"testing"

	"github.com/arcology-network/common-lib/crdt/commutative"
	stgintf "github.com/arcology-network/common-lib/storage/interface"
	"github.com/arcology-network/common-lib/storage/memdb"
)

func newUint64(v uint64) *commutative.Uint64 {
	u := commutative.NewUnboundedUint64().(*commutative.Uint64)
	u.SetValue(v)
	return u
}

func TestStorageProxyConstructors(t *testing.T) {
	mem := NewMemDBStoreProxy()
	if mem == nil {
		t.Fatal("NewMemDBStoreProxy returned nil")
	}
	if mem.Backend() == nil || mem.Cache() == nil || mem.EthStore() == nil {
		t.Fatal("expected all stores to be initialized")
	}

	cacheOnly := NewCacheOnlyStoreProxy()
	if cacheOnly == nil {
		t.Fatal("NewCacheOnlyStoreProxy returned nil")
	}

	if cacheOnly.Backend() != nil || cacheOnly.Cache() == nil {
		t.Fatal("expected cache-only proxy components to be initialized")
	}

	if got := len(mem.GetWriters()); got != 3 {
		t.Fatalf("expected 3 writers, got %d", got)
	}
	if got := len(mem.SyncWriters()); got != 1 {
		t.Fatalf("expected 1 sync writer, got %d", got)
	}
	if got := len(mem.AsyncWriters()); got != 2 {
		t.Fatalf("expected 2 async writers, got %d", got)
	}
}

func TestStorageProxySetGetHasAndReadBackend(t *testing.T) {
	proxy := NewMemDBStoreProxy()
	key := "/acct/balance"
	want := newUint64(77)

	if err := proxy.Set(key, want); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	if !proxy.Has(key) {
		t.Fatal("expected Has to return true for existing key")
	}

	got, err := proxy.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected Get to return non-nil value")
	}
	gotTyped, ok := got.(*commutative.Uint64)
	if !ok {
		t.Fatalf("expected cached value type *commutative.Uint64, got %T", got)
	}
	v, _, _ := gotTyped.Get()
	if v.(uint64) != 77 {
		t.Fatalf("expected cached value 77, got %v", v)
	}

	proxy.DebugClearexecStore()

	gotRaw, err := proxy.Get(key)
	if err != nil {
		t.Fatalf("Get (backend) failed: %v", err)
	}
	gotTyped, ok = gotRaw.(*commutative.Uint64)
	if !ok {
		t.Fatalf("expected decoded backend value type *commutative.Uint64, got %T", gotRaw)
	}
	v, _, _ = gotTyped.Get()
	if v.(uint64) != 77 {
		t.Fatalf("expected decoded value 77, got %v", v)
	}
}

func TestStorageProxyHasAndGetMissingKey(t *testing.T) {
	proxy := NewMemDBStoreProxy()
	key := "/missing/key"

	if proxy.Has(key) {
		t.Fatal("expected Has to return false for missing key")
	}

	v, err := proxy.Get(key)
	if !errors.Is(err, stgintf.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing key, got %v", err)
	}
	if v != nil {
		t.Fatalf("expected nil value for missing key, got %T", v)
	}

	proxy.DebugClearexecStore()

	v, err = proxy.Get(key)
	if !errors.Is(err, stgintf.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing key after cache clear, got %v", err)
	}
	if v != nil {
		t.Fatalf("expected nil value for missing key after cache clear, got %T", v)
	}
}

func TestInitLiveStorageRoundTripsThroughBackend(t *testing.T) {
	store := initLiveStorage(memdb.NewMemoryDB())
	key := "/acct/counter"
	want := newUint64(88)

	if err := store.Set(key, want); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	store.Cache().Clear()

	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get after cache clear failed: %v", err)
	}

	gotTyped, ok := got.(*commutative.Uint64)
	if !ok {
		t.Fatalf("expected backend-decoded value type *commutative.Uint64, got %T", got)
	}

	v, _, _ := gotTyped.Get()
	if v.(uint64) != 88 {
		t.Fatalf("expected backend-decoded value 88, got %v", v)
	}
}
