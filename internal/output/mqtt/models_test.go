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
	name          string
	interfaceName string
	uri           string
}

func (m *mockCanConn) GetID() int                  { return 0 }
func (m *mockCanConn) SetID(_ int)                 {}
func (m *mockCanConn) GetName() string             { return m.name }
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
			0: {name: "can0", interfaceName: "can0-can-vcan0"},
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
	assert.Equal(t, "can0", out.Interface)
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

func TestToJSON_InterfaceUsesConnectionName(t *testing.T) {
	resolver := &mockResolver{
		conns: map[int]*mockCanConn{
			0: {
				name:          "jeep0",
				interfaceName: "jeep0-playback-/Users/robbie.byrd/Documents/Jeep/Static Remote Start.log",
				uri:           "/Users/robbie.byrd/Documents/Jeep/Static Remote Start.log",
			},
		},
	}
	c := newTestMQTTClient(resolver)

	jsonStr, err := c.ToJSON(canModels.CanMessageTimestamped{Interface: 0, ID: 0x141})
	require.NoError(t, err)

	var out struct {
		Interface string `json:"interface"`
	}
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &out))
	assert.Equal(t, "jeep0", out.Interface)
	assert.NotContains(t, out.Interface, "/Users/robbie.byrd", "interface must not contain the file path")
}

func TestGetTopicFromMessage_UsesConnectionName(t *testing.T) {
	resolver := &mockResolver{
		conns: map[int]*mockCanConn{
			0: {name: "jeep0", uri: "/Users/robbie/Documents/Jeep/Static Remote Start.log"},
		},
	}
	c := &MQTTClient{
		resolver: resolver,
		topic:    "bb_app",
		cfg:      &canModels.Config{},
	}

	msg := canModels.CanMessageTimestamped{Interface: 0, ID: 0x123}
	topic := c.getTopicFromMessage(msg)

	assert.Equal(t, "/bb_app/jeep0/0x123/messages", topic)
	assert.NotContains(t, topic, "/Users/robbie", "topic must not contain the file path")
}

func TestGetTopicFromMessage_UnknownInterface(t *testing.T) {
	c := &MQTTClient{
		resolver: &mockResolver{conns: map[int]*mockCanConn{}},
		topic:    "bb_app",
		cfg:      &canModels.Config{},
	}

	msg := canModels.CanMessageTimestamped{Interface: 99, ID: 0xABC}
	topic := c.getTopicFromMessage(msg)

	assert.Equal(t, "/bb_app/unknown/0xABC/messages", topic)
}

func TestGetTopicFromSignal_UsesConnectionName(t *testing.T) {
	resolver := &mockResolver{
		conns: map[int]*mockCanConn{
			0: {name: "jeep0", uri: "/Users/robbie/Documents/Jeep/Static Remote Start.log"},
		},
	}
	c := &MQTTClient{
		resolver: resolver,
		topic:    "bb_app",
		cfg:      &canModels.Config{},
	}

	sig := canModels.CanSignalTimestamped{Interface: 0, ID: 0x141}
	topic := c.getTopicFromSignal(sig)

	assert.Equal(t, "/bb_app/jeep0/0x141/signals", topic)
	assert.NotContains(t, topic, "/Users/robbie", "topic must not contain the file path")
}

func TestGetTopicFromSignal_UnknownInterface(t *testing.T) {
	c := &MQTTClient{
		resolver: &mockResolver{conns: map[int]*mockCanConn{}},
		topic:    "bb_app",
		cfg:      &canModels.Config{},
	}

	sig := canModels.CanSignalTimestamped{Interface: 99, ID: 0xABC}
	topic := c.getTopicFromSignal(sig)

	assert.Equal(t, "/bb_app/unknown/0xABC/signals", topic)
}

func TestSignalToJSON_FieldValues(t *testing.T) {
	resolver := &mockResolver{
		conns: map[int]*mockCanConn{
			0: {name: "jeep0"},
		},
	}
	c := newTestMQTTClient(resolver)

	sig := canModels.CanSignalTimestamped{
		Timestamp: 1_000_000_000,
		Interface: 0,
		ID:        0x141,
		Signal:    "EngineSpeed",
		Value:     3500.0,
		Unit:      "rpm",
	}

	jsonStr, err := c.SignalToJSON(sig)
	require.NoError(t, err)

	var out struct {
		Timestamp int64   `json:"timestamp"`
		Interface string  `json:"interface"`
		ID        string  `json:"id"`
		Signal    string  `json:"signal"`
		Value     float64 `json:"value"`
		Unit      string  `json:"unit"`
	}
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &out))

	assert.Equal(t, int64(1_000_000_000), out.Timestamp)
	assert.Equal(t, "jeep0", out.Interface)
	assert.Equal(t, "0x141", out.ID)
	assert.Equal(t, "EngineSpeed", out.Signal)
	assert.Equal(t, 3500.0, out.Value)
	assert.Equal(t, "rpm", out.Unit)
}

func TestSignalToJSON_UnknownInterface(t *testing.T) {
	c := newTestMQTTClient(&mockResolver{conns: map[int]*mockCanConn{}})

	sig := canModels.CanSignalTimestamped{Interface: 99, ID: 0x001}

	jsonStr, err := c.SignalToJSON(sig)
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
