package mqtt

import (
	"encoding/json"
	"net"
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

func newTestMQTTClient(resolver canModels.InterfaceResolver) *MQTTClient {
	return &MQTTClient{
		resolver: resolver,
		filters:  make(map[string]canModels.FilterInterface),
	}
}

func TestToJSON_FieldValues(t *testing.T) {
	resolver := &mockResolver{
		conns: map[int]*mockCanConn{
			0: {interfaceName: "can0-can-vcan0"},
		},
	}
	c := newTestMQTTClient(resolver)

	msg := canModels.CanMessageTimestamped{
		Timestamp: 1_000_000_000,
		Interface: 0,
		ID:        0x1AB,
		Transmit:  true,
		Remote:    false,
		Length:    4,
		Data:      []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}

	jsonStr, err := c.ToJSON(msg)
	require.NoError(t, err)

	var out struct {
		Timestamp int64  `json:"timestamp"`
		Interface string `json:"interface"`
		ID        string `json:"id"`
		Transmit  bool   `json:"transmit"`
		Remote    bool   `json:"remote"`
		Length    uint8  `json:"length"`
		Data      string `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &out))

	assert.Equal(t, int64(1_000_000_000), out.Timestamp)
	assert.Equal(t, "can0-can-vcan0", out.Interface)
	assert.Equal(t, "0x1ab", out.ID)
	assert.True(t, out.Transmit)
	assert.False(t, out.Remote)
	assert.Equal(t, uint8(4), out.Length)
	assert.Equal(t, "deadbeef", out.Data)
}

func TestToJSON_UnknownInterface(t *testing.T) {
	c := newTestMQTTClient(&mockResolver{conns: map[int]*mockCanConn{}})

	msg := canModels.CanMessageTimestamped{
		Interface: 99,
		ID:        0x001,
		Data:      []byte{0x00},
	}

	jsonStr, err := c.ToJSON(msg)
	require.NoError(t, err)

	var out struct {
		Interface string `json:"interface"`
	}
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &out))
	assert.Equal(t, "", out.Interface, "unknown interface should produce empty string")
}

func TestToJSON_EmptyData(t *testing.T) {
	c := newTestMQTTClient(&mockResolver{conns: map[int]*mockCanConn{}})

	msg := canModels.CanMessageTimestamped{
		ID:   0x100,
		Data: []byte{},
	}

	jsonStr, err := c.ToJSON(msg)
	require.NoError(t, err)

	var out struct {
		Data string `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &out))
	assert.Equal(t, "", out.Data)
}
