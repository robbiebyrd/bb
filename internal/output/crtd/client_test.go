package crtd

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/robbiebyrd/cantou/internal/client/common"
	canModels "github.com/robbiebyrd/cantou/internal/models"
	crtdfmt "github.com/robbiebyrd/cantou/internal/parser/crtd"
)

// mockFilter implements canModels.FilterInterface for testing.
type mockFilter struct{}

func (m *mockFilter) Filter(_ canModels.CanMessageTimestamped) bool {
	return false
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func tempPath(t *testing.T, suffix string) string {
	t.Helper()
	f, err := os.CreateTemp("", "crtd_test_*"+suffix)
	require.NoError(t, err)
	name := f.Name()
	require.NoError(t, f.Close())
	t.Cleanup(func() { os.Remove(name) })
	return name
}

// newTestClient creates a CRTDLoggerClient backed by a temp file for testing.
func newTestClient(t *testing.T) (*CRTDLoggerClient, string) {
	t.Helper()
	path := tempPath(t, ".crtd")

	canWriter, err := crtdfmt.NewCANWriter(path, &canModels.Config{})
	require.NoError(t, err)

	return &CRTDLoggerClient{
		canWriter:     canWriter,
		canChannel:    make(chan canModels.CanMessageTimestamped, 16),
		signalChannel: make(chan canModels.CanSignalTimestamped, 16),
		filters:       common.NewFilterSet(),
		l:             silentLogger(),
	}, path
}

// newSignalTestClient creates a CRTDLoggerClient backed by both CAN and signal temp files.
func newSignalTestClient(t *testing.T) (*CRTDLoggerClient, string, string) {
	t.Helper()
	canPath := tempPath(t, ".crtd")
	sigPath := tempPath(t, ".crtd")

	canWriter, err := crtdfmt.NewCANWriter(canPath, &canModels.Config{})
	require.NoError(t, err)
	sigWriter, err := crtdfmt.NewSignalWriter(sigPath)
	require.NoError(t, err)

	return &CRTDLoggerClient{
		canWriter:     canWriter,
		signalWriter:  sigWriter,
		canChannel:    make(chan canModels.CanMessageTimestamped, 16),
		signalChannel: make(chan canModels.CanSignalTimestamped, 16),
		filters:       common.NewFilterSet(),
		l:             silentLogger(),
	}, canPath, sigPath
}

func readFileContents(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func TestNewClient(t *testing.T) {
	path := tempPath(t, ".crtd")

	ctx := context.Background()
	cfg := &canModels.Config{
		CRTDLogger:        canModels.CRTDLogConfig{CanOutputFile: path},
		MessageBufferSize: 16,
	}

	client, err := NewClient(ctx, cfg, silentLogger())
	require.NoError(t, err)
	assert.NotNil(t, client, "NewClient should return a non-nil client")

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "0.000000 CXX CRTD file created by cantou")
}

func TestNewClient_BadPath_ReturnsError(t *testing.T) {
	ctx := context.Background()
	cfg := &canModels.Config{
		CRTDLogger:        canModels.CRTDLogConfig{CanOutputFile: "/nonexistent/path/to/file.crtd"},
		MessageBufferSize: 16,
	}
	_, err := NewClient(ctx, cfg, silentLogger())
	assert.Error(t, err, "NewClient must return an error when the file cannot be opened")
}

func TestHandle_StandardMessage(t *testing.T) {
	client, path := newTestClient(t)

	msg := canModels.CanMessageTimestamped{
		Timestamp: 1000000000,
		Interface: 0,
		ID:        0x123,
		Transmit:  false,
		Data:      []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}

	client.HandleCanMessage(msg)
	require.NoError(t, client.canWriter.Flush())

	contents := readFileContents(t, path)
	assert.Contains(t, contents, "1.000000 0R11 123 DE AD BE EF")
}

func TestHandle_TransmitMessage(t *testing.T) {
	client, path := newTestClient(t)

	msg := canModels.CanMessageTimestamped{
		Timestamp: 2000000000,
		ID:        0x100,
		Transmit:  true,
		Data:      []byte{0x01},
	}

	client.HandleCanMessage(msg)
	require.NoError(t, client.canWriter.Flush())

	assert.Contains(t, readFileContents(t, path), "T11")
}

func TestHandle_Extended29BitID(t *testing.T) {
	client, path := newTestClient(t)

	msg := canModels.CanMessageTimestamped{
		Timestamp: 3000000000,
		ID:        0x800,
		Transmit:  false,
		Data:      []byte{0xAA},
	}

	client.HandleCanMessage(msg)
	require.NoError(t, client.canWriter.Flush())

	assert.Contains(t, readFileContents(t, path), "R29")
}

func TestHandle_TransmitExtended(t *testing.T) {
	client, path := newTestClient(t)

	msg := canModels.CanMessageTimestamped{
		Timestamp: 4000000000,
		ID:        0x1FFFFFFF,
		Transmit:  true,
		Data:      []byte{0xFF},
	}

	client.HandleCanMessage(msg)
	require.NoError(t, client.canWriter.Flush())

	assert.Contains(t, readFileContents(t, path), "T29")
}

func TestHandle_TimestampConversion(t *testing.T) {
	client, path := newTestClient(t)

	// 5 seconds + 123456 microseconds = 5000123456000 nanoseconds
	msg := canModels.CanMessageTimestamped{
		Timestamp: 5000123456000,
		ID:        0x01,
		Data:      []byte{0x00},
	}

	client.HandleCanMessage(msg)
	require.NoError(t, client.canWriter.Flush())

	assert.Contains(t, readFileContents(t, path), "5000.123456")
}

func TestHandle_EmptyData(t *testing.T) {
	client, path := newTestClient(t)

	msg := canModels.CanMessageTimestamped{
		Timestamp: 1000000000,
		ID:        0x7FF,
		Data:      []byte{},
	}

	client.HandleCanMessage(msg)
	require.NoError(t, client.canWriter.Flush())

	contents := readFileContents(t, path)
	lines := strings.Split(strings.TrimSpace(contents), "\n")
	assert.Equal(t, 2, len(lines), "should have header line and one CAN record")
	assert.Contains(t, lines[len(lines)-1], "0R11 7FF")
}

func TestAddFilter(t *testing.T) {
	client, _ := newTestClient(t)

	filter := &mockFilter{}
	err := client.AddFilter("test-filter", filter)
	assert.Nil(t, err)

	err = client.AddFilter("test-filter", filter)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "filter group already exists")

	err = client.AddFilter("another-filter", filter)
	assert.Nil(t, err)
}

func TestGetName(t *testing.T) {
	client, _ := newTestClient(t)
	assert.Equal(t, "output-crtd", client.GetName())
}

func TestGetChannel(t *testing.T) {
	client, _ := newTestClient(t)
	assert.NotNil(t, client.GetChannel())
}

func TestHandleChannel(t *testing.T) {
	client, path := newTestClient(t)

	msgs := []canModels.CanMessageTimestamped{
		{Timestamp: 1000000000, ID: 0x100, Data: []byte{0x01}},
		{Timestamp: 2000000000, ID: 0x200, Data: []byte{0x02}},
		{Timestamp: 3000000000, ID: 0x300, Data: []byte{0x03}},
	}

	go func() {
		for _, msg := range msgs {
			client.canChannel <- msg
		}
		close(client.canChannel)
	}()

	err := client.HandleCanMessageChannel()
	assert.Nil(t, err)

	contents := readFileContents(t, path)
	lines := strings.Split(strings.TrimSpace(contents), "\n")
	// First line is the CRTD header; remaining lines are the CAN records.
	records := lines[1:]
	require.Len(t, records, 3)
	assert.Contains(t, records[0], "100")
	assert.Contains(t, records[1], "200")
	assert.Contains(t, records[2], "300")
}

// TestHandleChannel_DataWrittenAfterChannelClose verifies that all data is
// flushed and readable after the channel is closed, without relying on
// per-message flushes.
func TestHandleChannel_DataWrittenAfterChannelClose(t *testing.T) {
	client, path := newTestClient(t)

	msgs := []canModels.CanMessageTimestamped{
		{Timestamp: 1000000000, ID: 0x111, Data: []byte{0xAA}},
		{Timestamp: 2000000000, ID: 0x222, Data: []byte{0xBB}},
	}

	go func() {
		for _, msg := range msgs {
			client.canChannel <- msg
		}
		close(client.canChannel)
	}()

	require.Nil(t, client.HandleCanMessageChannel())

	contents := readFileContents(t, path)
	assert.Contains(t, contents, "111")
	assert.Contains(t, contents, "222")
}

func TestCRTDClient_GetSignalChannel(t *testing.T) {
	client, _ := newTestClient(t)
	assert.NotNil(t, client.GetSignalChannel())
}

func TestCRTDClient_HandleSignal_NilWriter(t *testing.T) {
	client, _ := newTestClient(t)
	client.HandleSignal(canModels.CanSignalTimestamped{Signal: "RPM", Value: 1000})
}

func TestCRTDClient_HandleSignal_WritesLine(t *testing.T) {
	client, _, sigPath := newSignalTestClient(t)

	sig := canModels.CanSignalTimestamped{
		Timestamp: 1000000000,
		Interface: 0,
		Message:   "ENGINE",
		Signal:    "RPM",
		Value:     1500.5,
		Unit:      "rpm",
	}
	client.HandleSignal(sig)
	require.NoError(t, client.signalWriter.Flush())

	contents := readFileContents(t, sigPath)
	assert.Contains(t, contents, "1.000000")
	assert.Contains(t, contents, "SIG")
	assert.Contains(t, contents, "ENGINE/RPM")
	assert.Contains(t, contents, "1500.5")
	assert.Contains(t, contents, "rpm")
}

func TestCRTDClient_HandleSignalChannel(t *testing.T) {
	client, _, sigPath := newSignalTestClient(t)

	sigs := []canModels.CanSignalTimestamped{
		{Timestamp: 1000000000, Interface: 0, Message: "ENG", Signal: "RPM", Value: 1000, Unit: "rpm"},
		{Timestamp: 2000000000, Interface: 0, Message: "ENG", Signal: "TEMP", Value: 90, Unit: "C"},
	}
	go func() {
		for _, s := range sigs {
			client.signalChannel <- s
		}
		close(client.signalChannel)
	}()

	err := client.HandleSignalChannel()
	require.NoError(t, err)

	contents := readFileContents(t, sigPath)
	assert.Contains(t, contents, "RPM")
	assert.Contains(t, contents, "TEMP")
}
