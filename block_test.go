package cascade_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/anirudhRowjee/cascade"
)

func TestBlock_AppendAndPayload(t *testing.T) {
	b := cascade.NewBlock(cascade.BlockTypeData)
	data := []byte("hello world")
	if err := b.Append(data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(b.Payload(), data) {
		t.Fatalf("got payload %q, want %q", b.Payload(), data)
	}
}

func TestBlock_Remaining(t *testing.T) {
	b := cascade.NewBlock(cascade.BlockTypeData)
	if b.Remaining() != cascade.BlockPayloadSize {
		t.Fatalf("expected full remaining %d, got %d", cascade.BlockPayloadSize, b.Remaining())
	}
	data := make([]byte, 100)
	b.Append(data)
	if b.Remaining() != cascade.BlockPayloadSize-100 {
		t.Fatalf("expected %d remaining, got %d", cascade.BlockPayloadSize-100, b.Remaining())
	}
}

func TestBlock_ErrBlockFull(t *testing.T) {
	b := cascade.NewBlock(cascade.BlockTypeData)
	full := make([]byte, cascade.BlockPayloadSize)
	if err := b.Append(full); err != nil {
		t.Fatalf("unexpected error filling block: %v", err)
	}
	if err := b.Append([]byte{0x01}); !errors.Is(err, cascade.ErrBlockFull) {
		t.Fatalf("expected ErrBlockFull, got %v", err)
	}
}

// TestWriteReadBlock_InMemory round-trips a block through a bytes.Buffer.
func TestWriteReadBlock_InMemory(t *testing.T) {
	b := cascade.NewBlock(cascade.BlockTypeData)
	entry := cascade.KVEntry{Key: "foo", Value: "bar"}
	if err := b.Append(cascade.EncodeNPE(entry)); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := cascade.WriteBlock(&buf, b); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != cascade.BlockSize {
		t.Fatalf("expected %d bytes written, got %d", cascade.BlockSize, buf.Len())
	}

	got, err := cascade.ReadBlock(&buf, nil)
	if err != nil {
		t.Fatalf("ReadBlock: %v", err)
	}
	if got.Type() != cascade.BlockTypeData {
		t.Fatalf("got block type %d, want %d", got.Type(), cascade.BlockTypeData)
	}

	decoded, err := cascade.DecodeNPE(bytes.NewReader(got.Payload()))
	if err != nil {
		t.Fatalf("DecodeNPE: %v", err)
	}
	if decoded != entry {
		t.Fatalf("got %+v, want %+v", decoded, entry)
	}
}

// TestWriteReadBlock_DiskRoundTrip writes multiple blocks to a temp file, then reads
// each back by seeking to its block-aligned offset.
//
// Block N starts at byte offset N * BlockSize (4096). This is the same addressing
// scheme the index block uses to point participants to specific data blocks.
func TestWriteReadBlock_DiskRoundTrip(t *testing.T) {
	blocks := []*cascade.Block{
		cascade.NewBlock(cascade.BlockTypeHeader),
		cascade.NewBlock(cascade.BlockTypeIndex),
		cascade.NewBlock(cascade.BlockTypeData),
	}
	entries := []cascade.KVEntry{
		{Key: "hdr-key", Value: "hdr-val"},
		{Key: "idx-key", Value: "idx-val"},
		{Key: "data-key", Value: "data-val"},
	}
	for i, b := range blocks {
		if err := b.Append(cascade.EncodeNPE(entries[i])); err != nil {
			t.Fatal(err)
		}
	}

	f, err := os.CreateTemp(t.TempDir(), "block-*.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	for _, b := range blocks {
		if err := cascade.WriteBlock(f, b); err != nil {
			t.Fatal(err)
		}
	}

	counter := cascade.NewIOCounter()

	for i, want := range entries {
		// Each block occupies exactly BlockSize bytes, so block i starts at i*BlockSize.
		offset := int64(i) * int64(cascade.BlockSize)
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		got, err := cascade.ReadBlock(f, counter)
		if err != nil {
			t.Fatalf("block %d (offset %d): ReadBlock: %v", i, offset, err)
		}
		// Payload() returns the full 4092-byte slice including zero padding.
		// DecodeNPE stops at the first entry; ErrInvalidNPEMagic on padding is the stop signal.
		decoded, err := cascade.DecodeNPE(bytes.NewReader(got.Payload()))
		if err != nil {
			t.Fatalf("block %d: DecodeNPE: %v", i, err)
		}
		if decoded != want {
			t.Fatalf("block %d: got %+v, want %+v", i, decoded, want)
		}
	}

	// One ReadBlock call per block = one IO per block.
	if counter.Count() != int64(len(blocks)) {
		t.Fatalf("expected %d IO reads, got %d", len(blocks), counter.Count())
	}
}

// TestReadBlock_InvalidMagic ensures ReadBlock rejects corrupt data.
func TestReadBlock_InvalidMagic(t *testing.T) {
	garbage := make([]byte, cascade.BlockSize)
	_, err := cascade.ReadBlock(bytes.NewReader(garbage), nil)
	if err == nil {
		t.Fatal("expected error for invalid magic, got nil")
	}
}
