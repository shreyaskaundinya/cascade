package cascade_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/anirudhRowjee/cascade"
)

func TestEncodeDecodeNPE_Upsert(t *testing.T) {
	entry := cascade.KVEntry{Key: "hello", Value: "world"}
	got, err := cascade.DecodeNPE(bytes.NewReader(cascade.EncodeNPE(entry)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != entry {
		t.Fatalf("got %+v, want %+v", got, entry)
	}
}

func TestEncodeDecodeNPE_Tombstone(t *testing.T) {
	entry := cascade.KVEntry{Key: "gone", IsTombstone: true}
	got, err := cascade.DecodeNPE(bytes.NewReader(cascade.EncodeNPE(entry)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != entry {
		t.Fatalf("got %+v, want %+v", got, entry)
	}
}

func TestDecodeNPE_InvalidMagic(t *testing.T) {
	_, err := cascade.DecodeNPE(bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00}))
	if !errors.Is(err, cascade.ErrInvalidNPEMagic) {
		t.Fatalf("expected ErrInvalidNPEMagic, got %v", err)
	}
}

func TestDecodeNPE_EOF(t *testing.T) {
	_, err := cascade.DecodeNPE(bytes.NewReader([]byte{}))
	if err != io.EOF {
		t.Fatalf("expected io.EOF on empty reader, got %v", err)
	}
}

func TestNPEEncodedSize(t *testing.T) {
	cases := []struct {
		entry cascade.KVEntry
		want  int
	}{
		{cascade.KVEntry{Key: "hello", Value: "world"}, 6 + 5 + 5},
		{cascade.KVEntry{Key: "gone", IsTombstone: true}, 6 + 4},
		{cascade.KVEntry{Key: "k", Value: "v"}, 6 + 1 + 1},
	}
	for _, c := range cases {
		if got := cascade.NPEEncodedSize(c.entry); got != c.want {
			t.Fatalf("NPEEncodedSize(%+v) = %d, want %d", c.entry, got, c.want)
		}
	}
}

// TestEncodeDecodeNPE_DiskRoundTrip writes NPE entries to a real file, then reads
// each entry back by seeking to its computed byte offset — verifying the format is
// position-addressable (as the index block will require).
func TestEncodeDecodeNPE_DiskRoundTrip(t *testing.T) {
	entries := []cascade.KVEntry{
		{Key: "disk-key-1", Value: "disk-val-1"},
		{Key: "disk-key-2", IsTombstone: true},
		{Key: "disk-key-3", Value: "disk-val-3"},
	}

	f, err := os.CreateTemp(t.TempDir(), "npe-*.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// Write all entries, tracking the byte offset where each starts.
	offsets := make([]int64, len(entries))
	pos := int64(0)
	for i, e := range entries {
		offsets[i] = pos
		if _, err := f.Write(cascade.EncodeNPE(e)); err != nil {
			t.Fatal(err)
		}
		pos += int64(cascade.NPEEncodedSize(e))
	}

	// Read each entry by seeking to its offset directly (not sequential).
	for i, want := range entries {
		if _, err := f.Seek(offsets[i], io.SeekStart); err != nil {
			t.Fatal(err)
		}
		got, err := cascade.DecodeNPE(f)
		if err != nil {
			t.Fatalf("entry %d: unexpected error: %v", i, err)
		}
		if got != want {
			t.Fatalf("entry %d: got %+v, want %+v", i, got, want)
		}
	}
}
