package cascade

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
)

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

// SSTableHeader holds the metadata decoded from a header block.
// LowKey and HighKey are the range bounds for this SSTable — use them
// to skip the SSTable entirely if the search key is out of range (1 block read).
type SSTableHeader struct {
	TableNum   uint64
	LowKey     string
	HighKey    string
	ItemCount  uint64
	BlockCount uint64
}

// ParseHeaderBlock decodes the header block payload in field order:
// table_num (uint64) | key range (NPE: Key=low, Value=high) | item_count (uint64) | block_count (uint64)
func ParseHeaderBlock(b *Block) (SSTableHeader, error) {
	r := bytes.NewReader(b.Payload())

	var tableNum uint64
	if err := binary.Read(r, binary.BigEndian, &tableNum); err != nil {
		return SSTableHeader{}, err
	}

	keyRange, err := DecodeNPE(r)
	if err != nil {
		return SSTableHeader{}, err
	}

	var itemCount, blockCount uint64
	if err := binary.Read(r, binary.BigEndian, &itemCount); err != nil {
		return SSTableHeader{}, err
	}
	if err := binary.Read(r, binary.BigEndian, &blockCount); err != nil {
		return SSTableHeader{}, err
	}

	return SSTableHeader{
		TableNum:   tableNum,
		LowKey:     keyRange.Key,
		HighKey:    keyRange.Value,
		ItemCount:  itemCount,
		BlockCount: blockCount,
	}, nil
}

// ParseIndexBlock decodes all IndexEntry records from an index block.
// Iteration stops at ErrInvalidIndexEntryMagic (zero padding) or io.EOF.
func ParseIndexBlock(b *Block) ([]IndexEntry, error) {
	r := bytes.NewReader(b.Payload())
	var entries []IndexEntry
	for {
		ie, err := DecodeIndexEntry(r)
		if errors.Is(err, ErrInvalidIndexEntryMagic) || err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		entries = append(entries, ie)
	}
	return entries, nil
}

func WriteSSTable(tableNum uint64, path string, entries []KVEntry) (*SSTable, error) {
	if len(entries) == 0 {
		return nil, errors.New("cannot write SSTable with no entries")
	}

	// Phase 1: pack entries into data blocks, collecting one IndexEntry per sealed block.
	// entries is assumed pre-sorted (memtable flushes in key order).
	var dataBlocks []*Block
	var indexEntries []IndexEntry

	current := NewBlock(BlockTypeData)
	var highKey string

	closeCurrentBlock := func() {
		dataBlocks = append(dataBlocks, current)

		indexEntries = append(indexEntries, IndexEntry{
			DataBlockNum: uint16(len(dataBlocks) - 1),
			HighKey:      highKey,
		})

		current = NewBlock(BlockTypeData)
	}

	for _, entry := range entries {
		encoded := EncodeNPE(entry)
		if len(encoded) > current.Remaining() {
			closeCurrentBlock()
		}
		current.Append(encoded)
		highKey = entry.Key
	}

	closeCurrentBlock() // finalize the last (possibly partial) block

	// Phase 2: build index block — one IndexEntry per data block
	indexBlock := NewBlock(BlockTypeIndex)
	for _, ie := range indexEntries {
		indexBlock.Append(EncodeIndexEntry(ie))
	}

	// Phase 3: build header block
	// Payload layout: table_num (8 bytes) | low and high key (NPE)
	//                 item_count (8 bytes) | block_count (8 bytes)
	headerBlock := NewBlock(BlockTypeHeader)

	tableNumBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(tableNumBuf, tableNum)
	headerBlock.Append(tableNumBuf)

	KeyRange := KVEntry{
		Key:   entries[0].Key,              // Low Key for SSTable
		Value: entries[len(entries)-1].Key, // High Key for SSTable
	}
	headerBlock.Append(EncodeNPE(KeyRange))

	countBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(countBuf, uint64(len(entries)))
	headerBlock.Append(countBuf)

	binary.BigEndian.PutUint64(countBuf, uint64(len(dataBlocks)))
	headerBlock.Append(countBuf)

	// Phase 4: write to disk
	// Block 0: header, Block 1: index, Blocks 2..N: data
	// Data block K sits at file offset (2 + K) * BlockSize
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if err := WriteBlock(f, headerBlock); err != nil {
		return nil, err
	}
	if err := WriteBlock(f, indexBlock); err != nil {
		return nil, err
	}
	for _, db := range dataBlocks {
		if err := WriteBlock(f, db); err != nil {
			return nil, err
		}
	}

	return &SSTable{Path: path}, nil
}

func (s *SSTable) Get(key string, counter *IOCounter) (KVEntry, bool, error) {
	returnErr := func(err error) (KVEntry, bool, error) {
		return KVEntry{}, false, err
	}

	// get the file
	f, err := os.Open(s.Path)
	if err != nil {
		return returnErr(err)
	}

	fileStat, fileStatErr := f.Stat()
	if fileStatErr != nil {
		return returnErr(fileStatErr)
	}

	size := fileStat.Size()

	if size == 0 {
		return returnErr(ErrSSTableEmpty)
	}

	newOffset, seekErr := f.Seek(0, 0)
	if seekErr != nil {
		return returnErr(seekErr)
	}

	if newOffset != 0 {
		return returnErr(errors.New("could not seek to 0"))
	}

	// Read the Header
	headerBlock, headerBlockErr := ReadBlock(f, counter)
	if headerBlockErr != nil {
		return returnErr(headerBlockErr)
	}

	parsedHeader, parsedHeaderErr := ParseHeaderBlock(headerBlock)
	if parsedHeaderErr != nil {
		return returnErr(parsedHeaderErr)
	}

	if key < parsedHeader.LowKey || key > parsedHeader.HighKey {
		return returnErr(ErrNotFound)
	}

	// Read the Index Block
	indexBlkBlock, indexBlkBlockErr := ReadBlock(f, counter)
	if indexBlkBlockErr != nil {
		return returnErr(indexBlkBlockErr)
	}

	parsedIndexBlk, parsedIndexBlkErr := ParseIndexBlock(indexBlkBlock)
	if parsedIndexBlkErr != nil {
		return returnErr(parsedIndexBlkErr)
	}

	// go through all index entries and check which datablock to search
	//TODO: do binary search here

	blkNums := make([]uint16, 0)

	// sorted ascending
	// higher keys come later
	// if key > high key go to next
	for i := range parsedIndexBlk {
		if key <= parsedIndexBlk[i].HighKey {
			blkNums = append(blkNums, parsedIndexBlk[i].DataBlockNum)
		}
	}

	// Read all the data blocks

	// read the blk required
	for _, blkNum := range blkNums {
		// seen to start
		f.Seek(0, 0)

		blk := make([]byte, BlockSize)

		// get block offset
		blkOffset := BlockSize + BlockSize + int64((blkNum * BlockSize))

		n, blkReadErr := f.ReadAt(blk, blkOffset)
		if blkReadErr != nil {
			return returnErr(blkReadErr)
		}

		if n != BlockSize {
			return returnErr(ErrInvalidBlockSize)
		}

		parsedBlk, parsedBlkErr := ReadBlock(bytes.NewBuffer(blk), counter)
		if parsedBlkErr != nil {
			return returnErr(parsedBlkErr)
		}

		reader := bytes.NewBuffer(parsedBlk.Payload())

		//TODO: make this loop better
		for {
			entry, entryDecodeErr := DecodeNPE(reader)
			if entryDecodeErr != nil {
				break
			}

			if entry.Key == key {
				return entry, true, nil
			}
		}
	}

	return KVEntry{}, false, nil
}

func (s *SSTable) Scan(counter *IOCounter) ([]KVEntry, error) {
	returnErr := func(err error) ([]KVEntry, error) {
		return nil, err
	}

	// get the file
	f, err := os.Open(s.Path)
	if err != nil {
		return returnErr(err)
	}

	fileStat, fileStatErr := f.Stat()
	if fileStatErr != nil {
		return returnErr(fileStatErr)
	}

	size := fileStat.Size()

	if size == 0 {
		return returnErr(ErrSSTableEmpty)
	}

	newOffset, seekErr := f.Seek(0, 0)
	if seekErr != nil {
		return returnErr(seekErr)
	}

	if newOffset != 0 {
		return returnErr(errors.New("could not seek to 0"))
	}

	// Read the Header
	headerBlock, headerBlockErr := ReadBlock(f, counter)
	if headerBlockErr != nil {
		return returnErr(headerBlockErr)
	}

	parsedHeader, parsedHeaderErr := ParseHeaderBlock(headerBlock)
	if parsedHeaderErr != nil {
		return returnErr(parsedHeaderErr)
	}

	// skip index block
	f.Seek(BlockSize, 1)

	entries := make([]KVEntry, 0)

	// Read all the data blocks
	for range parsedHeader.BlockCount {
		parsedBlk, parsedBlkErr := ReadBlock(f, counter)
		if parsedBlkErr != nil {
			return returnErr(parsedBlkErr)
		}

		reader := bytes.NewBuffer(parsedBlk.Payload())

		//TODO: make this loop better

		for {
			entry, entryDecodeErr := DecodeNPE(reader)
			if entryDecodeErr != nil {
				break
			}

			entries = append(entries, entry)
		}
	}

	return entries, nil
}
