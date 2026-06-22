package cascade

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const npeMagic = uint16(0xC5CD)

var ErrInvalidNPEMagic = errors.New("invalid NPE magic")

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
