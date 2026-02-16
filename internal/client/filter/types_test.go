package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

func TestCanInterfaceFilter(t *testing.T) {
	testMessage1 := canModels.CanMessageTimestamped{
		Timestamp: 0,
		Interface: 0,
		Transmit:  false,
		ID:        123,
		Length:    8,
		Remote:    false,
		Data:      []byte{},
	}

	testFilter1 := CanInterfaceFilter{Value: 0}
	assert.Equal(t, true, testFilter1.Filter(testMessage1), "Should be true when interface IDs match.")

	testMessage2 := canModels.CanMessageTimestamped{
		Timestamp: 0,
		Interface: 1,
		Transmit:  false,
		ID:        123,
		Length:    8,
		Remote:    false,
		Data:      []byte{},
	}

	testFilter2 := CanInterfaceFilter{Value: 0}
	assert.Equal(t, false, testFilter2.Filter(testMessage2), "Should be false when interface IDs differ.")

	testFilter3 := CanInterfaceFilter{Value: 1}
	assert.Equal(t, true, testFilter3.Filter(testMessage2), "Should be true when interface IDs match.")
}
