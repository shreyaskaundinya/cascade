package cascade

import "errors"

var ErrKeyNotFound = errors.New("key not found")

/*
SSTable Format (Inspired by RocksDB's Block-based format)

Block Size: 4096 Bytes (4KB)

Nullable Pair Encoding Format
	Magic: 0xC5CD (2 Bytes, 16 Bits)
	Key_sz: 2 Bytes (16 bits)
	Val_sz: 2 Bytes (16 bits) - if zero, tombstone
	Key: <>....\0 (Key_sz Bytes)
	Value (if present): <>....\0 (Val_sz Bytes)

Common Block header Format
	Magic: 0xC5CDB1 (3 Bytes, 24 Bits)
	Block Type: 1: Header, 2: Index, 3: Data (1 Byte, 8 Bits)

Block 1: Header Block
	Contains
	- Block Header
	- Table Number: unsigned 64-bit integer (ID number of this SSTable)
	- NPE High Key and Low Key
	- Item Count: unsigned 64-bit integer
	- Block Count: unsigned 64-bit integer

Block 2: Index Block
	Contains
	- Block Header
	per block high key and offset of that block
	[block_num: 1 byte][block_offset: 1 byte][high key size: 1 byte][high key: ....]

Block 3..N: Data Blocks
	Contains
	- Block Header
	- NPE Encoded key-value pairs

*/

type SSTable struct {
	Path string
}

func WriteSSTable(path string, entries []KVEntry) (*SSTable, error) { return nil, nil }

func (s *SSTable) Get(key string, counter *IOCounter) (KVEntry, bool, error) {
	return KVEntry{}, false, nil
}

func (s *SSTable) Scan(counter *IOCounter) ([]KVEntry, error) { return nil, nil }
