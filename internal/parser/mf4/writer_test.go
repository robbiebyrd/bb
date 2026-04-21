package mf4

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tempFile returns a freshly-created temp file path the test will clean up.
func tempFile(t *testing.T, suffix string) string {
	t.Helper()
	f, err := os.CreateTemp("", "mf4_writer_test_*"+suffix)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

// readBlockID returns the first 4 bytes at offset addr.
func readBlockID(t *testing.T, path string, addr int64) string {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	buf := make([]byte, 4)
	_, err = f.ReadAt(buf, addr)
	require.NoError(t, err)
	return string(buf)
}

func TestNewCANWriter_WritesValidHeaders(t *testing.T) {
	path := tempFile(t, ".mf4")
	w, err := NewCANWriter(path)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	assert.True(t, IsMF4File(path), "file should be recognised as MF4")

	info, err := os.Stat(path)
	require.NoError(t, err)
	// ID(64) + HD(96) + 3xTX + CG1(104) + CG2(104) + 2xCN(136) + DG(64) + DT(24) = at least 696 bytes.
	assert.Greater(t, info.Size(), int64(600))

	// HD block sits at IDBlockSize.
	assert.Equal(t, "##HD", readBlockID(t, path, int64(IDBlockSize)))
}

func TestCANWriter_FinalizedAfterClose(t *testing.T) {
	path := tempFile(t, ".mf4")
	w, err := NewCANWriter(path)
	require.NoError(t, err)
	require.NoError(t, w.AppendCAN(0, 0x123, false, []byte{0x01, 0x02, 0x03}))
	require.NoError(t, w.Close())

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	var magic [8]byte
	_, err = f.ReadAt(magic[:], 0)
	require.NoError(t, err)
	assert.Equal(t, "MDF     ", string(magic[:]), "finalized file should carry MDF magic")

	var flag [2]byte
	_, err = f.ReadAt(flag[:], 60)
	require.NoError(t, err)
	assert.Equal(t, uint16(0), binary.LittleEndian.Uint16(flag[:]), "unfinalized flag should be cleared")
}

func TestCANWriter_UnfinalizedClose_PreservesMagic(t *testing.T) {
	path := tempFile(t, ".mf4")
	w, err := NewCANWriter(path)
	require.NoError(t, err)
	require.NoError(t, w.AppendCAN(0, 0x001, false, []byte{0x00}))
	require.NoError(t, w.CloseUnfinalized())

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	var magic [8]byte
	_, err = f.ReadAt(magic[:], 0)
	require.NoError(t, err)
	assert.Equal(t, "UnFinMF ", string(magic[:]))
}

func TestCANWriter_CloseIsIdempotent(t *testing.T) {
	path := tempFile(t, ".mf4")
	w, err := NewCANWriter(path)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, w.Close(), "second Close should be a no-op")
}

func TestSignalWriter_BasicRoundtripFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "signals.mf4")

	w, err := NewSignalWriter(path)
	require.NoError(t, err)
	require.NoError(t, w.AppendSignal(1_000_000, 0x101, 2, 42.5, "ENGINE", "RPM", "rpm"))
	require.NoError(t, w.Close())

	// Read through the block layer to confirm the CG / CN structure is
	// intact; we don't fully decode the record here, that's covered by
	// the reader roundtrip test in playback.
	assert.True(t, IsMF4File(path))
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	hdLinks, _, err := ReadBlock(f, int64(IDBlockSize))
	require.NoError(t, err)
	require.NotEmpty(t, hdLinks)
	dgLinks, dgData, err := ReadBlock(f, hdLinks[0])
	require.NoError(t, err)
	assert.EqualValues(t, 1, dgData[0], "rec id size")
	cgAddr := dgLinks[1]
	cgLinks, cgData, err := ReadBlock(f, cgAddr)
	require.NoError(t, err)
	acq := ReadText(f, cgLinks[2])
	assert.Equal(t, "Signal", acq)
	recordID := binary.LittleEndian.Uint64(cgData[0:8])
	assert.Equal(t, uint64(RecordIDPrimary), recordID)
	assert.Equal(t, uint32(SignalRecordSize), binary.LittleEndian.Uint32(cgData[24:28]))
}

func TestBuildTXBlock_HasCorrectHeaderAndPadding(t *testing.T) {
	b := BuildTXBlock("hello")
	require.Equal(t, "##TX", string(b[0:4]))
	blockLen := binary.LittleEndian.Uint64(b[8:16])
	assert.EqualValues(t, len(b), blockLen)
	assert.Zero(t, blockLen%8, "block length must be 8-byte aligned")
}

func TestAppendCAN_RejectsOversizedPayload(t *testing.T) {
	path := tempFile(t, ".mf4")
	w, err := NewCANWriter(path)
	require.NoError(t, err)
	defer w.CloseUnfinalized()

	err = w.AppendCAN(0, 0x100, false, make([]byte, MaxCANDataLen+1))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCANDataTooLong))

	// Exactly at the limit must still succeed.
	require.NoError(t, w.AppendCAN(0, 0x100, false, make([]byte, MaxCANDataLen)))
}

func TestAppendSignal_RejectsNULInLabel(t *testing.T) {
	path := tempFile(t, ".mf4")
	w, err := NewSignalWriter(path)
	require.NoError(t, err)
	defer w.CloseUnfinalized()

	err = w.AppendSignal(0, 0x100, 0, 1.0, "mes\x00sage", "sig", "u")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSignalLabelHasNUL))
}

func TestEncodeCANFrameRecord_RoundTripFields(t *testing.T) {
	rec := make([]byte, CANRecordSize)
	EncodeCANFrameRecord(rec, 12345, 0x7E8, true, 8, 42)

	// Timestamp (float64 LE of 12345).
	tsBits := binary.LittleEndian.Uint64(rec[0:8])
	assert.Equal(t, uint64(0x40C81C8000000000), tsBits)

	// CAN ID sits in bits 2..30 of the LE uint32 at [8:12].
	idField := binary.LittleEndian.Uint32(rec[8:12])
	assert.Equal(t, uint32(0x7E8)<<2, idField&0x7FFFFFFC)

	// Dir bit and dataLength share byte 4.
	assert.Equal(t, byte(1|(8<<1)), rec[12])

	// VLSD offset lives at bytes 6..13 of the CAN composite (rec[14:22]).
	off := binary.LittleEndian.Uint64(rec[14:22])
	assert.Equal(t, uint64(42), off)
}
