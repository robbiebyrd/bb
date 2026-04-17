package connection

import (
	"context"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

// mockConn implements canModels.CanConnection for testing without real hardware.
type mockConn struct {
	id          int
	name        string
	network     string
	uri         string
	dbcFilePath *string
	open        bool
}

func (m *mockConn) GetID() int                  { return m.id }
func (m *mockConn) SetID(id int)                { m.id = id }
func (m *mockConn) GetName() string             { return m.name }
func (m *mockConn) GetInterfaceName() string    { return m.name }
func (m *mockConn) SetName(n string)            { m.name = n }
func (m *mockConn) GetDBCFilePath() *string     { return m.dbcFilePath }
func (m *mockConn) SetDBCFilePath(p *string)    { m.dbcFilePath = p }
func (m *mockConn) GetConnection() net.Conn     { return nil }
func (m *mockConn) SetConnection(_ net.Conn)    {}
func (m *mockConn) GetNetwork() string          { return m.network }
func (m *mockConn) SetNetwork(n string)         { m.network = n }
func (m *mockConn) GetURI() string              { return m.uri }
func (m *mockConn) SetURI(u string)             { m.uri = u }
func (m *mockConn) Open() error                 { m.open = true; return nil }
func (m *mockConn) Close() error                { m.open = false; return nil }
func (m *mockConn) IsOpen() bool                { return m.open }
func (m *mockConn) Discontinue() error          { return nil }
func (m *mockConn) Receive(_ *sync.WaitGroup)   {}

func newMock(name string) *mockConn {
	return &mockConn{name: name, network: "sim", uri: name}
}

func newTestManager(t *testing.T) canModels.ConnectionManager {
	t.Helper()
	ch := make(chan canModels.CanMessageTimestamped, 16)
	cfg := &canModels.Config{CanInterfaceSeparator: "-", SimEmitRate: 1000000}
	l := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewConnectionManager(context.Background(), cfg, ch, l)
}

func TestConnectionManager_Add_AssignsIncrementingIDs(t *testing.T) {
	cm := newTestManager(t)

	id0 := cm.Add(newMock("conn0"))
	id1 := cm.Add(newMock("conn1"))
	id2 := cm.Add(newMock("conn2"))

	assert.Equal(t, 0, id0)
	assert.Equal(t, 1, id1)
	assert.Equal(t, 2, id2)
}

func TestConnectionManager_Add_NilReturnsNegativeOne(t *testing.T) {
	cm := newTestManager(t)
	id := cm.Add(nil)
	assert.Equal(t, -1, id)
}

func TestConnectionManager_Add_SetsIDOnConnection(t *testing.T) {
	cm := newTestManager(t)
	mock := newMock("conn")
	id := cm.Add(mock)
	assert.Equal(t, id, mock.GetID())
}

func TestConnectionManager_ConnectionByID(t *testing.T) {
	cm := newTestManager(t)
	cm.Add(newMock("alpha"))

	conn := cm.ConnectionByID(0)
	require.NotNil(t, conn)
	assert.Equal(t, "alpha", conn.GetName())
}

func TestConnectionManager_ConnectionByID_UnknownReturnsNil(t *testing.T) {
	cm := newTestManager(t)
	assert.Nil(t, cm.ConnectionByID(99))
}

func TestConnectionManager_ConnectionByName(t *testing.T) {
	cm := newTestManager(t)
	cm.Add(newMock("beta"))
	cm.Add(newMock("gamma"))

	conn := cm.ConnectionByName("gamma")
	require.NotNil(t, conn)
	assert.Equal(t, "gamma", conn.GetName())
}

func TestConnectionManager_ConnectionByName_UnknownReturnsNil(t *testing.T) {
	cm := newTestManager(t)
	assert.Nil(t, cm.ConnectionByName("ghost"))
}

func TestConnectionManager_Connections_Empty(t *testing.T) {
	cm := newTestManager(t)
	assert.Empty(t, cm.Connections())
}

func TestConnectionManager_Connections_ReturnsAll(t *testing.T) {
	cm := newTestManager(t)
	cm.Add(newMock("a"))
	cm.Add(newMock("b"))
	assert.Len(t, cm.Connections(), 2)
}

func TestConnectionManager_DeleteConnection_RemovesAndCloses(t *testing.T) {
	cm := newTestManager(t)
	mock := newMock("target")
	mock.open = true
	cm.Add(mock)

	cm.DeleteConnection("target")

	assert.Empty(t, cm.Connections())
	assert.False(t, mock.open, "Close should have been called on the removed connection")
}

func TestConnectionManager_DeleteConnection_UnknownIsNoop(t *testing.T) {
	cm := newTestManager(t)
	// Should not panic for a name that was never added.
	cm.DeleteConnection("ghost")
}

func TestConnectionManager_OpenAll(t *testing.T) {
	cm := newTestManager(t)
	m1 := newMock("c1")
	m2 := newMock("c2")
	cm.Add(m1)
	cm.Add(m2)

	err := cm.OpenAll()
	require.NoError(t, err)
	assert.True(t, m1.open)
	assert.True(t, m2.open)
}

func TestConnectionManager_CloseAll(t *testing.T) {
	cm := newTestManager(t)
	m1 := newMock("c1")
	m2 := newMock("c2")
	cm.Add(m1)
	cm.Add(m2)
	_ = cm.OpenAll()

	err := cm.CloseAll()
	require.NoError(t, err)
	assert.False(t, m1.open)
	assert.False(t, m2.open)
}

func TestConnectionManager_ReceiveAll_NoConnections(t *testing.T) {
	cm := newTestManager(t)
	// No connections — ReceiveAll should return immediately.
	err := cm.ReceiveAll()
	assert.NoError(t, err)
}

func TestConnectionManager_ReceiveAll_WithMockConnections(t *testing.T) {
	cm := newTestManager(t)
	cm.Add(newMock("r1"))
	cm.Add(newMock("r2"))
	// Mock Receive is a no-op so ReceiveAll returns immediately.
	err := cm.ReceiveAll()
	assert.NoError(t, err)
}

func TestConnectionManager_Connect_Sim(t *testing.T) {
	cm := newTestManager(t)
	cm.Connect(canModels.CanInterfaceOption{Name: "sim1", Network: "sim"})

	assert.Len(t, cm.Connections(), 1)
	conn := cm.ConnectionByID(0)
	require.NotNil(t, conn)
	assert.Equal(t, "sim1", conn.GetName())
}

func TestConnectionManager_ConnectMultiple(t *testing.T) {
	cm := newTestManager(t)
	opts := canModels.CanInterfaceOptions{
		{Name: "s1", Network: "sim"},
		{Name: "s2", Network: "sim"},
	}
	cm.ConnectMultiple(opts)

	assert.Len(t, cm.Connections(), 2)
}
