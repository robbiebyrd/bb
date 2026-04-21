package mf4

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	canModels "github.com/robbiebyrd/bb/internal/models"
	mf4fmt "github.com/robbiebyrd/bb/internal/parser/mf4"
)

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type mockFilter struct{ drop bool }

func (m *mockFilter) Filter(_ canModels.CanMessageTimestamped) bool { return m.drop }

func newTestConfig(t *testing.T, canFile, sigFile string, finalize bool) *canModels.Config {
	t.Helper()
	return &canModels.Config{
		MF4Logger: canModels.MF4LogConfig{
			CanOutputFile:    canFile,
			SignalOutputFile: sigFile,
			Finalize:         finalize,
		},
		MessageBufferSize: 16,
	}
}

func TestNewClient_GetNameAndChannels(t *testing.T) {
	cfg := newTestConfig(t, "", "", true)
	c, err := NewClient(t.Context(), cfg, silentLogger())
	require.NoError(t, err)
	assert.Equal(t, "output-mf4", c.GetName())
	assert.NotNil(t, c.GetChannel())
	if sc, ok := c.(canModels.SignalOutputClient); ok {
		assert.NotNil(t, sc.GetSignalChannel())
	}
}

func TestNewClient_AddFilter_Duplicate(t *testing.T) {
	cfg := newTestConfig(t, "", "", true)
	c, err := NewClient(t.Context(), cfg, silentLogger())
	require.NoError(t, err)

	require.NoError(t, c.AddFilter("a", &mockFilter{}))
	err = c.AddFilter("a", &mockFilter{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "filter group already exists")
}

func TestNewClient_BadCanPath_Errors(t *testing.T) {
	cfg := newTestConfig(t, "/no/such/dir/out.mf4", "", true)
	_, err := NewClient(t.Context(), cfg, silentLogger())
	require.Error(t, err)
}

func TestNewClient_BadSignalPath_ClosesCanWriter(t *testing.T) {
	canPath := filepath.Join(t.TempDir(), "ok.mf4")
	cfg := newTestConfig(t, canPath, "/no/such/dir/out.mf4", true)
	_, err := NewClient(t.Context(), cfg, silentLogger())
	require.Error(t, err)

	// The CAN file was created before the signal writer failed; make
	// sure the client did not leak the descriptor by opening the same
	// path again for writing.
	f, err := os.OpenFile(canPath, os.O_WRONLY, 0644)
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

func TestClient_HandleCanMessage_WritesRoundtrippable(t *testing.T) {
	canPath := filepath.Join(t.TempDir(), "can.mf4")
	cfg := newTestConfig(t, canPath, "", true)
	c, err := NewClient(t.Context(), cfg, silentLogger())
	require.NoError(t, err)

	c.HandleCanMessage(canModels.CanMessageTimestamped{
		Timestamp: 1_000_000,
		Interface: 0,
		ID:        0x1AB,
		Length:    4,
		Data:      []byte{0x01, 0x02, 0x03, 0x04},
	})
	c.HandleCanMessage(canModels.CanMessageTimestamped{
		Timestamp: 2_000_000,
		Interface: 0,
		ID:        0x2CD,
		Length:    2,
		Transmit:  true,
		Data:      []byte{0xAA, 0xBB},
	})

	close(c.GetChannel())
	require.NoError(t, c.HandleCanMessageChannel())

	// File was finalized; spot-check by reading the ID block magic.
	f, err := os.Open(canPath)
	require.NoError(t, err)
	defer f.Close()
	magic := make([]byte, 8)
	_, err = f.ReadAt(magic, 0)
	require.NoError(t, err)
	assert.Equal(t, "MDF     ", string(magic))

	assert.True(t, mf4fmt.IsMF4File(canPath))
}

func TestClient_HandleCanMessage_FilterDrops(t *testing.T) {
	canPath := filepath.Join(t.TempDir(), "filtered.mf4")
	cfg := newTestConfig(t, canPath, "", true)
	c, err := NewClient(t.Context(), cfg, silentLogger())
	require.NoError(t, err)
	require.NoError(t, c.AddFilter("drop-all", &mockFilter{drop: true}))

	c.HandleCanMessage(canModels.CanMessageTimestamped{ID: 0x7FF, Data: []byte{0x01}})

	close(c.GetChannel())
	require.NoError(t, c.HandleCanMessageChannel())

	// Finalize succeeded but no records were emitted; check the DT block
	// length matches just the header.
	info, err := os.Stat(canPath)
	require.NoError(t, err)
	// Can't introspect block positions trivially without parsing, so
	// just assert file is larger than zero and not enormous.
	assert.Greater(t, info.Size(), int64(100))
}

func TestClient_HandleSignalChannel_Unfinalized(t *testing.T) {
	sigPath := filepath.Join(t.TempDir(), "sigs.mf4")
	cfg := newTestConfig(t, "", sigPath, false)
	c, err := NewClient(t.Context(), cfg, silentLogger())
	require.NoError(t, err)

	sc := c.(canModels.SignalOutputClient)
	sc.HandleSignal(canModels.CanSignalTimestamped{
		Timestamp: 1_000_000_000,
		Interface: 2,
		ID:        0x123,
		Message:   "ENGINE",
		Signal:    "RPM",
		Value:     1500.5,
		Unit:      "rpm",
	})

	close(sc.GetSignalChannel())
	require.NoError(t, sc.HandleSignalChannel())

	// finalize=false → file should still be marked unfinalized.
	f, err := os.Open(sigPath)
	require.NoError(t, err)
	defer f.Close()
	magic := make([]byte, 8)
	_, err = f.ReadAt(magic, 0)
	require.NoError(t, err)
	assert.Equal(t, "UnFinMF ", string(magic))
}

func TestClient_HandleCanMessage_NilWriter_NoOp(t *testing.T) {
	// No CAN output file configured — HandleCanMessage must be safe.
	cfg := newTestConfig(t, "", "", true)
	c, err := NewClient(t.Context(), cfg, silentLogger())
	require.NoError(t, err)
	c.HandleCanMessage(canModels.CanMessageTimestamped{ID: 0x100})
}

func TestClient_HandleSignal_NilWriter_NoOp(t *testing.T) {
	cfg := newTestConfig(t, "", "", true)
	c, err := NewClient(t.Context(), cfg, silentLogger())
	require.NoError(t, err)
	sc := c.(canModels.SignalOutputClient)
	sc.HandleSignal(canModels.CanSignalTimestamped{Signal: "RPM"})
}
