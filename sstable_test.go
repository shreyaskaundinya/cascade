package cascade

import (
	"os"
	"testing"
)

func TestSSTableWriter(t *testing.T) {
	entries := []KVEntry{
		GenerateUpsert("hello", "world"),
		GenerateUpsert("hello1", "world2"),
		GenerateUpsert("hello2", "world2"),
	}
	_, err := WriteSSTable(1, t.TempDir()+"/test.sst", entries)
	if err != nil {
		t.Fatalf("WriteSSTable: %v", err)
	}
}

func TestParseHeaderBlock(t *testing.T) {
	entries := []KVEntry{
		GenerateUpsert("apple", "1"),
		GenerateUpsert("mango", "2"),
		GenerateUpsert("zebra", "3"),
	}
	path := t.TempDir() + "/test.sst"
	if _, err := WriteSSTable(42, path, entries); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	b, err := ReadBlock(f, nil)
	if err != nil {
		t.Fatalf("ReadBlock: %v", err)
	}

	hdr, err := ParseHeaderBlock(b)
	if err != nil {
		t.Fatalf("ParseHeaderBlock: %v", err)
	}

	if hdr.LowKey != "apple" {
		t.Errorf("LowKey: got %q, want %q", hdr.LowKey, "apple")
	}
	if hdr.HighKey != "zebra" {
		t.Errorf("HighKey: got %q, want %q", hdr.HighKey, "zebra")
	}
	if hdr.ItemCount != uint64(len(entries)) {
		t.Errorf("ItemCount: got %d, want %d", hdr.ItemCount, len(entries))
	}
	if hdr.BlockCount != 1 {
		t.Errorf("BlockCount: got %d, want 1", hdr.BlockCount)
	}
}

func TestParseIndexBlock(t *testing.T) {
	entries := []KVEntry{
		GenerateUpsert("apple", "1"),
		GenerateUpsert("mango", "2"),
		GenerateUpsert("zebra", "3"),
	}
	path := t.TempDir() + "/test.sst"
	if _, err := WriteSSTable(1, path, entries); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// skip header block
	if _, err := ReadBlock(f, nil); err != nil {
		t.Fatalf("reading header block: %v", err)
	}

	idxBlock, err := ReadBlock(f, nil)
	if err != nil {
		t.Fatalf("ReadBlock (index): %v", err)
	}

	indexEntries, err := ParseIndexBlock(idxBlock)
	if err != nil {
		t.Fatalf("ParseIndexBlock: %v", err)
	}

	// 3 entries fit in one data block, so there should be exactly one index entry
	if len(indexEntries) != 1 {
		t.Fatalf("got %d index entries, want 1", len(indexEntries))
	}
	if indexEntries[0].DataBlockNum != 0 {
		t.Errorf("DataBlockNum: got %d, want 0", indexEntries[0].DataBlockNum)
	}
	if indexEntries[0].HighKey != "zebra" {
		t.Errorf("HighKey: got %q, want %q", indexEntries[0].HighKey, "zebra")
	}
}
