package csv

import (
	"encoding/csv"
	"io"
	"log/slog"
	"net"
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
	uri           string
}

func (m *mockCanConn) GetID() int                  { return 0 }
func (m *mockCanConn) SetID(_ int)                 {}
func (m *mockCanConn) GetName() string             { return "" }
func (m *mockCanConn) GetInterfaceName() string    { return m.interfaceName }
func (m *mockCanConn) SetName(_ string)            {}
func (m *mockCanConn) GetDBCFilePath() *string     { return nil }
func (m *mockCanConn) SetDBCFilePath(_ *string)    {}
func (m *mockCanConn) GetConnection() net.Conn     { return nil }
func (m *mockCanConn) SetConnection(_ net.Conn)    {}
func (m *mockCanConn) GetNetwork() string          { return "" }
func (m *mockCanConn) SetNetwork(_ string)         {}
func (m *mockCanConn) GetURI() string              { return m.uri }
func (m *mockCanConn) SetURI(_ string)             {}
func (m *mockCanConn) Open() error                 { return nil }
func (m *mockCanConn) Close() error                { return nil }
func (m *mockCanConn) IsOpen() bool                { return false }
func (m *mockCanConn) Discontinue() error          { return nil }
func (m *mockCanConn) Receive(_ *sync.WaitGroup)   {}

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
		w:               csv.NewWriter(f),
		incomingChannel: make(chan canModels.CanMessageTimestamped, 16),
		filters:         make(map[string]canModels.FilterInterface),
		l:               silentLogger(),
		resolver:        resolver,
	}, f
}

func readRows(t *testing.T, f *os.File) [][]string {
	t.Helper()
	_, err := f.Seek(0, io.SeekStart)
	require.NoError(t, err)
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

func TestCSVClient_Run(t *testing.T) {
	client, _ := newTestClient(t, &mockResolver{})
	assert.NoError(t, client.Run())
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
			client.incomingChannel <- m
		}
		close(client.incomingChannel)
	}()

	err := client.HandleCanMessageChannel()
	assert.NoError(t, err)

	rows := readRows(t, f)
	require.Len(t, rows, 3)
	assert.Equal(t, "256", rows[0][1])  // 0x100
	assert.Equal(t, "512", rows[1][1])  // 0x200
	assert.Equal(t, "768", rows[2][1])  // 0x300
}
