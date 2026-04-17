package mqtt

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

func (c *MQTTClient) ToJSON(canMsg canModels.CanMessageTimestamped) (string, error) {

	a := struct {
		Timestamp int64  `json:"timestamp"`
		Interface string `json:"interface"`
		ID        string `json:"id"`
		Transmit  bool   `json:"transmit"`
		Remote    bool   `json:"remote"`
		Length    uint8  `json:"length"`
		Data      string `json:"data"`
	}{
		Timestamp: canMsg.Timestamp,
		Interface: func() string {
			if conn := c.resolver.ConnectionByID(canMsg.Interface); conn != nil {
				return conn.GetName()
			}
			return ""
		}(),
		ID:        "0x" + fmt.Sprintf("%x", canMsg.ID),
		Transmit:  canMsg.Transmit,
		Remote:    canMsg.Remote,
		Length:    canMsg.Length,
		Data:      hex.EncodeToString(canMsg.Data),
	}

	jsonBytes, err := json.Marshal(a)
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}
