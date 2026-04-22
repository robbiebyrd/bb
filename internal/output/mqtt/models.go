package mqtt

import (
	"encoding/hex"
	"encoding/json"
	"strconv"

	canModels "github.com/robbiebyrd/cantou/internal/models"
)

func signalPayload(sig canModels.CanSignalTimestamped) string {
	return strconv.FormatFloat(sig.Value, 'f', -1, 64)
}

// mqttCanMessage is the wire shape for JSON-encoded CAN frames. Fields are
// serialized in the order listed.
type mqttCanMessage struct {
	Timestamp int64  `json:"timestamp"`
	Interface string `json:"interface"`
	ID        string `json:"id"`
	Transmit  bool   `json:"transmit"`
	Remote    bool   `json:"remote"`
	Length    uint8  `json:"length"`
	Data      string `json:"data"`
}

// toJSONBytes marshals a CAN message to JSON bytes with minimal allocations.
// The returned slice is safe to pass to paho's Publish, which takes a []byte.
func (c *MQTTClient) toJSONBytes(canMsg canModels.CanMessageTimestamped) ([]byte, error) {
	interfaceName := ""
	if conn := c.resolver.ConnectionByID(canMsg.Interface); conn != nil {
		interfaceName = conn.GetName()
	}
	return json.Marshal(mqttCanMessage{
		Timestamp: canMsg.Timestamp,
		Interface: interfaceName,
		ID:        "0x" + strconv.FormatUint(uint64(canMsg.ID), 16),
		Transmit:  canMsg.Transmit,
		Remote:    canMsg.Remote,
		Length:    canMsg.Length,
		Data:      hex.EncodeToString(canMsg.Data),
	})
}

// ToJSON is the string-returning wrapper preserved for external callers and
// tests. Prefer toJSONBytes on the hot path.
func (c *MQTTClient) ToJSON(canMsg canModels.CanMessageTimestamped) (string, error) {
	b, err := c.toJSONBytes(canMsg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
