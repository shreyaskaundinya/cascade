package cascade

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

const BlockSize = 4096

var blockMagic = [3]byte{0xC5, 0xCD, 0xB1}

// BlockType identifies the kind of block.
type BlockType uint8

const (
	BlockTypeHeader BlockType = 1
	BlockTypeIndex  BlockType = 2
	BlockTypeData   BlockType = 3
)

const blockHeaderSize = 4 // 3 magic bytes + 1 type byte
const BlockPayloadSize = BlockSize - blockHeaderSize

var (
	ErrSSTableEmpty     = errors.New("sstable is empty")
	ErrBlockEmpty       = errors.New("block is empty")
	ErrInvalidBlockSize = errors.New("block size is invalid")
	ErrBlockFull        = errors.New("block is full")
)

// Block is a fixed-size (4096 byte) unit of storage.
type Block struct {
	btype   BlockType
	payload bytes.Buffer
}

// NewBlock creates an empty block of the given type.
func NewBlock(t BlockType) *Block {
	return &Block{btype: t}
}

// Append adds data to the block's payload. Returns ErrBlockFull if there is not enough space.
func (b *Block) Append(data []byte) error {
	if b.payload.Len()+len(data) > BlockPayloadSize {
		return ErrBlockFull
	}
	b.payload.Write(data)
	return nil
}

// Remaining returns how many payload bytes are still available.
func (b *Block) Remaining() int {
	return BlockPayloadSize - b.payload.Len()
}

// Type returns the block's type.
func (b *Block) Type() BlockType {
	return b.btype
}

// Payload returns the raw payload bytes written so far (everything after the block header).
func (b *Block) Payload() []byte {
	return b.payload.Bytes()
}

// serialize serializes the block to a fixed [BlockSize]byte array, zero-padding unused space.
func (b *Block) serialize() [BlockSize]byte {
	var buf [BlockSize]byte
	buf[0] = blockMagic[0]
	buf[1] = blockMagic[1]
	buf[2] = blockMagic[2]
	buf[3] = byte(b.btype)
	copy(buf[blockHeaderSize:], b.payload.Bytes())
	return buf
}

// WriteBlock serializes b to w as exactly BlockSize bytes.
func WriteBlock(w io.Writer, b *Block) error {
	buf := b.serialize()
	_, err := w.Write(buf[:])
	return err
}

// ReadBlock reads exactly BlockSize bytes from r and returns the decoded Block.
// counter is incremented once for the read; pass nil to skip tracking.
func ReadBlock(r io.Reader, counter *IOCounter) (*Block, error) {
	var buf [BlockSize]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return nil, err
	}
	if counter != nil {
		counter.Increment()
	}
	if buf[0] != blockMagic[0] || buf[1] != blockMagic[1] || buf[2] != blockMagic[2] {
		return nil, fmt.Errorf("invalid block magic: %#x %#x %#x", buf[0], buf[1], buf[2])
	}
	b := &Block{btype: BlockType(buf[3])}
	b.payload.Write(buf[blockHeaderSize:])
	return b, nil
}
