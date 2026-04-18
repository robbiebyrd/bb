package crtd

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

// mockFilter implements canModels.FilterInterface for testing.
type mockFilter struct {
	filters []canModels.CanMessageFilter
	mode    canModels.CanFilterGroupOperator
}

func (m *mockFilter) Add(filter canModels.CanMessageFilter) error {
	m.filters = append(m.filters, filter)
	return nil
}

func (m *mockFilter) Filter(_ canModels.CanMessageTimestamped) bool {
	return false
}

func (m *mockFilter) Mode(mode canModels.CanFilterGroupOperator) {
	m.mode = mode
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTestClient creates a CRTDLoggerClient backed by a temp file for testing.
func newTestClient(t *testing.T) (*CRTDLoggerClient, *os.File) {
	t.Helper()
	f, err := os.CreateTemp("", "crtd_test_*.crtd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })

	return &CRTDLoggerClient{
		w:       bufio.NewWriter(f),
		file:    f,
		c:       make(chan canModels.CanMessageTimestamped, 16),
		filters: make(map[string]canModels.FilterInterface),
		l:       silentLogger(),
	}, f
}

func readFileContents(t *testing.T, f *os.File) string {
	t.Helper()
	data, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestNewClient(t *testing.T) {
	f, err := os.CreateTemp("", "crtd_newclient_*.crtd")
	if err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	f.Close()
	t.Cleanup(func() { os.Remove(name) })

	ctx := context.Background()
	cfg := &canModels.Config{
		CRTDLogger:        canModels.CRTDLogConfig{OutputFile: name},
		MessageBufferSize: 16,
	}

	client, err := NewClient(ctx, cfg, silentLogger())
	require.NoError(t, err)
	assert.NotNil(t, client, "NewClient should return a non-nil client")

	data, err := os.ReadFile(name)
	assert.Nil(t, err, "Should be able to read the output file")
	assert.Contains(
		t,
		string(data),
		"0.000000 CXX CRTD file created by bb",
		"File should contain CRTD header",
	)
}

func TestNewClient_BadPath_ReturnsError(t *testing.T) {
	ctx := context.Background()
	cfg := &canModels.Config{
		CRTDLogger:        canModels.CRTDLogConfig{OutputFile: "/nonexistent/path/to/file.crtd"},
		MessageBufferSize: 16,
	}
	_, err := NewClient(ctx, cfg, silentLogger())
	assert.Error(t, err, "NewClient must return an error when the file cannot be opened")
}

func TestHandle_StandardMessage(t *testing.T) {
	client, f := newTestClient(t)

	msg := canModels.CanMessageTimestamped{
		Timestamp: 1000000000, // 1 second in nanoseconds
		Interface: 0,
		ID:        0x123,
		Transmit:  false,
		Data:      []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}

	client.HandleCanMessage(msg)
	_ = client.w.Flush()

	contents := readFileContents(t, f)
	assert.Contains(
		t,
		contents,
		"1.000000 0R11 123 DE AD BE EF",
		"Should format standard RX 11-bit message correctly",
	)
}

func TestHandle_TransmitMessage(t *testing.T) {
	client, f := newTestClient(t)

	msg := canModels.CanMessageTimestamped{
		Timestamp: 2000000000,
		ID:        0x100,
		Transmit:  true,
		Data:      []byte{0x01},
	}

	client.HandleCanMessage(msg)
	_ = client.w.Flush()

	contents := readFileContents(t, f)
	assert.Contains(t, contents, "T11", "Transmit message should have record type starting with T")
}

func TestHandle_Extended29BitID(t *testing.T) {
	client, f := newTestClient(t)

	msg := canModels.CanMessageTimestamped{
		Timestamp: 3000000000,
		ID:        0x800, // > 0x7FF
		Transmit:  false,
		Data:      []byte{0xAA},
	}

	client.HandleCanMessage(msg)
	_ = client.w.Flush()

	contents := readFileContents(t, f)
	assert.Contains(t, contents, "R29", "Extended ID message should have record type R29")
}

func TestHandle_TransmitExtended(t *testing.T) {
	client, f := newTestClient(t)

	msg := canModels.CanMessageTimestamped{
		Timestamp: 4000000000,
		ID:        0x1FFFFFFF,
		Transmit:  true,
		Data:      []byte{0xFF},
	}

	client.HandleCanMessage(msg)
	_ = client.w.Flush()

	contents := readFileContents(t, f)
	assert.Contains(t, contents, "T29", "Transmit extended message should have record type T29")
}

func TestHandle_TimestampConversion(t *testing.T) {
	client, f := newTestClient(t)

	// 5 seconds + 123456 microseconds = 5000123456000 nanoseconds
	msg := canModels.CanMessageTimestamped{
		Timestamp: 5000123456000,
		ID:        0x01,
		Data:      []byte{0x00},
	}

	client.HandleCanMessage(msg)
	_ = client.w.Flush()

	contents := readFileContents(t, f)
	assert.Contains(
		t,
		contents,
		"5000.123456",
		"Timestamp should be converted to seconds.microseconds",
	)
}

func TestHandle_EmptyData(t *testing.T) {
	client, f := newTestClient(t)

	msg := canModels.CanMessageTimestamped{
		Timestamp: 1000000000,
		ID:        0x7FF,
		Data:      []byte{},
	}

	client.HandleCanMessage(msg)
	_ = client.w.Flush()

	contents := readFileContents(t, f)
	lines := strings.Split(strings.TrimSpace(contents), "\n")
	assert.Equal(t, 1, len(lines), "Should have exactly one line")
	assert.Contains(
		t,
		lines[0],
		"0R11 7FF",
		"Should contain the interface ID, record type, and CAN ID",
	)
}

func TestAddFilter(t *testing.T) {
	client, _ := newTestClient(t)

	filter := &mockFilter{}
	err := client.AddFilter("test-filter", filter)
	assert.Nil(t, err, "First AddFilter call should succeed")

	err = client.AddFilter("test-filter", filter)
	assert.NotNil(t, err, "Duplicate filter name should return an error")
	assert.Contains(
		t,
		err.Error(),
		"filter group already exists",
		"Error message should mention duplicate filter group",
	)

	err = client.AddFilter("another-filter", filter)
	assert.Nil(t, err, "Adding a filter with a different name should succeed")
}

func TestGetName(t *testing.T) {
	client, _ := newTestClient(t)
	assert.Equal(t, "output-crtd", client.GetName(), "GetName should return output-crtd")
}

func TestGetChannel(t *testing.T) {
	client, _ := newTestClient(t)
	ch := client.GetChannel()
	assert.NotNil(t, ch, "GetChannel should return a non-nil channel")
}

func TestHandleChannel(t *testing.T) {
	client, f := newTestClient(t)

	msgs := []canModels.CanMessageTimestamped{
		{Timestamp: 1000000000, ID: 0x100, Data: []byte{0x01}},
		{Timestamp: 2000000000, ID: 0x200, Data: []byte{0x02}},
		{Timestamp: 3000000000, ID: 0x300, Data: []byte{0x03}},
	}

	go func() {
		for _, msg := range msgs {
			client.c <- msg
		}
		close(client.c)
	}()

	err := client.HandleCanMessageChannel()
	assert.Nil(t, err, "HandleChannel should return nil after channel is closed")

	contents := readFileContents(t, f)
	lines := strings.Split(strings.TrimSpace(contents), "\n")
	assert.Equal(t, 3, len(lines), "Should have written 3 lines, one per message")
	assert.Contains(t, lines[0], "100", "First line should contain ID 100")
	assert.Contains(t, lines[1], "200", "Second line should contain ID 200")
	assert.Contains(t, lines[2], "300", "Third line should contain ID 300")
}

func TestRun(t *testing.T) {
	client, _ := newTestClient(t)
	err := client.Run()
	assert.Nil(t, err, "Run should return nil")
}

// TestHandleChannel_DataWrittenAfterChannelClose verifies that all data is
// flushed and readable after the channel is closed, without relying on
// per-message flushes.
func TestHandleChannel_DataWrittenAfterChannelClose(t *testing.T) {
	client, f := newTestClient(t)

	msgs := []canModels.CanMessageTimestamped{
		{Timestamp: 1000000000, ID: 0x111, Data: []byte{0xAA}},
		{Timestamp: 2000000000, ID: 0x222, Data: []byte{0xBB}},
	}

	go func() {
		for _, msg := range msgs {
			client.c <- msg
		}
		close(client.c)
	}()

	err := client.HandleCanMessageChannel()
	assert.Nil(t, err)

	contents := readFileContents(t, f)
	assert.Contains(t, contents, "111", "First message ID should be written and flushed")
	assert.Contains(t, contents, "222", "Second message ID should be written and flushed")
}

// alwaysFailWriter rejects every write with an error.
type alwaysFailWriter struct{}

func (alwaysFailWriter) Write(_ []byte) (int, error) {
	return 0, io.ErrClosedPipe
}

// TestNewClient_HeaderFirstLineErrorLogged verifies that a write error on the
// first header line is surfaced (not silently overwritten by a later write).
func TestNewClient_HeaderFirstLineErrorLogged(t *testing.T) {
	var buf strings.Builder
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})
	logger := slog.New(handler)

	cfg := &canModels.Config{
		CanInterfaces: []canModels.CanInterfaceOption{{Name: "can0"}},
	}

	// A 1-byte bufio.Writer backed by an always-failing writer forces the first
	// multi-byte write to overflow into the underlying writer, surfacing the error.
	w := bufio.NewWriterSize(alwaysFailWriter{}, 1)
	writeHeader(w, cfg, logger)

	assert.Contains(t, buf.String(), "Could not", "Header write error must be logged")
}
