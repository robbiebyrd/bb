package mqtt

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/robbiebyrd/cantou/internal/client/common"
	canModels "github.com/robbiebyrd/cantou/internal/models"
)

// mockCanConn implements canModels.CanConnection for testing.
type mockCanConn struct {
	name          string
	interfaceName string
}

func (m *mockCanConn) SetID(_ int)                {}
func (m *mockCanConn) GetName() string            { return m.name }
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

func newTestMQTTClient(resolver canModels.InterfaceResolver) *MQTTClient {
	return &MQTTClient{
		resolver: resolver,
		filters:  common.NewFilterSet(),
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
			0: {name: "jeep0"},
		},
	}
	c := &MQTTClient{
		resolver: resolver,
		topic:    "bb_app",
	}

	msg := canModels.CanMessageTimestamped{Interface: 0, ID: 0x123}
	topic := c.getTopicFromMessage(msg)

	assert.Equal(t, "/bb_app/jeep0/0/messages/0x123", topic)
	assert.NotContains(t, topic, "/Users/robbie", "topic must not contain the file path")
}

func TestGetTopicFromMessage_UnknownInterface(t *testing.T) {
	c := &MQTTClient{
		resolver: &mockResolver{conns: map[int]*mockCanConn{}},
		topic:    "bb_app",
	}

	msg := canModels.CanMessageTimestamped{Interface: 99, ID: 0xABC}
	topic := c.getTopicFromMessage(msg)

	assert.Equal(t, "/bb_app/unknown/99/messages/0xabc", topic)
}

func TestGetTopicFromSignal_UsesConnectionName(t *testing.T) {
	resolver := &mockResolver{
		conns: map[int]*mockCanConn{
			0: {name: "jeep0"},
		},
	}
	c := &MQTTClient{
		resolver: resolver,
		topic:    "bb_app",
	}

	sig := canModels.CanSignalTimestamped{Interface: 0, ID: 0x141, Message: "MSG_ENGINE", Signal: "EngineSpeed"}
	topic := c.getTopicFromSignal(sig)

	assert.Equal(t, "/bb_app/jeep0/0/signals/MSG_ENGINE/EngineSpeed", topic)
	assert.NotContains(t, topic, "/Users/robbie", "topic must not contain the file path")
}

func TestGetTopicFromSignal_UnknownInterface(t *testing.T) {
	c := &MQTTClient{
		resolver: &mockResolver{conns: map[int]*mockCanConn{}},
		topic:    "bb_app",
	}

	sig := canModels.CanSignalTimestamped{Interface: 99, ID: 0xABC, Message: "MSG_TEST", Signal: "TestSig"}
	topic := c.getTopicFromSignal(sig)

	assert.Equal(t, "/bb_app/unknown/99/signals/MSG_TEST/TestSig", topic)
}

func TestSignalPayload_WholeNumber(t *testing.T) {
	sig := canModels.CanSignalTimestamped{Value: 3500.0}
	assert.Equal(t, "3500", signalPayload(sig))
}

func TestSignalPayload_Zero(t *testing.T) {
	sig := canModels.CanSignalTimestamped{Value: 0}
	assert.Equal(t, "0", signalPayload(sig))
}

func TestSignalPayload_Fractional(t *testing.T) {
	sig := canModels.CanSignalTimestamped{Value: 3500.5}
	assert.Equal(t, "3500.5", signalPayload(sig))
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
