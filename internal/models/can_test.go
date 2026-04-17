package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/robbiebyrd/bb/internal/models"
)

func TestCanSignalTimestamped_Fields(t *testing.T) {
	sig := models.CanSignalTimestamped{
		Timestamp: 1_000_000_000,
		Interface: 2,
		ID:        0x141,
		Signal:    "EngineRPM",
		Value:     3500.5,
		Unit:      "rpm",
	}

	assert.Equal(t, int64(1_000_000_000), sig.Timestamp)
	assert.Equal(t, 2, sig.Interface)
	assert.Equal(t, uint32(0x141), sig.ID)
	assert.Equal(t, "EngineRPM", sig.Signal)
	assert.InDelta(t, 3500.5, sig.Value, 0.001)
	assert.Equal(t, "rpm", sig.Unit)
}

func TestCanSignalTimestamped_ZeroValue(t *testing.T) {
	var sig models.CanSignalTimestamped
	assert.Equal(t, int64(0), sig.Timestamp)
	assert.Equal(t, float64(0), sig.Value)
	assert.Equal(t, "", sig.Unit)
}
