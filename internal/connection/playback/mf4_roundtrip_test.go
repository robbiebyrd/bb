package playback

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mf4fmt "github.com/robbiebyrd/bb/internal/parser/mf4"
)

// TestMF4Writer_RoundTripThroughParser asserts that a file produced by
// mf4fmt.NewCANWriter can be consumed by the playback MF4Parser — the
// writer is the inverse of the reader by construction, and this test
// pins that contract.
func TestMF4Writer_RoundTripThroughParser(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roundtrip.mf4")

	w, err := mf4fmt.NewCANWriter(path)
	require.NoError(t, err)

	frames := []struct {
		ts   int64
		id   uint32
		tx   bool
		data []byte
	}{
		{0, 0x123, false, []byte{0xDE, 0xAD, 0xBE, 0xEF}},
		{50_000_000, 0x456, true, []byte{0xCA, 0xFE}},
		{120_000_000, 0x1ABCDEF, false, []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88}},
	}
	for _, fr := range frames {
		require.NoError(t, w.AppendCAN(fr.ts, fr.id, fr.tx, fr.data))
	}
	require.NoError(t, w.Close())

	entries, err := (&MF4Parser{l: silentLogger()}).Parse(path)
	require.NoError(t, err)
	require.Len(t, entries, len(frames))

	assert.Equal(t, int64(0), entries[0].OffsetNs)
	assert.Equal(t, uint32(0x123), entries[0].ID)
	assert.False(t, entries[0].Transmit)
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF}, entries[0].Data)

	assert.Equal(t, int64(50_000_000), entries[1].OffsetNs)
	assert.Equal(t, uint32(0x456), entries[1].ID)
	assert.True(t, entries[1].Transmit)
	assert.Equal(t, []byte{0xCA, 0xFE}, entries[1].Data)

	assert.Equal(t, int64(120_000_000), entries[2].OffsetNs)
	assert.Equal(t, uint32(0x1ABCDEF), entries[2].ID)
	assert.Equal(t, []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88}, entries[2].Data)
}

// TestMF4Writer_UnfinalizedRoundTrip confirms the reader's unfinalized
// code path still works when the writer is closed via CloseUnfinalized.
func TestMF4Writer_UnfinalizedRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unfin.mf4")

	w, err := mf4fmt.NewCANWriter(path)
	require.NoError(t, err)
	require.NoError(t, w.AppendCAN(0, 0x7FF, false, []byte{0xAA, 0xBB}))
	require.NoError(t, w.CloseUnfinalized())

	assert.True(t, mf4fmt.IsMF4File(path))
	entries, err := (&MF4Parser{l: silentLogger()}).Parse(path)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, uint32(0x7FF), entries[0].ID)
	assert.Equal(t, []byte{0xAA, 0xBB}, entries[0].Data)
}
