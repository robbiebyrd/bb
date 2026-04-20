package csv

import (
	"encoding/csv"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

// mockCanConn implements canModels.CanConnection for testing.
type mockCanConn struct {
	interfaceName string
}

func (m *mockCanConn) SetID(_ int)                {}
func (m *mockCanConn) GetName() string            { return "" }
func (m *mockCanConn) GetInterfaceName() string   { return m.interfaceName }
func (m *mockCanConn) Open() error                { return nil }
func (m *mockCanConn) Close() error               { return nil }
func (m *mockCanConn) Receive(_ *sync.WaitGroup)  {}

// mockResolver implements canModels.InterfaceResolver for testing.
type mockResolver struct {
	conns map[int]*mockCanConn
}

func (r *mockResolver) ConnectionByID(id int) canModels.CanConnection {
	if c, ok := r.conns[id]; ok {
		return c
	}
	return nil
}

// mockFilter implements canModels.FilterInterface for testing.
type mockFilter struct{}

func (m *mockFilter) Filter(_ canModels.CanMessageTimestamped) bool { return false }

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTestClient builds a CSVClient backed by a temp file.
func newTestClient(t *testing.T, resolver canModels.InterfaceResolver) (*CSVClient, *os.File) {
	t.Helper()
	f, err := os.CreateTemp("", "csv_test_*.csv")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(f.Name()) })

	return &CSVClient{
		w:             csv.NewWriter(f),
		file:          f,
		canChannel:    make(chan canModels.CanMessageTimestamped, 16),
		signalChannel: make(chan canModels.CanSignalTimestamped, 16),
		filters:       make(map[string]canModels.FilterInterface),
		l:             silentLogger(),
		resolver:      resolver,
	}, f
}

// newSignalTestClient builds a CSVClient with both CAN and signal files.
func newSignalTestClient(t *testing.T, resolver canModels.InterfaceResolver) (*CSVClient, *os.File, *os.File) {
	t.Helper()
	canFile, err := os.CreateTemp("", "csv_can_*.csv")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(canFile.Name()) })

	sigFile, err := os.CreateTemp("", "csv_sig_*.csv")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(sigFile.Name()) })

	return &CSVClient{
		w:             csv.NewWriter(canFile),
		file:          canFile,
		signalWriter:  csv.NewWriter(sigFile),
		signalFile:    sigFile,
		canChannel:    make(chan canModels.CanMessageTimestamped, 16),
		signalChannel: make(chan canModels.CanSignalTimestamped, 16),
		filters:       make(map[string]canModels.FilterInterface),
		l:             silentLogger(),
		resolver:      resolver,
	}, canFile, sigFile
}

func readRows(t *testing.T, f *os.File) [][]string {
	t.Helper()
	_, err := f.Seek(0, io.SeekStart)
	require.NoError(t, err)
	rows, err := csv.NewReader(f).ReadAll()
	require.NoError(t, err)
	return rows
}

func readRowsByName(t *testing.T, name string) [][]string {
	t.Helper()
	f, err := os.Open(name)
	require.NoError(t, err)
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	require.NoError(t, err)
	return rows
}

func TestCSVClient_GetName(t *testing.T) {
	client, _ := newTestClient(t, &mockResolver{})
	assert.Equal(t, "output-csv", client.GetName())
}

func TestCSVClient_GetChannel(t *testing.T) {
	client, _ := newTestClient(t, &mockResolver{})
	assert.NotNil(t, client.GetChannel())
}

func TestCSVClient_AddFilter(t *testing.T) {
	client, _ := newTestClient(t, &mockResolver{})
	f := &mockFilter{}

	err := client.AddFilter("my-filter", f)
	assert.NoError(t, err)

	err = client.AddFilter("my-filter", f)
	assert.Error(t, err, "duplicate filter name should return an error")
	assert.Contains(t, err.Error(), "filter group already exists")

	err = client.AddFilter("another-filter", f)
	assert.NoError(t, err)
}

func TestCSVClient_Handle_RowFormat(t *testing.T) {
	resolver := &mockResolver{
		conns: map[int]*mockCanConn{
			0: {interfaceName: "can0-can-vcan0"},
		},
	}
	client, f := newTestClient(t, resolver)

	msg := canModels.CanMessageTimestamped{
		Timestamp: 1000000000,
		Interface: 0,
		ID:        0x1AB,
		Remote:    false,
		Transmit:  true,
		Length:    4,
		Data:      []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}
	client.HandleCanMessage(msg)
	client.w.Flush()

	rows := readRows(t, f)
	require.Len(t, rows, 1)
	row := rows[0]

	assert.Equal(t, "1000000000", row[0], "timestamp")
	assert.Equal(t, "427", row[1], "id (decimal of 0x1AB)")
	assert.Equal(t, "can0-can-vcan0", row[2], "interface name from resolver")
	assert.Equal(t, "false", row[3], "remote")
	assert.Equal(t, "true", row[4], "transmit")
	assert.Equal(t, "4", row[5], "length")
	assert.Equal(t, "deadbeef", row[6], "data hex")
}

func TestCSVClient_Handle_UnknownInterface(t *testing.T) {
	client, f := newTestClient(t, &mockResolver{conns: map[int]*mockCanConn{}})

	msg := canModels.CanMessageTimestamped{
		Interface: 99,
		ID:        0x001,
		Data:      []byte{0x01},
	}
	client.HandleCanMessage(msg)
	client.w.Flush()

	rows := readRows(t, f)
	require.Len(t, rows, 1)
	assert.Equal(t, "", rows[0][2], "unknown interface should produce empty interface name")
}

func TestCSVClient_HandleChannel(t *testing.T) {
	resolver := &mockResolver{
		conns: map[int]*mockCanConn{0: {interfaceName: "iface0"}},
	}
	client, f := newTestClient(t, resolver)

	msgs := []canModels.CanMessageTimestamped{
		{ID: 0x100, Interface: 0, Data: []byte{0x01}},
		{ID: 0x200, Interface: 0, Data: []byte{0x02}},
		{ID: 0x300, Interface: 0, Data: []byte{0x03}},
	}

	go func() {
		for _, m := range msgs {
			client.canChannel <- m
		}
		close(client.canChannel)
	}()

	err := client.HandleCanMessageChannel()
	assert.NoError(t, err)

	// File is closed after HandleCanMessageChannel returns; read by name.
	rows := readRowsByName(t, f.Name())
	require.Len(t, rows, 3)
	assert.Equal(t, "256", rows[0][1])  // 0x100
	assert.Equal(t, "512", rows[1][1])  // 0x200
	assert.Equal(t, "768", rows[2][1])  // 0x300
}

func TestCSVClient_HandleCanMessageChannel_ClosesFile(t *testing.T) {
	client, f := newTestClient(t, &mockResolver{})

	close(client.canChannel)
	err := client.HandleCanMessageChannel()
	assert.NoError(t, err)

	// The file handle should be closed; a second Close call returns an error.
	assert.Error(t, f.Close(), "file should already be closed after HandleCanMessageChannel returns")
}

func newTestConfig(t *testing.T, includeHeaders bool) *canModels.Config {
	t.Helper()
	f, err := os.CreateTemp("", "csv_newclient_*.csv")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	t.Cleanup(func() { os.Remove(f.Name()) })
	return &canModels.Config{
		CSVLog: canModels.CSVLogConfig{
			CanOutputFile:  f.Name(),
			IncludeHeaders: includeHeaders,
		},
		MessageBufferSize: 16,
	}
}

func TestNewClient_IncludeHeaders_True(t *testing.T) {
	cfg := newTestConfig(t, true)
	client, err := NewClient(t.Context(), cfg, silentLogger(), &mockResolver{})
	require.NoError(t, err)
	csvClient := client.(*CSVClient)
	csvClient.w.Flush()

	rows := readRowsByName(t, cfg.CSVLog.CanOutputFile)
	require.Len(t, rows, 1, "header row should be written when IncludeHeaders is true")
	assert.Equal(t, []string{"timestamp", "id", "interface", "remote", "transmit", "length", "data"}, rows[0])
}

func TestNewClient_IncludeHeaders_False(t *testing.T) {
	cfg := newTestConfig(t, false)
	client, err := NewClient(t.Context(), cfg, silentLogger(), &mockResolver{})
	require.NoError(t, err)
	csvClient := client.(*CSVClient)
	csvClient.w.Flush()

	rows := readRowsByName(t, cfg.CSVLog.CanOutputFile)
	assert.Empty(t, rows, "no header row should be written when IncludeHeaders is false")
}

func TestCSVClient_GetSignalChannel(t *testing.T) {
	client, _, _ := newSignalTestClient(t, &mockResolver{})
	assert.NotNil(t, client.GetSignalChannel())
}

func TestCSVClient_HandleSignal_NilWriter(t *testing.T) {
	// When no signal file is configured, HandleSignal must be a no-op.
	client, _ := newTestClient(t, &mockResolver{})
	client.HandleSignal(canModels.CanSignalTimestamped{Signal: "RPM", Value: 1000})
}

func TestCSVClient_HandleSignal_WritesRow(t *testing.T) {
	resolver := &mockResolver{
		conns: map[int]*mockCanConn{0: {interfaceName: "can0-can-vcan0"}},
	}
	client, _, sigFile := newSignalTestClient(t, resolver)

	sig := canModels.CanSignalTimestamped{
		Timestamp: 1000000000,
		Interface: 0,
		Message:   "ENGINE",
		Signal:    "RPM",
		Value:     1500.5,
		Unit:      "rpm",
	}
	client.HandleSignal(sig)
	client.signalWriter.Flush()

	rows := readRows(t, sigFile)
	require.Len(t, rows, 1)
	assert.Equal(t, "1000000000", rows[0][0], "timestamp")
	assert.Equal(t, "can0-can-vcan0", rows[0][1], "interface")
	assert.Equal(t, "ENGINE", rows[0][2], "message")
	assert.Equal(t, "RPM", rows[0][3], "signal")
	assert.Equal(t, "1500.5", rows[0][4], "value")
	assert.Equal(t, "rpm", rows[0][5], "unit")
}

func TestCSVClient_HandleSignalChannel(t *testing.T) {
	resolver := &mockResolver{
		conns: map[int]*mockCanConn{0: {interfaceName: "iface0"}},
	}
	client, _, sigFile := newSignalTestClient(t, resolver)

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

	rows := readRowsByName(t, sigFile.Name())
	require.Len(t, rows, 2)
	assert.Equal(t, "RPM", rows[0][3])
	assert.Equal(t, "TEMP", rows[1][3])
}

func TestCSVClient_HandleSignalChannel_WritesHeader(t *testing.T) {
	client, _, sigFile := newSignalTestClient(t, &mockResolver{})

	// Signal header is written when IncludeHeaders is set.
	client.includeHeaders = true
	client.signalWriter.Write([]string{"timestamp", "interface", "message", "signal", "value", "unit"}) //nolint:errcheck
	client.signalWriter.Flush()

	rows := readRows(t, sigFile)
	require.Len(t, rows, 1)
	assert.Equal(t, "timestamp", rows[0][0])
}
