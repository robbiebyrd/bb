package playback

import (
	"context"
	"encoding/binary"
	"math"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- MF4 test helpers --------------------------------------------------------

// mf4Builder constructs a minimal MF4 binary file for testing.
// It builds an unfinalized, unsorted file with a single DG containing:
//   - CG 1: CAN_DataFrame (22 bytes per record: 8 ts + 14 CAN composite)
//   - CG 2: VLSD (variable-length data bytes for CAN payloads)
type mf4Builder struct {
	frames []testCANFrame
}

type testCANFrame struct {
	timestampNs float64 // relative to measurement start, in nanoseconds
	id          uint32  // 29-bit CAN ID
	dataLength  uint8
	dlc         uint8
	data        []byte
	dir         uint8 // 0=Rx, 1=Tx
}

func (b *mf4Builder) build() []byte {
	// Layout:
	// [ID block 64B][HD block][TX "CAN_DataFrame"][CG1][CG2][CN timestamp][CN CAN_DataFrame][DT header][records...]
	//
	// We simplify by placing blocks sequentially and calculating addresses.

	buf := make([]byte, 0, 4096)

	// === ID Block (64 bytes) ===
	idBlock := make([]byte, 64)
	copy(idBlock[0:8], "UnFinMF ")
	copy(idBlock[8:16], "4.11    ")
	copy(idBlock[16:24], "TEST    ")
	binary.LittleEndian.PutUint16(idBlock[28:30], 411)
	binary.LittleEndian.PutUint16(idBlock[60:62], 1) // unfinalized flag
	buf = append(buf, idBlock...)

	// Track addresses for blocks we'll create
	hdAddr := int64(64)

	// === TX Block for "CAN_DataFrame" ===
	txAcqName := buildTXBlock("CAN_DataFrame")
	txTimestamp := buildTXBlock("Timestamp")
	txCANDF := buildTXBlock("CAN_DataFrame")

	// === Pre-calculate addresses ===
	// HD: 24 header + 6*8 links + 24 data = 96 bytes
	hdSize := int64(96)
	txAcqNameAddr := hdAddr + hdSize
	txTimestampAddr := txAcqNameAddr + int64(len(txAcqName))
	txCANDFAddr := txTimestampAddr + int64(len(txTimestamp))

	// CG1: 24 header + 6*8 links + 32 data = 104 bytes
	cg1Addr := txCANDFAddr + int64(len(txCANDF))
	cg1Size := int64(104)

	// CG2 (VLSD): 24 header + 6*8 links + 32 data = 104 bytes
	cg2Addr := cg1Addr + cg1Size
	cg2Size := int64(104)

	// CN Timestamp: 24 header + 8*8 links + 48 data = 136 bytes
	cnTsAddr := cg2Addr + cg2Size
	cnTsSize := int64(136)

	// CN CAN_DataFrame: 24 header + 8*8 links + 48 data = 136 bytes
	cnCANAddr := cnTsAddr + cnTsSize
	cnCANSize := int64(136)

	// DG: 24 header + 4*8 links + 8 data = 64 bytes
	dgAddr := cnCANAddr + cnCANSize
	dgSize := int64(64)

	// DT Block header (24 bytes) - unfinalized so length=24
	dtAddr := dgAddr + dgSize

	// === HD Block ===
	hd := make([]byte, hdSize)
	copy(hd[0:4], "##HD")
	binary.LittleEndian.PutUint64(hd[8:16], uint64(hdSize))
	binary.LittleEndian.PutUint64(hd[16:24], 6) // link count
	// Links: DgFirst, FhFirst, ChFirst, AtFirst, EvFirst, MdComment
	binary.LittleEndian.PutUint64(hd[24:32], uint64(dgAddr))
	// Data: StartTimeNs(8) TZOffset(2) DSTOffset(2) TimeFlags(1) TimeClass(1) Flags(1) Reserved(1) StartAngle(8) StartDist(8)
	binary.LittleEndian.PutUint64(hd[72:80], 1579615444000000000) // StartTimeNs
	buf = append(buf, hd...)

	// === TX Blocks ===
	buf = append(buf, txAcqName...)
	buf = append(buf, txTimestamp...)
	buf = append(buf, txCANDF...)

	// === CG1 Block (CAN_DataFrame) ===
	cg1 := make([]byte, cg1Size)
	copy(cg1[0:4], "##CG")
	binary.LittleEndian.PutUint64(cg1[8:16], uint64(cg1Size))
	binary.LittleEndian.PutUint64(cg1[16:24], 6) // link count
	// Links: Next, CnFirst, TxAcqName, SiAcqSource, SrFirst, MdComment
	binary.LittleEndian.PutUint64(cg1[24:32], uint64(cg2Addr))       // Next -> CG2
	binary.LittleEndian.PutUint64(cg1[32:40], uint64(cnTsAddr))      // CnFirst -> CN Timestamp
	binary.LittleEndian.PutUint64(cg1[40:48], uint64(txAcqNameAddr)) // TxAcqName
	// Data: RecordID(8) CycleCount(8) Flags(2) PathSeparator(2) Reserved(4) DataBytes(4) InvalBytes(4)
	dataOffset := 24 + 6*8                                                  // 72
	binary.LittleEndian.PutUint64(cg1[dataOffset:dataOffset+8], 1)          // RecordID=1
	binary.LittleEndian.PutUint64(cg1[dataOffset+8:dataOffset+16], 0)       // CycleCount (0 for unfinalized)
	binary.LittleEndian.PutUint16(cg1[dataOffset+16:dataOffset+18], 0x0006) // Flags: bus event
	binary.LittleEndian.PutUint32(cg1[dataOffset+24:dataOffset+28], 22)     // DataBytes=22
	buf = append(buf, cg1...)

	// === CG2 Block (VLSD) ===
	cg2 := make([]byte, cg2Size)
	copy(cg2[0:4], "##CG")
	binary.LittleEndian.PutUint64(cg2[8:16], uint64(cg2Size))
	binary.LittleEndian.PutUint64(cg2[16:24], 6) // link count
	// Links: all zero (no next, no channels)
	// Data:
	binary.LittleEndian.PutUint64(cg2[dataOffset:dataOffset+8], 2)          // RecordID=2
	binary.LittleEndian.PutUint16(cg2[dataOffset+16:dataOffset+18], 0x0001) // Flags: VLSD
	buf = append(buf, cg2...)

	// === CN Timestamp ===
	cnTs := make([]byte, cnTsSize)
	copy(cnTs[0:4], "##CN")
	binary.LittleEndian.PutUint64(cnTs[8:16], uint64(cnTsSize))
	binary.LittleEndian.PutUint64(cnTs[16:24], 8) // link count
	// Links: Next, Composition, TxName, SiSource, CcConversion, Data, MdUnit, MdComment
	binary.LittleEndian.PutUint64(cnTs[24:32], uint64(cnCANAddr))       // Next -> CN CAN_DataFrame
	binary.LittleEndian.PutUint64(cnTs[40:48], uint64(txTimestampAddr)) // TxName
	// Data: Type(1) SyncType(1) DataType(1) BitOffset(1) ByteOffset(4) BitCount(4) ...
	cnTsData := cnTs[24+8*8:]                         // offset past header+links
	cnTsData[0] = 2                                   // Type=Master
	cnTsData[1] = 1                                   // SyncType=Time
	cnTsData[2] = 4                                   // DataType=IEEE754FloatLE
	binary.LittleEndian.PutUint32(cnTsData[4:8], 0)   // ByteOffset=0
	binary.LittleEndian.PutUint32(cnTsData[8:12], 64) // BitCount=64
	buf = append(buf, cnTs...)

	// === CN CAN_DataFrame ===
	cnCAN := make([]byte, cnCANSize)
	copy(cnCAN[0:4], "##CN")
	binary.LittleEndian.PutUint64(cnCAN[8:16], uint64(cnCANSize))
	binary.LittleEndian.PutUint64(cnCAN[16:24], 8) // link count
	// Links: Next=0, Composition=0, TxName, ...
	binary.LittleEndian.PutUint64(cnCAN[40:48], uint64(txCANDFAddr)) // TxName
	// Data:
	cnCANData := cnCAN[24+8*8:]
	cnCANData[0] = 0                                    // Type=FixedLength
	cnCANData[2] = 10                                   // DataType=ByteArray
	binary.LittleEndian.PutUint32(cnCANData[4:8], 8)    // ByteOffset=8
	binary.LittleEndian.PutUint32(cnCANData[8:12], 112) // BitCount=112 (14 bytes)
	buf = append(buf, cnCAN...)

	// === DG Block ===
	dg := make([]byte, dgSize)
	copy(dg[0:4], "##DG")
	binary.LittleEndian.PutUint64(dg[8:16], uint64(dgSize))
	binary.LittleEndian.PutUint64(dg[16:24], 4) // link count
	// Links: Next, CgFirst, Data, MdComment
	binary.LittleEndian.PutUint64(dg[32:40], uint64(cg1Addr)) // CgFirst
	binary.LittleEndian.PutUint64(dg[40:48], uint64(dtAddr))  // Data -> DT
	// Data: RecIDSize
	dg[56] = 1 // RecIDSize=1
	buf = append(buf, dg...)

	// === DT Block (header only, length=24 for unfinalized) ===
	dtHeader := make([]byte, 24)
	copy(dtHeader[0:4], "##DT")
	binary.LittleEndian.PutUint64(dtHeader[8:16], 24) // unfinalized: length=24
	buf = append(buf, dtHeader...)

	// === Data records ===
	vlsdOffset := uint64(0)
	for _, frame := range b.frames {
		// CAN_DataFrame record (RecID=1, 22 bytes)
		rec := make([]byte, 23) // 1 recID + 22 data
		rec[0] = 1              // RecordID

		// Timestamp: float64 LE at offset 1
		binary.LittleEndian.PutUint64(rec[1:9], math.Float64bits(frame.timestampNs))

		// CAN composite (14 bytes at offset 9)
		can := rec[9:23]

		// BusChannel: byte 0, bits 0-1 (always 0 for test)
		// ID: byte 0, bits 2-30 (29 bits)
		idField := uint32(frame.id) << 2
		binary.LittleEndian.PutUint32(can[0:4], idField)

		// Dir: byte 4, bit 0
		can[4] = frame.dir & 1

		// DataLength: byte 4, bits 1-7
		can[4] |= (frame.dataLength & 0x7F) << 1

		// EDL: byte 5, bit 0 (always 0 for classic CAN)
		// BRS: byte 5, bit 1 (always 0)
		// DLC: byte 5, bits 2-5
		can[5] = (frame.dlc & 0x0F) << 2

		// DataBytes VLSD offset: bytes 6-13
		binary.LittleEndian.PutUint64(can[6:14], vlsdOffset)

		buf = append(buf, rec...)

		// VLSD record (RecID=2, 4-byte length + data)
		vlsdRec := make([]byte, 1+4+len(frame.data))
		vlsdRec[0] = 2 // RecordID
		binary.LittleEndian.PutUint32(vlsdRec[1:5], uint32(len(frame.data)))
		copy(vlsdRec[5:], frame.data)
		buf = append(buf, vlsdRec...)

		vlsdOffset += uint64(4 + len(frame.data))
	}

	return buf
}

func buildTXBlock(text string) []byte {
	// TX block: header (24 bytes) + text + null terminator, padded to 8-byte alignment
	textBytes := append([]byte(text), 0)
	blockLen := 24 + len(textBytes)
	// Pad to 8-byte alignment
	if blockLen%8 != 0 {
		blockLen += 8 - (blockLen % 8)
	}
	b := make([]byte, blockLen)
	copy(b[0:4], "##TX")
	binary.LittleEndian.PutUint64(b[8:16], uint64(blockLen))
	// LinkCount = 0
	copy(b[24:], textBytes)
	return b
}

func writeMF4Temp(t *testing.T, data []byte) string {
	t.Helper()
	f, err := os.CreateTemp("", "playback_mf4_test_*.mf4")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(f.Name()) })
	_, err = f.Write(data)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

func twoFrameMF4(t *testing.T) string {
	t.Helper()
	b := &mf4Builder{
		frames: []testCANFrame{
			{
				timestampNs: 0,
				id:          0x123,
				dataLength:  8,
				dlc:         8,
				data:        []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x02, 0x03, 0x04},
				dir:         0,
			},
			{
				timestampNs: 50_000_000, // 50ms later
				id:          0x456,
				dataLength:  4,
				dlc:         4,
				data:        []byte{0xCA, 0xFE, 0xBA, 0xBE},
				dir:         1,
			},
		},
	}
	return writeMF4Temp(t, b.build())
}

// --- MF4Parser tests ---------------------------------------------------------

func TestMF4Parser_Parse_SingleFrame(t *testing.T) {
	b := &mf4Builder{
		frames: []testCANFrame{
			{
				timestampNs: 0,
				id:          0x028,
				dataLength:  8,
				dlc:         8,
				data:        []byte{0x07, 0xD0, 0x03, 0xFC, 0x07, 0xD0, 0x90, 0x34},
				dir:         0,
			},
		},
	}
	path := writeMF4Temp(t, b.build())
	entries, err := (&MF4Parser{l: silentLogger()}).Parse(path)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	e := entries[0]
	assert.Equal(t, int64(0), e.OffsetNs)
	assert.Equal(t, uint32(0x028), e.ID)
	assert.False(t, e.Transmit)
	assert.Equal(t, uint8(8), e.Length)
	assert.Equal(t, []byte{0x07, 0xD0, 0x03, 0xFC, 0x07, 0xD0, 0x90, 0x34}, e.Data)
}

func TestMF4Parser_Parse_TwoFrames_OffsetRelativeToFirst(t *testing.T) {
	path := twoFrameMF4(t)
	entries, err := (&MF4Parser{l: silentLogger()}).Parse(path)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	assert.Equal(t, int64(0), entries[0].OffsetNs)
	assert.Equal(t, uint32(0x123), entries[0].ID)
	assert.False(t, entries[0].Transmit)
	assert.Equal(t, uint8(8), entries[0].Length)
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x02, 0x03, 0x04}, entries[0].Data)

	assert.Equal(t, int64(50*time.Millisecond), entries[1].OffsetNs)
	assert.Equal(t, uint32(0x456), entries[1].ID)
	assert.True(t, entries[1].Transmit)
	assert.Equal(t, uint8(4), entries[1].Length)
	assert.Equal(t, []byte{0xCA, 0xFE, 0xBA, 0xBE}, entries[1].Data)
}

func TestMF4Parser_Parse_EmptyFile(t *testing.T) {
	b := &mf4Builder{frames: nil}
	path := writeMF4Temp(t, b.build())
	entries, err := (&MF4Parser{l: silentLogger()}).Parse(path)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestMF4Parser_Parse_ShortFrame(t *testing.T) {
	b := &mf4Builder{
		frames: []testCANFrame{
			{
				timestampNs: 0,
				id:          0x00F,
				dataLength:  1,
				dlc:         1,
				data:        []byte{0x42},
				dir:         0,
			},
		},
	}
	path := writeMF4Temp(t, b.build())
	entries, err := (&MF4Parser{l: silentLogger()}).Parse(path)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, uint32(0x00F), entries[0].ID)
	assert.Equal(t, uint8(1), entries[0].Length)
	assert.Equal(t, []byte{0x42}, entries[0].Data)
}

func TestMF4Parser_Parse_TxDirection(t *testing.T) {
	b := &mf4Builder{
		frames: []testCANFrame{
			{timestampNs: 0, id: 0x100, dataLength: 2, dlc: 2, data: []byte{0xAB, 0xCD}, dir: 1},
		},
	}
	path := writeMF4Temp(t, b.build())
	entries, err := (&MF4Parser{l: silentLogger()}).Parse(path)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Transmit)
}

func TestMF4Parser_Parse_NonZeroStartTimestamp(t *testing.T) {
	b := &mf4Builder{
		frames: []testCANFrame{
			{timestampNs: 1_000_000_000, id: 0x001, dataLength: 1, dlc: 1, data: []byte{0xAA}, dir: 0},
			{timestampNs: 1_100_000_000, id: 0x002, dataLength: 1, dlc: 1, data: []byte{0xBB}, dir: 0},
		},
	}
	path := writeMF4Temp(t, b.build())
	entries, err := (&MF4Parser{l: silentLogger()}).Parse(path)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, int64(0), entries[0].OffsetNs)
	assert.Equal(t, int64(100*time.Millisecond), entries[1].OffsetNs)
}

func TestMF4Parser_Parse_MissingFile(t *testing.T) {
	_, err := (&MF4Parser{l: silentLogger()}).Parse("/no/such/file.mf4")
	assert.Error(t, err)
}

// --- DetectParser for MF4 ----------------------------------------------------

func TestDetectParser_MF4_Unfinalized(t *testing.T) {
	path := twoFrameMF4(t)
	parser, err := DetectParser(path, silentLogger())
	require.NoError(t, err)
	assert.IsType(t, &MF4Parser{}, parser)
}

func TestDetectParser_MF4_Finalized(t *testing.T) {
	// Build an MF4 with finalized magic
	b := &mf4Builder{frames: nil}
	data := b.build()
	copy(data[0:8], "MDF     ")                   // finalized magic
	binary.LittleEndian.PutUint16(data[60:62], 0) // clear unfinalized flag
	path := writeMF4Temp(t, data)
	parser, err := DetectParser(path, silentLogger())
	require.NoError(t, err)
	assert.IsType(t, &MF4Parser{}, parser)
}

// --- Receive integration with MF4 -------------------------------------------

func TestPlaybackCanClient_Receive_MF4(t *testing.T) {
	ch := testChannel()
	path := twoFrameMF4(t)
	conn := NewPlaybackCanClient(ctx(), testConfig(), "test", ch, path, false, silentLogger(), nil, nil, nil)
	receiveAndWait(conn)

	require.Equal(t, 2, len(ch))
	first := <-ch
	second := <-ch
	assert.Equal(t, uint32(0x123), first.ID)
	assert.Equal(t, uint32(0x456), second.ID)
	assert.True(t, second.Transmit)
}

// helper to avoid importing context everywhere
func ctx() context.Context { return context.Background() }
