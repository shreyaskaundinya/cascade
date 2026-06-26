package cascade_test

// Stage 2: SSTable + Flush
// Goal: implement WriteSSTable, SSTable.Get, and SSTable.Scan using the
// block and NPE encoding primitives, then wire them into the engine Flush path.
// Run with: go test -run TestStage2 ./...

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"testing"

	cascade "github.com/anirudhRowjee/cascade"
)

// ---------------------------------------------------------------------------
// SSTable writer / reader
// ---------------------------------------------------------------------------

// TestStage2_WriteSSTable verifies that WriteSSTable produces a non-empty file on disk.
// This is already implemented for you and should pass already
func TestStage2_WriteSSTable(t *testing.T) {
	dir := t.TempDir()
	entries := []cascade.KVEntry{
		cascade.GenerateUpsert("apple", "fruit"),
		cascade.GenerateUpsert("banana", "yellow"),
		cascade.GenerateDelete("cherry"),
	}

	_, err := cascade.WriteSSTable(dir+"/test.sst", entries)
	if err != nil {
		t.Fatalf("WriteSSTable: %v", err)
	}

	info, err := os.Stat(dir + "/test.sst")
	if err != nil {
		t.Fatalf("SSTable file not found: %v", err)
	}
	// A valid SSTable has at least one full block on disk.
	if info.Size() < int64(cascade.BlockSize) {
		t.Fatalf("SSTable file too small: %d bytes (want >= %d)", info.Size(), cascade.BlockSize)
	}
}

func TestStage2_SSTableReaderSanity(t *testing.T) {
	dir := t.TempDir()
	customEntry := cascade.GenerateNumberedUpsert(1, "hello")
	entries := []cascade.KVEntry{customEntry}

	sst, err := cascade.WriteSSTable(dir+"/test.sst", entries)
	if err != nil {
		t.Fatalf("SSTable file not found: %v", err)
	}

	iocnt := cascade.NewIOCounter()
	kve, found, err := sst.Get(customEntry.Key, iocnt)
	if err != nil || found == false {
		t.Errorf("Could not read from SSTable : %s", err)
	}

	if kve != customEntry {
		t.Fatalf("Items written [%s: %s: %t] and read [%s: %s: %t] do not match", customEntry.Key, customEntry.Value, customEntry.IsTombstone, kve.Key, kve.Value, kve.IsTombstone)
	}

}

// TestStage2_SSTable_Get_100Keys writes 100 entries and reads each back by key.
//
// Each cold Get requires at most 3 block reads: header block, index block, data block.
// The test asserts the IO counter stays within that bound.
func TestStage2_SSTable_Get_100Keys(t *testing.T) {
	dir := t.TempDir()
	const n = 100

	entries := make([]cascade.KVEntry, n)
	for i := range n {
		entries[i] = cascade.GenerateNumberedUpsert(int64(i), fmt.Sprintf("val-%d", i))
	}

	sst, err := cascade.WriteSSTable(dir+"/test.sst", entries)
	if err != nil {
		t.Fatalf("WriteSSTable: %v", err)
	}

	counter := cascade.NewIOCounter()
	for i := range n {
		key := fmt.Sprintf("key-%010d", i)
		got, found, err := sst.Get(key, counter)
		if err != nil {
			t.Fatalf("Get(%q): %v", key, err)
		}
		if !found {
			t.Fatalf("Get(%q): key not found", key)
		}
		want := fmt.Sprintf("val-%d", i)
		if got.Value != want {
			t.Fatalf("Get(%q): got %q, want %q", key, got.Value, want)
		}
	}

	if counter.Count() > int64(n)*3 {
		t.Fatalf("too many block reads: got %d, want <= %d (3 per key max)", counter.Count(), n*3)
	}
}

// TestStage2_SSTable_MultiBlock_Get writes 1000 entries (spanning multiple data blocks)
// and verifies Get works correctly across block boundaries.
//
// With a 4 KB block and ~37 bytes per NPE entry (6 header + 15 key + 16 value),
// roughly 110 entries fit per block — so 1000 keys spans ~9 data blocks.
// Each cold Get costs at most 3 block reads (header + index + data block).
func TestStage2_SSTable_MultiBlock_Get(t *testing.T) {
	dir := t.TempDir()
	const n = 1000

	entries := make([]cascade.KVEntry, n)
	for i := range n {
		entries[i] = cascade.GenerateNumberedUpsert(int64(i), fmt.Sprintf("val-%d", i))
	}

	sst, err := cascade.WriteSSTable(dir+"/test.sst", entries)
	if err != nil {
		t.Fatalf("WriteSSTable: %v", err)
	}

	counter := cascade.NewIOCounter()
	for i := range n {
		key := fmt.Sprintf("key-%010d", i)
		got, found, err := sst.Get(key, counter)
		if err != nil {
			t.Fatalf("Get(%q): %v", key, err)
		}
		if !found {
			t.Fatalf("Get(%q): key not found", key)
		}
		want := fmt.Sprintf("val-%d", i)
		if got.Value != want {
			t.Fatalf("Get(%q): got %q, want %q", key, got.Value, want)
		}
	}

	if counter.Count() > int64(n)*3 {
		t.Fatalf("too many block reads: got %d, want <= %d (3 per key max)", counter.Count(), n*3)
	}
}

// TestStage2_SSTable_Scan writes 100 entries (including tombstones) and verifies
// Scan returns all of them in the original sorted key order.
//
// Scan must return tombstones — suppression of deleted keys is the engine's job,
// not the SSTable's.
func TestStage2_SSTable_Scan(t *testing.T) {
	dir := t.TempDir()
	const n = 100

	entries := make([]cascade.KVEntry, n)
	for i := range n {
		entries[i] = cascade.GenerateNumberedUpsert(int64(i), fmt.Sprintf("val-%d", i))
	}
	// Replace two live entries with tombstones; key order is unchanged.
	entries[10] = cascade.GenerateNumberedDelete(10)
	entries[50] = cascade.GenerateNumberedDelete(50)

	sst, err := cascade.WriteSSTable(dir+"/test.sst", entries)
	if err != nil {
		t.Fatalf("WriteSSTable: %v", err)
	}

	counter := cascade.NewIOCounter()
	got, err := sst.Scan(counter)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(got) != n {
		t.Fatalf("Scan: got %d entries, want %d", len(got), n)
	}
	for i, e := range got {
		if e != entries[i] {
			t.Fatalf("Scan entry %d: got %+v, want %+v", i, e, entries[i])
		}
	}
}

// ---------------------------------------------------------------------------
// Engine flush path
// ---------------------------------------------------------------------------

func TestStage2_HomeDirIsCreated(t *testing.T) {
	testPath := "./cascade_data"
	cascade.NewEngine(testPath)

	info, err := os.Stat(testPath)
	if err != nil {
		if !info.IsDir() {
			t.Fatal("File exists but not as directory")
		}
		if errors.Is(err, fs.ErrNotExist) {
			t.Fatal("Directory does not exist")
		}
		if errors.Is(err, fs.ErrPermission) {
			t.Fatal("Permissions not present on directory")
		}
	}

}

// Flushing a non-empty memtable must create at least one SSTable file on disk.
func TestStage2_FlushCreatesSSTableFile(t *testing.T) {
	e := newTestEngine(t)

	_ = e.Upsert("a", []byte("1"))
	_ = e.Upsert("b", []byte("2"))

	if err := e.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// TODO: assert at least one SSTable file exists under the engine's L0 directory.
}

// Each flush must produce a distinct SSTable; n flushes → n L0 files.
func TestStage2_MultipleFlushesProduceMultipleSSTables(t *testing.T) {
	e := newTestEngine(t)

	batches := []struct{ key, val string }{
		{"a", "1"},
		{"b", "2"},
		{"c", "3"},
	}
	for _, b := range batches {
		_ = e.Upsert(b.key, []byte(b.val))
		if err := e.Flush(); err != nil {
			t.Fatalf("Flush: %v", err)
		}
	}

	// TODO: assert the L0 SSTable count equals len(batches).
}

// A tombstone written before a flush must suppress the key on subsequent reads.
func TestStage2_TombstonePersistedToSSTable(t *testing.T) {
	e := newTestEngine(t)

	_ = e.Upsert("gone", []byte("here"))
	_ = e.Delete("gone")
	if err := e.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	_, err := e.Get("gone")
	if err != cascade.ErrNotFound {
		t.Fatalf("want ErrNotFound for deleted key after flush, got %v", err)
	}
}

// When the same key appears in multiple L0 SSTables, the newest value must win.
// This validates that the read path scans L0 from most-recently-written to oldest.
func TestStage2_ReadSearchesNewestFirst(t *testing.T) {
	e := newTestEngine(t)

	_ = e.Upsert("k", []byte("old"))
	_ = e.Flush()

	_ = e.Upsert("k", []byte("new"))
	_ = e.Flush()

	got, err := e.Get("k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "new" {
		t.Fatalf("want %q (newest value), got %q", "new", got)
	}
}

// A key written to the memtable and then flushed must be readable afterward.
func TestStage2_GetAfterFlush(t *testing.T) {
	e := newTestEngine(t)

	_ = e.Upsert("hello", []byte("world"))
	if err := e.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	got, err := e.Get("hello")
	if err != nil {
		t.Fatalf("Get after flush: %v", err)
	}
	if string(got) != "world" {
		t.Fatalf("want %q, got %q", "world", got)
	}
}
