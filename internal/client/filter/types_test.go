package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	canModel "github.com/robbiebyrd/bb/internal/models"
)

func TestCanInterfaceFilter(t *testing.T) {
	testMessage1 := canModel.CanMessage{
		Timestamp: 0,
		Interface: "can0:>",
		Transmit:  false,
		ID:        123,
		Length:    8,
		Remote:    false,
		Data:      []byte{},
	}

	testFilter1 := CanInterfaceFilter{Value: "can0", Operator: canModel.TextContains}
	assert.Equal(t, true, testFilter1.Filter(testMessage1), "Should be true.")

	testMessage2 := canModel.CanMessage{
		Timestamp: 0,
		Interface: "can1:>",
		Transmit:  false,
		ID:        123,
		Length:    8,
		Remote:    false,
		Data:      []byte{},
	}

	testFilter2 := CanInterfaceFilter{Value: "can0", Operator: canModel.TextContains}
	assert.Equal(t, false, testFilter2.Filter(testMessage2), "Should be false.")

	testMessage3 := canModel.CanMessage{
		Timestamp: 0,
		Interface: "can0:>",
		Transmit:  false,
		ID:        123,
		Length:    8,
		Remote:    false,
		Data:      []byte{},
	}

	testFilter3 := CanInterfaceFilter{Value: "can0:>", Operator: canModel.TextEquals}
	assert.Equal(t, true, testFilter3.Filter(testMessage3), "Should be true.")

	testMessage4 := canModel.CanMessage{
		Timestamp: 0,
		Interface: "can1:>",
		Transmit:  false,
		ID:        123,
		Length:    8,
		Remote:    false,
		Data:      []byte{},
	}

	testFilter4 := CanInterfaceFilter{Value: "can0", Operator: canModel.TextEquals}
	assert.Equal(t, false, testFilter4.Filter(testMessage4), "Should be false.")
}
