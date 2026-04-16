package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

func TestCanDataFilter_AndOperator(t *testing.T) {
	msgAllMatch := canModels.CanMessageTimestamped{
		Data: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
	}
	msgOneMismatch := canModels.CanMessageTimestamped{
		Data: []byte{0x01, 0xFF, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
	}

	andFilter := CanDataFilter{
		Operator: canModels.FilterAnd,
		Filters: []struct {
			Byte     uint8
			Operator canModels.CanDataFilterOperator
			Target   uint8
		}{
			{Byte: 0, Operator: canModels.DataEquals, Target: 0x01},
			{Byte: 1, Operator: canModels.DataEquals, Target: 0x02},
		},
	}

	assert.Equal(t, true, andFilter.Filter(msgAllMatch), "AND: all conditions match should return true")
	assert.Equal(t, false, andFilter.Filter(msgOneMismatch), "AND: one mismatch should return false")
}

func TestCanInterfaceFilter(t *testing.T) {
	testMessage1 := canModels.CanMessageTimestamped{
		Timestamp: 0,
		Interface: "can0=",
		Transmit:  false,
		ID:        123,
		Length:    8,
		Remote:    false,
		Data:      []byte{},
	}

	testFilter1 := CanInterfaceFilter{Value: "can0", Operator: canModels.TextContains}
	assert.Equal(t, true, testFilter1.Filter(testMessage1), "Should be true.")

	testMessage2 := canModels.CanMessageTimestamped{
		Timestamp: 0,
		Interface: "can1=",
		Transmit:  false,
		ID:        123,
		Length:    8,
		Remote:    false,
		Data:      []byte{},
	}

	testFilter2 := CanInterfaceFilter{Value: "can0", Operator: canModels.TextContains}
	assert.Equal(t, false, testFilter2.Filter(testMessage2), "Should be false.")

	testMessage3 := canModels.CanMessageTimestamped{
		Timestamp: 0,
		Interface: "can0=",
		Transmit:  false,
		ID:        123,
		Length:    8,
		Remote:    false,
		Data:      []byte{},
	}

	testFilter3 := CanInterfaceFilter{Value: "can0=", Operator: canModels.TextEquals}
	assert.Equal(t, true, testFilter3.Filter(testMessage3), "Should be true.")

	testMessage4 := canModels.CanMessageTimestamped{
		Timestamp: 0,
		Interface: "can1=",
		Transmit:  false,
		ID:        123,
		Length:    8,
		Remote:    false,
		Data:      []byte{},
	}

	testFilter4 := CanInterfaceFilter{Value: "can0", Operator: canModels.TextEquals}
	assert.Equal(t, false, testFilter4.Filter(testMessage4), "Should be false.")
}
