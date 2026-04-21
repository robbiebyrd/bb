// Package mf4 contains shared primitives for reading and writing ASAM MDF4
// (.mf4) files. Block layout constants, block header read/write helpers, and
// CAN_DataFrame record encoding are used by both the playback parser
// (internal/connection/playback) and the MF4 output client
// (internal/output/mf4).
package mf4

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
)

// MDF4 block layout constants.
const (
	// IDBlockSize is the length of the fixed-format identification block
	// that precedes all MDF4 block content.
	IDBlockSize = 64

	// HeaderSize is the size of the common block header:
	//   [4]byte  block id   (e.g. "##HD")
	//   [4]byte  reserved
	//   uint64   block length (includes header, links, data)
	//   uint64   link count
	HeaderSize = 24
)

// Block type identifiers.
const (
	BlockIDHD = "##HD"
	BlockIDDG = "##DG"
	BlockIDCG = "##CG"
	BlockIDCN = "##CN"
	BlockIDTX = "##TX"
	BlockIDMD = "##MD"
	BlockIDDT = "##DT"
	BlockIDFH = "##FH"
)

// CAN channel group acquisition names used to tag channel groups inside a
// data group.
const (
	AcqNameCANDataFrame = "CAN_DataFrame"
	AcqNameTimestamp    = "Timestamp"
)

// CAN_DataFrame record size: 8 bytes float64 timestamp + 14 bytes composite.
const CANRecordSize = 22

// CG flag bits.
const (
	CGFlagVLSD = 0x0001
	CGFlagBus  = 0x0006 // "bus event" - CAN CG metadata hint
)

// Channel data types (subset relevant to CAN logging).
const (
	DataTypeFloatLE  = 4  // IEEE-754 little-endian float
	DataTypeByteArr  = 10 // raw byte array
	DataTypeUnsignLE = 0
)

// Channel types.
const (
	ChannelTypeFixedLength = 0
	ChannelTypeMaster      = 2
)

// SyncType for master (time) channels.
const SyncTypeTime = 1

// ReadBlock reads a generic MDF4 block at the given address and returns its
// link array and the remaining data bytes.
func ReadBlock(f io.ReadSeeker, addr int64) ([]int64, []byte, error) {
	if _, err := f.Seek(addr, io.SeekStart); err != nil {
		return nil, nil, err
	}
	var hdr [HeaderSize]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return nil, nil, err
	}
	linkCount := binary.LittleEndian.Uint64(hdr[16:24])
	blockLen := binary.LittleEndian.Uint64(hdr[8:16])

	links := make([]int64, linkCount)
	for i := uint64(0); i < linkCount; i++ {
		if err := binary.Read(f, binary.LittleEndian, &links[i]); err != nil {
			return nil, nil, err
		}
	}

	dataSize := blockLen - HeaderSize - linkCount*8
	data := make([]byte, dataSize)
	if _, err := io.ReadFull(f, data); err != nil {
		return nil, nil, err
	}
	return links, data, nil
}

// ReadDTData reads the raw data bytes from a ##DT block. For unfinalized
// files the DT header reports blockLen==HeaderSize (no data), so we read
// from after the header to EOF.
func ReadDTData(f *os.File, addr int64) ([]byte, error) {
	if _, err := f.Seek(addr, io.SeekStart); err != nil {
		return nil, err
	}
	var hdr [HeaderSize]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return nil, err
	}

	id := string(hdr[0:4])
	blockLen := binary.LittleEndian.Uint64(hdr[8:16])

	if id != BlockIDDT {
		return nil, fmt.Errorf("expected %s block, got %q", BlockIDDT, id)
	}

	dataSize := blockLen - HeaderSize
	if dataSize > 0 {
		data := make([]byte, dataSize)
		if _, err := io.ReadFull(f, data); err != nil {
			return nil, err
		}
		return data, nil
	}

	return io.ReadAll(f)
}

// ReadText reads a ##TX or ##MD block and returns the text content with any
// trailing NUL padding stripped. Returns "" if addr == 0 or on any error.
func ReadText(f io.ReadSeeker, addr int64) string {
	if addr == 0 {
		return ""
	}
	_, data, err := ReadBlock(f, addr)
	if err != nil || len(data) == 0 {
		return ""
	}
	return strings.TrimRight(string(data), "\x00")
}

// ReadRecordID reads a 1, 2, 4, or 8 byte record ID from the start of buf.
// Returns 0 if size is not one of those values.
func ReadRecordID(buf []byte, size int) uint64 {
	switch size {
	case 1:
		return uint64(buf[0])
	case 2:
		return uint64(binary.LittleEndian.Uint16(buf[:2]))
	case 4:
		return uint64(binary.LittleEndian.Uint32(buf[:4]))
	case 8:
		return binary.LittleEndian.Uint64(buf[:8])
	default:
		return 0
	}
}

// WriteRecordID writes a record ID into the start of buf using the given
// size. Size must be 1, 2, 4, or 8 bytes.
func WriteRecordID(buf []byte, size int, id uint64) {
	switch size {
	case 1:
		buf[0] = byte(id)
	case 2:
		binary.LittleEndian.PutUint16(buf[:2], uint16(id))
	case 4:
		binary.LittleEndian.PutUint32(buf[:4], uint32(id))
	case 8:
		binary.LittleEndian.PutUint64(buf[:8], id)
	}
}

// IsMF4File checks the first 8 bytes of a file for MDF4 magic bytes.
// Both finalized ("MDF     ") and unfinalized ("UnFinMF ") files are
// recognized.
func IsMF4File(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var magic [8]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return false
	}
	s := string(magic[:])
	return strings.HasPrefix(s, "MDF") || strings.HasPrefix(s, "UnFinMF")
}

// EncodeCANFrameRecord writes a 22-byte CAN_DataFrame record body into rec.
// rec must be at least CANRecordSize bytes long. The record layout matches
// the ASAM MDF4 CAN_DataFrame channel layout consumed by the playback
// parser:
//
//	[0:8]   float64 LE  timestamp (ns since measurement start)
//	[8:22]  CAN composite:
//	  byte 0, bits 0-1:   BusChannel (always 0 here)
//	  byte 0, bits 2-30:  CAN ID (29 bits, LE uint32)
//	  byte 3, bit 7:      IDE (set when canID > 11 bits)
//	  byte 4, bit 0:      Dir (0=Rx, 1=Tx)
//	  byte 4, bits 1-7:   DataLength
//	  byte 5, bit 0:      EDL (0 for classic CAN)
//	  byte 5, bit 1:      BRS
//	  byte 5, bits 2-5:   DLC
//	  bytes 6-13:         VLSD offset (uint64 LE)
func EncodeCANFrameRecord(rec []byte, tsNs int64, canID uint32, tx bool, dataLen uint8, vlsdOffset uint64) {
	if len(rec) < CANRecordSize {
		return
	}
	for i := range rec[:CANRecordSize] {
		rec[i] = 0
	}

	// Timestamp as float64 nanoseconds.
	binary.LittleEndian.PutUint64(rec[0:8], math.Float64bits(float64(tsNs)))

	can := rec[8:22]
	// Byte 0, bits 2-30 hold the 29-bit CAN ID (shifted left by 2).
	idField := (canID & 0x1FFFFFFF) << 2
	binary.LittleEndian.PutUint32(can[0:4], idField)

	// IDE bit (extended frame) at byte 3, bit 7 if the ID exceeds 11 bits.
	if canID > 0x7FF {
		can[3] |= 0x80
	}

	var dir byte
	if tx {
		dir = 1
	}
	can[4] = dir | ((dataLen & 0x7F) << 1)
	can[5] = (dataLen & 0x0F) << 2 // DLC mirrors DataLength for classic CAN

	binary.LittleEndian.PutUint64(can[6:14], vlsdOffset)
}

// WriteIDBlock writes the 64-byte identification block to w. If unfinalized
// is true the block is flagged unfinalized and the "UnFinMF " magic is used;
// otherwise "MDF     " is written.
func WriteIDBlock(unfinalized bool) []byte {
	buf := make([]byte, IDBlockSize)
	if unfinalized {
		copy(buf[0:8], "UnFinMF ")
	} else {
		copy(buf[0:8], "MDF     ")
	}
	copy(buf[8:16], "4.11    ")
	copy(buf[16:24], "bb      ")
	binary.LittleEndian.PutUint16(buf[28:30], 411)
	if unfinalized {
		binary.LittleEndian.PutUint16(buf[60:62], 1)
	}
	return buf
}

// Block is a staged MDF4 block prior to serialization. Link values are
// resolved as absolute file offsets when Bytes is called.
type Block struct {
	ID    string
	Links []int64
	Data  []byte
}

// NewBlock starts assembling a block with linkCount link slots (filled with
// zero by default) and zero-filled data of dataSize bytes.
func NewBlock(id string, linkCount int, dataSize int) *Block {
	return &Block{
		ID:    id,
		Links: make([]int64, linkCount),
		Data:  make([]byte, dataSize),
	}
}

// Size returns the total serialized length in bytes.
func (b *Block) Size() int64 {
	return int64(HeaderSize + len(b.Links)*8 + len(b.Data))
}

// Bytes serializes the block to its on-disk representation.
func (b *Block) Bytes() []byte {
	total := b.Size()
	out := make([]byte, total)
	copy(out[0:4], b.ID)
	binary.LittleEndian.PutUint64(out[8:16], uint64(total))
	binary.LittleEndian.PutUint64(out[16:24], uint64(len(b.Links)))
	off := HeaderSize
	for _, link := range b.Links {
		binary.LittleEndian.PutUint64(out[off:off+8], uint64(link))
		off += 8
	}
	copy(out[off:], b.Data)
	return out
}

// BuildTXBlock builds a ##TX block carrying a NUL-terminated, 8-byte aligned
// text string.
func BuildTXBlock(text string) []byte {
	payload := append([]byte(text), 0)
	blockLen := HeaderSize + len(payload)
	if blockLen%8 != 0 {
		blockLen += 8 - (blockLen % 8)
	}
	b := make([]byte, blockLen)
	copy(b[0:4], BlockIDTX)
	binary.LittleEndian.PutUint64(b[8:16], uint64(blockLen))
	// LinkCount = 0
	copy(b[HeaderSize:], payload)
	return b
}
