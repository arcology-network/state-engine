package cache

import (
	"strings"
	"testing"

	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
	"github.com/arcology-network/common-lib/crdt/commutative"
	noncommutative "github.com/arcology-network/common-lib/crdt/noncommutative"
	statecell "github.com/arcology-network/common-lib/crdt/statecell"
	"github.com/arcology-network/common-lib/exp/associative"
	storageintf "github.com/arcology-network/common-lib/storage/interface"
	statecommon "github.com/arcology-network/state-engine/common"
)

type stubWriter struct {
	name string
}

func (this *stubWriter) Import([]*statecell.StateCell) {}
func (this *stubWriter) Precommit(bool) error          { return nil }
func (this *stubWriter) Commit(uint64) error           { return nil }
func (this *stubWriter) IsSync() bool                  { return false }
func (this *stubWriter) Name() string                  { return this.name }

type stubReadWriteStore struct {
	values     map[string]crdtcommon.CRDT
	writers    []storageintf.StoreWriter[*statecell.StateCell]
	getCalls   int
	preloadArg []byte
	version    [32]byte
}

var testAccount = statecommon.ETH_ACCOUNT_PREFIX + "0x" + strings.Repeat("1", 40)

func testPath(suffix string) string {
	if strings.HasPrefix(suffix, "/") {
		return testAccount + suffix
	}
	return testAccount + "/" + suffix
}

func newStubReadWriteStore() *stubReadWriteStore {
	return &stubReadWriteStore{
		values:  map[string]crdtcommon.CRDT{},
		writers: []storageintf.StoreWriter[*statecell.StateCell]{},
	}
}

func (this *stubReadWriteStore) Has(key string) bool {
	_, ok := this.values[key]
	return ok
}

func (this *stubReadWriteStore) Get(key string) (any, error) {
	this.getCalls++
	return this.values[key], nil
}

// func (this *stubReadWriteStore) ReadBackend(key string, _ any) (any, error) {
// 	return this.values[key], nil
// }

func (this *stubReadWriteStore) Preload(data []byte) any {
	this.preloadArg = append([]byte(nil), data...)
	return len(data)
}

func (this *stubReadWriteStore) GetWriters() []storageintf.StoreWriter[*statecell.StateCell] {
	return this.writers
}

func (this *stubReadWriteStore) SetVersion(version [32]byte) error {
	this.version = version
	return nil
}

func TestNewExecutionStateStoreInitializesDefaults(t *testing.T) {
	backend := newStubReadWriteStore()
	store := NewExecutionStateStore(backend, 8, 2)

	if store == nil {
		t.Fatal("expected store instance")
	}
	if got, ok := store.CommittedStore().(*stubReadWriteStore); !ok || got != backend {
		t.Fatal("expected readonly backend to be preserved")
	}
	if store.GetID() != 0 {
		t.Fatalf("expected default id 0, got %d", store.GetID())
	}
	if store.GetVersion() != statecommon.LATEST_STATE_VERSION {
		t.Fatalf("expected latest state version %d, got %d", statecommon.LATEST_STATE_VERSION, store.GetVersion())
	}
	if store.pool == nil {
		t.Fatal("expected mempool to be initialized")
	}
	if len(store.localCells) != 0 {
		t.Fatalf("expected empty local cache, got %d entries", len(store.localCells))
	}
	if len(store.pendingWildcardDeletes) != 0 {
		t.Fatalf("expected no pending wildcard deletes, got %d", len(store.pendingWildcardDeletes))
	}

	store.SetID(42)
	store.SetVersion(99)
	store.SetCommittedStore(nil)

	if store.GetID() != 42 {
		t.Fatalf("expected updated id 42, got %d", store.GetID())
	}
	if store.GetVersion() != 99 {
		t.Fatalf("expected updated version 99, got %d", store.GetVersion())
	}
	if store.CommittedStore() != nil {
		t.Fatal("expected readonly backend to be replaced")
	}
}

func TestExecutionStateStoreLoadFromCommittedUsesCommittedStoreGet(t *testing.T) {
	backend := newStubReadWriteStore()
	expected := noncommutative.NewString("alpha")
	backend.values["/acct/alpha"] = expected

	store := NewExecutionStateStore(backend, 8, 1)
	cell := store.loadFromCommitted(7, "/acct/alpha", noncommutative.NewString(""))

	if backend.getCalls != 1 {
		t.Fatalf("expected exactly one Get call, got %d", backend.getCalls)
	}
	if cell == nil || cell.Value() == nil {
		t.Fatal("expected committed load to populate a state cell value")
	}
	if !cell.Value().(crdtcommon.CRDT).Equal(expected) {
		t.Fatal("expected committed load value to match backend value")
	}
	if _, ok := store.localCells["/acct/alpha"]; ok {
		t.Fatal("expected committed load to avoid mutating local cache")
	}
}

func TestExecutionStateStoreGetReturnsClone(t *testing.T) {
	store := NewExecutionStateStore(nil, 8, 1)
	original := commutative.NewUint64Delta(5)

	if _, err := store.Inject(statecommon.SYSTEM, "/acct/value", original); err != nil {
		t.Fatalf("inject failed: %v", err)
	}

	raw, err := store.Get("/acct/value")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	cloned, ok := raw.(*commutative.Uint64)
	if !ok {
		t.Fatalf("expected cloned uint64 CRDT, got %T", raw)
	}
	if cloned == original {
		t.Fatal("expected Get to return a clone, not the original pointer")
	}
	if value, _, _ := cloned.Get(); value != uint64(5) {
		t.Fatalf("expected cloned value 5, got %v", value)
	}

	cloned.SetDelta(uint64(9), true)
	if value, _, _ := original.Get(); value != uint64(5) {
		t.Fatal("expected mutating returned clone not to affect cached value")
	}
}

func TestExecutionStateStoreGetWritersIncludesBackendWriters(t *testing.T) {
	backend := newStubReadWriteStore()
	backend.writers = []storageintf.StoreWriter[*statecell.StateCell]{&stubWriter{name: "backend-writer"}}

	store := NewDefaultExecutionStateStore(backend)
	writers := store.GetWriters()

	if len(writers) != 2 {
		t.Fatalf("expected self writer plus backend writer, got %d writers", len(writers))
	}
	if _, ok := writers[0].(*ExecutionCacheWriter); !ok {
		t.Fatalf("expected first writer to be ExecutionCacheWriter, got %T", writers[0])
	}
	if writers[1].Name() != "backend-writer" {
		t.Fatalf("expected backend writer to be preserved, got %q", writers[1].Name())
	}
}

func TestExecutionStateStoreBackendWrappers(t *testing.T) {
	backend := newStubReadWriteStore()
	backend.values[testPath("/code")] = noncommutative.NewString("code")

	store := NewDefaultExecutionStateStore(backend)
	if store.CommittedStore() != backend {
		t.Fatal("expected CommittedStore accessor to return readonly backend")
	}

	value, err := store.CommittedStore().Get(testPath("/code"))
	if err != nil {
		t.Fatalf("expected committed store read to succeed: %v", err)
	}
	if !value.(crdtcommon.CRDT).Equal(noncommutative.NewString("code")) {
		t.Fatal("expected backend value to round-trip")
	}

	nilStore := NewExecutionStateStore(nil, 4, 1)
	if nilStore.CommittedStore() != nil {
		t.Fatal("expected nil store to expose nil committed store")
	}
}

func TestExecutionStateStoreWriteReadAndCacheHelpers(t *testing.T) {
	store := NewExecutionStateStore(nil, 8, 1)
	path := testPath(statecommon.PATH_BALANCE)
	value := noncommutative.NewString("hello")

	if got := store.Cache(); got != &store.localCells {
		t.Fatal("expected Cache accessor to expose local cell map")
	}
	if !store.ExistsInParent(1, testPath("/balance"), nil) {
		t.Fatal("expected immediate child of system path to exist in parent")
	}

	before := store.DiffSize(statecommon.SYSTEM, path, value)
	if before != int64(value.MemSize()) {
		t.Fatalf("expected diff size to equal new value size before write, got %d", before)
	}

	callbackCalled := false
	sizeDiff, err := store.Write(statecommon.SYSTEM, path, value, func(cell *statecell.StateCell, diff int64) {
		callbackCalled = true
		if cell == nil || cell.Value() == nil {
			t.Fatal("expected callback cell to be populated")
		}
		if diff != 0 {
			t.Fatalf("expected callback diff 0 after write accounting, got %d", diff)
		}
	})
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if !callbackCalled {
		t.Fatal("expected write callback to be invoked")
	}
	if sizeDiff != 0 {
		t.Fatalf("expected write size diff 0 after write accounting, got %d", sizeDiff)
	}
	if !store.Has(path) {
		t.Fatal("expected written path to exist")
	}
	if cached, ok := store.GetIfCached(path); !ok || cached == nil {
		t.Fatal("expected written path to be cached")
	}

	readValue, cellAny, readSize := store.Read(statecommon.SYSTEM, path, noncommutative.NewString(""))
	cell, ok := cellAny.(*statecell.StateCell)
	if !ok || cell == nil || cell.Value() == nil {
		t.Fatal("expected read to return a populated state cell")
	}
	_ = readValue
	if readSize != statecommon.MIN_READ_SIZE+value.MemSize() {
		t.Fatalf("expected read size %d, got %d", statecommon.MIN_READ_SIZE+value.MemSize(), readSize)
	}

	raw, err := store.Get(path)
	if err != nil || raw == nil {
		t.Fatalf("expected get to succeed, got value=%v err=%v", raw, err)
	}

	post := store.DiffSize(statecommon.SYSTEM, path, value)
	if post != 0 {
		t.Fatalf("expected diff size 0 for unchanged value, got %d", post)
	}

	missingValue, missingCell, found := store.LookupForRead(1, testPath("/missing/deep/value"), noncommutative.NewString(""), nil)
	if found || missingValue != nil || missingCell == nil {
		t.Fatal("expected missing lookup to return empty non-cached cell")
	}
	if _, err := store.Write(statecommon.SYSTEM, path, nil); err != nil {
		t.Fatalf("expected delete write to succeed: %v", err)
	}
	if store.Has(path) {
		t.Fatal("expected deleted path to be absent")
	}
	if got := store.DiffSize(statecommon.SYSTEM, path, nil); got != 0 {
		t.Fatalf("expected nil diff after deletion, got %d", got)
	}
	bad := &stubInvalidCRDT{}
	if _, err := store.Write(statecommon.SYSTEM, path, bad); err == nil {
		t.Fatal("expected invalid CRDT type write to fail")
	}
	if store.Has(testPath("/not-present")) {
		t.Fatal("expected absent path to be false")
	}

	backend := newStubReadWriteStore()
	backend.values[testPath("/remote/value")] = noncommutative.NewString("remote")
	remoteStore := NewExecutionStateStore(backend, 8, 1)
	if !remoteStore.Has(testPath("/remote/value")) {
		t.Fatal("expected backend-backed path to be reported as existing")
	}
	v, size := remoteStore.ReadCommitted(7, testPath("/remote/value"), noncommutative.NewString(""))
	if v == nil || size != statecommon.MIN_READ_SIZE+uint64(len("remote")) {
		t.Fatalf("expected committed read value and size, got value=%v size=%d", v, size)
	}

	errStore := NewExecutionStateStore(nil, 4, 1)
	if cell := errStore.loadFromCommitted(1, testPath("/err"), noncommutative.NewString("")); cell == nil {
		t.Fatal("expected load from committed to still return a state cell when committed store is nil")
	}
}

func TestExecutionStateStoreInsertExportAndResetHelpers(t *testing.T) {
	store := NewExecutionStateStore(nil, 8, 1)
	pathA := testPath("/custom/a")
	pathB := testPath("/custom/b")
	pathMeta := testPath("/container/")

	cellA, err := store.Inject(statecommon.SYSTEM, pathA, noncommutative.NewString("alpha"))
	if err != nil {
		t.Fatalf("inject A failed: %v", err)
	}
	cellB, err := store.Inject(statecommon.SYSTEM, pathB, noncommutative.NewString("beta"))
	if err != nil {
		t.Fatalf("inject B failed: %v", err)
	}
	pathCell := statecell.NewStateCell(statecommon.SYSTEM, pathMeta, 0, 1, 0, commutative.NewPath("child"), nil)

	if store.set(nil) != store {
		t.Fatal("expected set(nil) to be a no-op returning the receiver")
	}
	committedPath := statecell.NewStateCell(statecommon.SYSTEM, pathMeta, 0, 1, 0, commutative.NewPath("child"), nil)
	committedPath.Init(statecommon.SYSTEM, pathMeta, 0, 1, 0, committedPath.Value(), true)
	store.set(committedPath)
	if _, ok := store.localCells[pathMeta]; ok {
		t.Fatal("expected committed path cell to be ignored by set")
	}

	exported := store.Export(statecell.Sorter)
	if len(exported) != 2 {
		t.Fatalf("expected 2 exported transitions, got %d", len(exported))
	}
	accesses, transitions := store.ExportAll(statecell.Sorter)
	if len(accesses) != len(transitions) || len(transitions) != 2 {
		t.Fatalf("expected export all to preserve two transitions, got accesses=%d transitions=%d", len(accesses), len(transitions))
	}
	keys, values := store.KVs()
	if len(keys) != 2 || len(values) != 2 {
		t.Fatalf("expected two kv pairs, got %d keys and %d values", len(keys), len(values))
	}
	if keys[0] != pathA || keys[1] != pathB {
		t.Fatalf("expected sorted keys [%s %s], got %v", pathA, pathB, keys)
	}

	checksum := store.Checksum()
	other := NewExecutionStateStore(nil, 8, 1)
	if other.Insert(nil) != other {
		t.Fatal("expected insert(nil) to be a no-op")
	}
	other.Insert([]*statecell.StateCell{cellB, cellA})
	if !store.Equal(other) {
		t.Fatal("expected inserted store to equal original store")
	}
	if checksum != other.Checksum() {
		t.Fatal("expected equal stores to share checksum")
	}

	pathOnly := NewExecutionStateStore(nil, 8, 1)
	pathOnly.Insert([]*statecell.StateCell{pathCell})
	if _, ok := pathOnly.localCells[pathMeta]; !ok {
		t.Fatal("expected path creation transition to be inserted into cache")
	}

	other.Clear()
	if len(other.localCells) != 0 {
		t.Fatal("expected clear to empty local cache")
	}
	if store.Equal(other) {
		t.Fatal("expected cleared store not to equal original store")
	}
	store.Print()
	if got := store.GetWriters(); len(got) != 1 {
		t.Fatalf("expected only self writer without backend, got %d writers", len(got))
	}
	if got, ok := store.GetIfCached(pathA); !ok || got == nil {
		t.Fatal("expected cached cell for inserted key")
	}
	if _, ok := other.GetIfCached(pathA); ok {
		t.Fatal("expected cleared store to have no cached entry")
	}
}

func TestExecutionStateStoreWildcardMaterializationAndParentChecks(t *testing.T) {
	backend := newStubReadWriteStore()
	containerPath := testPath("/container/")
	childPath := testPath("/container/item")

	pathMeta := commutative.NewPath().(*commutative.Path)
	pathMeta.Insert("item")
	store := NewExecutionStateStore(backend, 8, 1)
	store.localCells[containerPath] = statecell.NewStateCell(statecommon.SYSTEM, containerPath, 0, 1, 0, pathMeta, nil)
	backend.values[childPath] = noncommutative.NewString("backend-value")

	if !store.ExistsInParent(1, childPath, nil) {
		t.Fatal("expected child to exist via parent path metadata")
	}
	if store.ExistsInParent(1, testPath("/container/missing"), nil) {
		t.Fatal("expected missing child key not to exist in parent metadata")
	}

	store.pendingWildcardDeletes = append(store.pendingWildcardDeletes, &associative.Pair[uint64, string]{
		First:  2,
		Second: containerPath,
	})

	v, cell, inCache := store.LookupOrMaterialize(2, childPath, noncommutative.NewString(""), nil, true)
	if v != nil {
		t.Fatal("expected wildcard materialization to return nil value (deleted)")
	}
	if cell == nil {
		t.Fatal("expected wildcard materialization to return a state cell")
	}
	if inCache {
		t.Fatal("expected wildcard branch to report inCache=false by contract")
	}
	if cachedCell, ok := store.localCells[childPath]; !ok || cachedCell == nil {
		t.Fatal("expected wildcard path to be inserted into local cache")
	}

	if exported := store.Export(statecell.Sorter); len(exported) == 0 {
		t.Fatal("expected wildcard export to include at least one transition")
	}
}

func TestExecutionStateStoreAdditionalBranches(t *testing.T) {
	store := NewExecutionStateStore(nil, 8, 1)

	if !store.Has("short") {
		t.Fatal("expected short paths to be treated as system paths")
	}

	leafPath := testPath("/custom/leaf")
	if _, err := store.Write(1, leafPath, noncommutative.NewString("x")); err == nil {
		t.Fatal("expected non-system write without parent to fail")
	}

	pathCell := statecell.NewStateCell(statecommon.SYSTEM, testPath("/custom/copied"), 0, 1, 0, noncommutative.NewString("copied"), nil)
	store.set(pathCell)
	if got, ok := store.localCells[testPath("/custom/copied")]; !ok || got == nil {
		t.Fatal("expected set to copy non-path state cell into cache")
	}

	emptyPath := commutative.NewPath().(*commutative.Path)
	pathValuePath := testPath(statecommon.PATH_BALANCE)
	if _, err := store.Inject(statecommon.SYSTEM, pathValuePath, emptyPath); err != nil {
		t.Fatalf("inject path meta failed: %v", err)
	}
	raw, err := store.Get(pathValuePath)
	if err != nil {
		t.Fatalf("get path meta failed: %v", err)
	}
	clonedPath, ok := raw.(*commutative.Path)
	if !ok {
		t.Fatalf("expected path clone type, got %T", raw)
	}
	if clonedPath == emptyPath {
		t.Fatal("expected Get path branch to return a clone")
	}
}

type stubInvalidCRDT struct{}

func (*stubInvalidCRDT) TypeID() uint8                      { return 0 }
func (*stubInvalidCRDT) Equal(any) bool                     { return false }
func (*stubInvalidCRDT) Clone() any                         { return &stubInvalidCRDT{} }
func (*stubInvalidCRDT) IsNumeric() bool                    { return false }
func (*stubInvalidCRDT) IsCommutative() bool                { return false }
func (*stubInvalidCRDT) Value() any                         { return nil }
func (*stubInvalidCRDT) Delta() (any, bool)                 { return nil, false }
func (*stubInvalidCRDT) Limits() (any, any)                 { return nil, nil }
func (*stubInvalidCRDT) IsDeltaApplied() bool               { return true }
func (*stubInvalidCRDT) New(any, any, any, any, any) any    { return &stubInvalidCRDT{} }
func (*stubInvalidCRDT) CloneDelta() (any, bool)            { return nil, false }
func (*stubInvalidCRDT) SetDelta(any, bool)                 {}
func (*stubInvalidCRDT) SetValue(any)                       {}
func (*stubInvalidCRDT) GetCascadeSub(string, any) []string { return nil }
func (*stubInvalidCRDT) Get() (any, uint32, uint32)         { return nil, 0, 0 }
func (*stubInvalidCRDT) Set(any, any) (any, uint32, uint32, uint32, error) {
	return nil, 0, 0, 0, nil
}
func (*stubInvalidCRDT) CopyTo(any) (any, uint32, uint32, uint32) { return nil, 0, 0, 0 }
func (*stubInvalidCRDT) ApplyDelta([]crdtcommon.CRDT) (crdtcommon.CRDT, int, error) {
	return nil, 0, nil
}
func (*stubInvalidCRDT) IsDeletable(any, any) bool { return false }
func (*stubInvalidCRDT) MemSize() uint64           { return 0 }
func (*stubInvalidCRDT) Size() uint64              { return 0 }
func (*stubInvalidCRDT) Encode() []byte            { return nil }
func (*stubInvalidCRDT) EncodeTo([]byte) int       { return 0 }
func (*stubInvalidCRDT) Decode([]byte) any         { return &stubInvalidCRDT{} }
func (*stubInvalidCRDT) Preload(string, any)       {}
func (*stubInvalidCRDT) Hash() [32]byte            { return [32]byte{} }
func (*stubInvalidCRDT) ShortHash() (uint64, bool) { return 0, false }
func (*stubInvalidCRDT) Print()                    {}
