package dedupe

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/robbiebyrd/bb/internal/client/logging"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

func TestCanInterfaceFilter(t *testing.T) {
	lvl := new(slog.LevelVar)
	lvl.Set(slog.LevelInfo)
	l := logging.NewJSONLogger(lvl)

	a := NewDedupeFilterClient(l, 1000, []uint32{})

	stamped := canModels.CanMessageTimestamped{
		Timestamp: 123456789,
		Interface: "can0",
		ID:        42,
		Transmit:  false,
		Remote:    false,
		Length:    8,
		Data:      []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
	}
	unstamped := canModels.CanMessageData{}
	unstampedCheck := canModels.CanMessageData{
		Interface: "can0",
		ID:        42,
		Transmit:  false,
		Remote:    false,
		Length:    8,
		Data:      []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
	}

	marshalFrom(&stamped, &unstamped)
	assert.Equal(t, unstamped, unstampedCheck, "Unstamped message should match expected value.")

	hash1, err := hashCanMessageData(stamped)
	assert.Nil(t, err, "Hashing should not return an error.")

	hash2, err := hashCanMessageData(stamped)
	assert.Nil(t, err, "Hashing should not return an error.")

	differentMsg := canModels.CanMessageTimestamped{
		Timestamp: 987654321,
		Interface: "can1",
		ID:        31,
		Transmit:  false,
		Remote:    false,
		Length:    8,
		Data:      []byte{0x03, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
	}

	hash3, err := hashCanMessageData(differentMsg)
	assert.Nil(t, err, "Hashing should not return an error.")

	assert.Equal(t, hash1, hash2, "Hashes of identical messages should be equal.")
	assert.NotEqual(t, hash1, hash3, "Hashes of messages that are different should not be equal.")

	skip1 := a.Filter(stamped)
	assert.Equal(t, skip1, false, "First occurrence of message should not be skipped.")
	skip2 := a.Filter(stamped)
	assert.Equal(t, skip2, true, "Second occurrence of message should be skipped.")
}
