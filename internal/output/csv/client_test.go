package csv

import (
	stdcsv "encoding/csv"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/robbiebyrd/cantou/internal/client/common"
	canModels "github.com/robbiebyrd/cantou/internal/models"
	csvfmt "github.com/robbiebyrd/cantou/internal/parser/csv"
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

// newTestClient builds a CSVClient backed by a temp file for CAN messages.
// Returns the client and the path of the temp file.
func newTestClient(t *testing.T, resolver canModels.InterfaceResolver) (*CSVClient, string) {
	t.Helper()
	f, err := os.CreateTemp("", "csv_test_*.csv")
	require.NoError(t, err)
	name := f.Name()
	require.NoError(t, f.Close())
	t.Cleanup(func() { os.Remove(name) })

	canWriter, err := csvfmt.NewCANWriter(name, false)
	require.NoError(t, err)

	return &CSVClient{
		canWriter:     canWriter,
		canChannel:    make(chan canModels.CanMessageTimestamped, 16),
		signalChannel: make(chan canModels.CanSignalTimestamped, 16),
		filters:       common.NewFilterSet(),
		l:             silentLogger(),
		resolver:      resolver,
	}, name
}

// newSignalTestClient builds a CSVClient with both CAN and signal writers.
// Returns the client and the paths of the CAN and signal temp files.
func newSignalTestClient(t *testing.T, resolver canModels.InterfaceResolver) (*CSVClient, string, string) {
	t.Helper()
	canFile, err := os.CreateTemp("", "csv_can_*.csv")
	require.NoError(t, err)
	canName := canFile.Name()
	require.NoError(t, canFile.Close())
	t.Cleanup(func() { os.Remove(canName) })

	sigFile, err := os.CreateTemp("", "csv_sig_*.csv")
	require.NoError(t, err)
	sigName := sigFile.Name()
	require.NoError(t, sigFile.Close())
	t.Cleanup(func() { os.Remove(sigName) })

	canWriter, err := csvfmt.NewCANWriter(canName, false)
	require.NoError(t, err)
	sigWriter, err := csvfmt.NewSignalWriter(sigName, false)
	require.NoError(t, err)

	return &CSVClient{
		canWriter:     canWriter,
		signalWriter:  sigWriter,
		canChannel:    make(chan canModels.CanMessageTimestamped, 16),
		signalChannel: make(chan canModels.CanSignalTimestamped, 16),
		filters:       common.NewFilterSet(),
		l:             silentLogger(),
		resolver:      resolver,
	}, canName, sigName
}

func readRowsByName(t *testing.T, name string) [][]string {
	t.Helper()
	f, err := os.Open(name)
	require.NoError(t, err)
	defer f.Close()
	rows, err := stdcsv.NewReader(f).ReadAll()
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
	client, canPath := newTestClient(t, resolver)

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
	require.NoError(t, client.canWriter.Flush())

	rows := readRowsByName(t, canPath)
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
	client, canPath := newTestClient(t, &mockResolver{conns: map[int]*mockCanConn{}})

	msg := canModels.CanMessageTimestamped{
		Interface: 99,
		ID:        0x001,
		Data:      []byte{0x01},
	}
	client.HandleCanMessage(msg)
	require.NoError(t, client.canWriter.Flush())

	rows := readRowsByName(t, canPath)
	require.Len(t, rows, 1)
	assert.Equal(t, "", rows[0][2], "unknown interface should produce empty interface name")
}

func TestCSVClient_HandleChannel(t *testing.T) {
	resolver := &mockResolver{
		conns: map[int]*mockCanConn{0: {interfaceName: "iface0"}},
	}
	client, canPath := newTestClient(t, resolver)

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

	rows := readRowsByName(t, canPath)
	require.Len(t, rows, 3)
	assert.Equal(t, "256", rows[0][1])  // 0x100
	assert.Equal(t, "512", rows[1][1])  // 0x200
	assert.Equal(t, "768", rows[2][1])  // 0x300
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
	require.NoError(t, csvClient.canWriter.Flush())

	rows := readRowsByName(t, cfg.CSVLog.CanOutputFile)
	require.Len(t, rows, 1, "header row should be written when IncludeHeaders is true")
	assert.Equal(t, []string{"timestamp", "id", "interface", "remote", "transmit", "length", "data"}, rows[0])
}

func TestNewClient_IncludeHeaders_False(t *testing.T) {
	cfg := newTestConfig(t, false)
	client, err := NewClient(t.Context(), cfg, silentLogger(), &mockResolver{})
	require.NoError(t, err)
	csvClient := client.(*CSVClient)
	require.NoError(t, csvClient.canWriter.Flush())

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
	client, _, sigPath := newSignalTestClient(t, resolver)

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

	rows := readRowsByName(t, sigPath)
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
	client, _, sigPath := newSignalTestClient(t, resolver)

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

	rows := readRowsByName(t, sigPath)
	require.Len(t, rows, 2)
	assert.Equal(t, "RPM", rows[0][3])
	assert.Equal(t, "TEMP", rows[1][3])
}

func TestCSVClient_HandleSignalChannel_WritesHeader(t *testing.T) {
	_, _, sigPath := newSignalTestClient(t, &mockResolver{})

	sigWriter, err := csvfmt.NewSignalWriter(sigPath, true)
	require.NoError(t, err)
	require.NoError(t, sigWriter.Flush())
	require.NoError(t, sigWriter.Close())

	rows := readRowsByName(t, sigPath)
	require.Len(t, rows, 1)
	assert.Equal(t, "timestamp", rows[0][0])
}
