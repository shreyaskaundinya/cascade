package cascade

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const npeMagic = uint16(0xC5CD)
const indexEntryMagic = uint16(0xC5CE)

var ErrInvalidNPEMagic = errors.New("invalid NPE magic")
var ErrInvalidIndexEntryMagic = errors.New("invalid index entry magic")

// NPEEncodedSize returns the number of bytes EncodeNPE will produce for entry.
func NPEEncodedSize(entry KVEntry) int {
	if entry.IsTombstone {
		return 6 + len(entry.Key)
	}
	return 6 + len(entry.Key) + len(entry.Value)
}

// EncodeNPE serializes a KVEntry into the Nullable Pair Encoding format.
func EncodeNPE(entry KVEntry) []byte {
	keySz := uint16(len(entry.Key))
	var valSz uint16
	if !entry.IsTombstone {
		valSz = uint16(len(entry.Value))
	}

	buf := make([]byte, NPEEncodedSize(entry))
	binary.BigEndian.PutUint16(buf[0:], npeMagic)
	binary.BigEndian.PutUint16(buf[2:], keySz)
	binary.BigEndian.PutUint16(buf[4:], valSz)
	copy(buf[6:], entry.Key)
	if !entry.IsTombstone {
		copy(buf[6+int(keySz):], entry.Value)
	}
	return buf
}

// DecodeNPE reads one NPE-encoded entry from r.
// Returns ErrInvalidNPEMagic if the magic bytes don't match (including zero padding).
func DecodeNPE(r io.Reader) (KVEntry, error) {
	var magic uint16
	if err := binary.Read(r, binary.BigEndian, &magic); err != nil {
		return KVEntry{}, err
	}
	if magic != npeMagic {
		return KVEntry{}, fmt.Errorf("%w: got %#x", ErrInvalidNPEMagic, magic)
	}

	var keySz, valSz uint16
	if err := binary.Read(r, binary.BigEndian, &keySz); err != nil {
		return KVEntry{}, err
	}
	if err := binary.Read(r, binary.BigEndian, &valSz); err != nil {
		return KVEntry{}, err
	}

	key := make([]byte, keySz)
	if _, err := io.ReadFull(r, key); err != nil {
		return KVEntry{}, err
	}

	entry := KVEntry{Key: string(key)}
	if valSz == 0 {
		entry.IsTombstone = true
		return entry, nil
	}

	val := make([]byte, valSz)
	if _, err := io.ReadFull(r, val); err != nil {
		return KVEntry{}, err
	}

	entry.Value = string(val)
	return entry, nil
}

// IndexEntry records the high key and position of one data block inside the index block.
// The data block's file offset is: (2 + DataBlockNum) * BlockSize
// (blocks 0 and 1 are the header and index blocks respectively).
type IndexEntry struct {
	DataBlockNum uint16
	HighKey      string
}

// IndexEntryEncodedSize returns the number of bytes EncodeIndexEntry will produce.
func IndexEntryEncodedSize(e IndexEntry) int {
	return 6 + len(e.HighKey) // 2 magic + 2 block_num + 2 key_sz
}

// EncodeIndexEntry serializes an IndexEntry.
func EncodeIndexEntry(e IndexEntry) []byte {
	keySz := uint16(len(e.HighKey))
	buf := make([]byte, IndexEntryEncodedSize(e))
	binary.BigEndian.PutUint16(buf[0:], indexEntryMagic)
	binary.BigEndian.PutUint16(buf[2:], e.DataBlockNum)
	binary.BigEndian.PutUint16(buf[4:], keySz)
	copy(buf[6:], e.HighKey)
	return buf
}

// DecodeIndexEntry reads one index entry from r.
// Returns ErrInvalidIndexEntryMagic if the magic bytes don't match (including zero padding).
func DecodeIndexEntry(r io.Reader) (IndexEntry, error) {
	var magic uint16
	if err := binary.Read(r, binary.BigEndian, &magic); err != nil {
		return IndexEntry{}, err
	}
	if magic != indexEntryMagic {
		return IndexEntry{}, fmt.Errorf("%w: got %#x", ErrInvalidIndexEntryMagic, magic)
	}

	var blockNum, keySz uint16
	if err := binary.Read(r, binary.BigEndian, &blockNum); err != nil {
		return IndexEntry{}, err
	}
	if err := binary.Read(r, binary.BigEndian, &keySz); err != nil {
		return IndexEntry{}, err
	}

	key := make([]byte, keySz)
	if _, err := io.ReadFull(r, key); err != nil {
		return IndexEntry{}, err
	}

	return IndexEntry{DataBlockNum: blockNum, HighKey: string(key)}, nil
}
