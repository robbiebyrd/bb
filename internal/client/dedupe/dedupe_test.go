package dedupe

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/robbiebyrd/bb/internal/client/logging"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

func TestFilter_SweepEvictsExpiredEntries(t *testing.T) {
	lvl := new(slog.LevelVar)
	lvl.Set(slog.LevelInfo)
	l := logging.NewJSONLogger(lvl)

	// 1ms timeout
	dc := NewDedupeFilterClient(l, 1, []uint32{}).(*DedupeFilterClient)

	// Insert 5 unique messages — each creates a new storage entry
	for i := 0; i < 5; i++ {
		msg := canModels.CanMessageTimestamped{
			ID:   uint32(i),
			Data: []byte{byte(i)},
		}
		dc.Filter(msg)
	}
	assert.Equal(t, 5, len(dc.storage), "should have 5 entries before expiry")

	// Wait for all entries to expire
	time.Sleep(10 * time.Millisecond)

	// Insert one new unique message — sweep triggered on new addition should evict all 5 expired entries
	newMsg := canModels.CanMessageTimestamped{ID: 99, Data: []byte{0x99}}
	dc.Filter(newMsg)

	assert.Equal(t, 1, len(dc.storage), "storage should contain only the new entry after sweep evicts expired ones")
}

func TestCanInterfaceFilter(t *testing.T) {
	lvl := new(slog.LevelVar)
	lvl.Set(slog.LevelInfo)
	l := logging.NewJSONLogger(lvl)

	a := NewDedupeFilterClient(l, 1000, []uint32{42})

	stamped := canModels.CanMessageTimestamped{
		Timestamp: 123456789,
		Interface: 0,
		ID:        42,
		Transmit:  false,
		Remote:    false,
		Length:    8,
		Data:      []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
	}

	hash1, err := hashCanMessageData(stamped)
	assert.Nil(t, err, "Hashing should not return an error.")

	hash2, err := hashCanMessageData(stamped)
	assert.Nil(t, err, "Hashing should not return an error.")

	differentMsg := canModels.CanMessageTimestamped{
		Timestamp: 987654321,
		Interface: 1,
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

func TestFilterNonWatchedID(t *testing.T) {
	lvl := new(slog.LevelVar)
	lvl.Set(slog.LevelInfo)
	l := logging.NewJSONLogger(lvl)

	// Watch only ID 42; send a message with ID 99
	a := NewDedupeFilterClient(l, 1000, []uint32{42})

	msg := canModels.CanMessageTimestamped{
		Timestamp: 111111111,
		Interface: 0,
		ID:        99,
		Transmit:  false,
		Remote:    false,
		Length:    4,
		Data:      []byte{0x01, 0x02, 0x03, 0x04},
	}

	result := a.Filter(msg)
	assert.Equal(t, false, result, "Message with non-watched ID should not be filtered (returned false).")
}

func TestHashCanMessageData_Deterministic(t *testing.T) {
	msg := canModels.CanMessageTimestamped{
		Timestamp: 999, // should NOT affect hash
		Interface: 1, ID: 42, Transmit: true, Remote: false, Length: 4,
		Data: []byte{0x01, 0x02, 0x03, 0x04},
	}
	h1, err1 := hashCanMessageData(msg)
	msg.Timestamp = 12345 // different timestamp, same content
	h2, err2 := hashCanMessageData(msg)
	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, h1, h2, "same content with different timestamp must hash identically")
}

func TestFilterTimeoutExpiry(t *testing.T) {
	lvl := new(slog.LevelVar)
	lvl.Set(slog.LevelInfo)
	l := logging.NewJSONLogger(lvl)

	// 1ms timeout so it expires almost immediately
	a := NewDedupeFilterClient(l, 1, []uint32{42})

	msg := canModels.CanMessageTimestamped{
		Timestamp: 222222222,
		Interface: 0,
		ID:        42,
		Transmit:  false,
		Remote:    false,
		Length:    4,
		Data:      []byte{0xAA, 0xBB, 0xCC, 0xDD},
	}

	first := a.Filter(msg)
	assert.Equal(t, false, first, "First occurrence should not be filtered.")

	second := a.Filter(msg)
	assert.Equal(t, true, second, "Immediate repeat should be filtered.")

	// Wait for timeout to expire
	time.Sleep(5 * time.Millisecond)

	afterExpiry := a.Filter(msg)
	assert.Equal(t, false, afterExpiry, "After timeout expiry, message should not be filtered (entry expired).")
}
