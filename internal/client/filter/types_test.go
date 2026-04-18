package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

func TestCanTransmitFilter(t *testing.T) {
	txMsg := canModels.CanMessageTimestamped{Transmit: true}
	rxMsg := canModels.CanMessageTimestamped{Transmit: false}

	txFilter := CanTransmitFilter{Value: canModels.TXOnly}
	assert.Equal(t, true, txFilter.Filter(txMsg), "TXOnly: TX message should pass")
	assert.Equal(t, false, txFilter.Filter(rxMsg), "TXOnly: RX message should not pass")

	rxFilter := CanTransmitFilter{Value: canModels.RXOnly}
	assert.Equal(t, false, rxFilter.Filter(txMsg), "RXOnly: TX message should not pass")
	assert.Equal(t, true, rxFilter.Filter(rxMsg), "RXOnly: RX message should pass")

	allFilter := CanTransmitFilter{Value: canModels.TXAndRX}
	assert.Equal(t, true, allFilter.Filter(txMsg), "TXAndRX: TX message should pass")
	assert.Equal(t, true, allFilter.Filter(rxMsg), "TXAndRX: RX message should pass")
}

func TestCanDataLengthFilter(t *testing.T) {
	msg4 := canModels.CanMessageTimestamped{Length: 4}
	msg8 := canModels.CanMessageTimestamped{Length: 8}

	gtFilter := CanDataLengthFilter{Value: 4, Operator: canModels.LengthGreaterThan}
	assert.Equal(t, true, gtFilter.Filter(msg8), "GreaterThan: 8 > 4 should pass")
	assert.Equal(t, false, gtFilter.Filter(msg4), "GreaterThan: 4 > 4 should not pass")

	ltFilter := CanDataLengthFilter{Value: 8, Operator: canModels.LengthLessThan}
	assert.Equal(t, true, ltFilter.Filter(msg4), "LessThan: 4 < 8 should pass")
	assert.Equal(t, false, ltFilter.Filter(msg8), "LessThan: 8 < 8 should not pass")

	neFilter := CanDataLengthFilter{Value: 4, Operator: canModels.LengthNotEquals}
	assert.Equal(t, true, neFilter.Filter(msg8), "NotEquals: 8 != 4 should pass")
	assert.Equal(t, false, neFilter.Filter(msg4), "NotEquals: 4 != 4 should not pass")

	eqFilter := CanDataLengthFilter{Value: 4, Operator: canModels.LengthEquals}
	assert.Equal(t, true, eqFilter.Filter(msg4), "Equals: 4 == 4 should pass")
	assert.Equal(t, false, eqFilter.Filter(msg8), "Equals: 8 == 4 should not pass")
}

func TestCanRemoteFilter(t *testing.T) {
	remoteMsg := canModels.CanMessageTimestamped{Remote: true}
	normalMsg := canModels.CanMessageTimestamped{Remote: false}

	excludeFilter := CanRemoteFilter{Value: canModels.ExcludeRemote}
	assert.Equal(t, false, excludeFilter.Filter(remoteMsg), "ExcludeRemote: remote message should not pass")
	assert.Equal(t, true, excludeFilter.Filter(normalMsg), "ExcludeRemote: normal message should pass")

	onlyFilter := CanRemoteFilter{Value: canModels.RemoteOnly}
	assert.Equal(t, true, onlyFilter.Filter(remoteMsg), "RemoteOnly: remote message should pass")
	assert.Equal(t, false, onlyFilter.Filter(normalMsg), "RemoteOnly: normal message should not pass")

	includeFilter := CanRemoteFilter{Value: canModels.IncludeRemote}
	assert.Equal(t, true, includeFilter.Filter(remoteMsg), "IncludeRemote: remote message should pass")
	assert.Equal(t, true, includeFilter.Filter(normalMsg), "IncludeRemote: normal message should pass")
}

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

func TestCanDataFilter_OrOperator(t *testing.T) {
	msgAllMatch := canModels.CanMessageTimestamped{
		Data: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
	}
	msgOneMismatch := canModels.CanMessageTimestamped{
		Data: []byte{0x01, 0xFF, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
	}
	msgNoMatch := canModels.CanMessageTimestamped{
		Data: []byte{0xFF, 0xFF, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
	}

	orFilter := CanDataFilter{
		Operator: canModels.FilterOr,
		Filters: []struct {
			Byte     uint8
			Operator canModels.CanDataFilterOperator
			Target   uint8
		}{
			{Byte: 0, Operator: canModels.DataEquals, Target: 0x01},
			{Byte: 1, Operator: canModels.DataEquals, Target: 0x02},
		},
	}

	assert.Equal(t, true, orFilter.Filter(msgAllMatch), "OR: all conditions match should return true")
	assert.Equal(t, true, orFilter.Filter(msgOneMismatch), "OR: one match should return true")
	assert.Equal(t, false, orFilter.Filter(msgNoMatch), "OR: no conditions match should return false")
}

func TestCanDataFilter_GreaterThan(t *testing.T) {
	filter := CanDataFilter{
		Operator: canModels.FilterAnd,
		Filters: []struct {
			Byte     uint8
			Operator canModels.CanDataFilterOperator
			Target   uint8
		}{
			{Byte: 0, Operator: canModels.DataGreaterThan, Target: 3},
		},
	}

	msgAbove := canModels.CanMessageTimestamped{Data: []byte{5}}
	assert.Equal(t, true, filter.Filter(msgAbove), "GreaterThan: 5 > 3 should pass")

	msgBelow := canModels.CanMessageTimestamped{Data: []byte{7}}
	filterBelow := CanDataFilter{
		Operator: canModels.FilterAnd,
		Filters: []struct {
			Byte     uint8
			Operator canModels.CanDataFilterOperator
			Target   uint8
		}{
			{Byte: 0, Operator: canModels.DataGreaterThan, Target: 7},
		},
	}
	assert.Equal(t, false, filterBelow.Filter(msgBelow), "GreaterThan: 7 > 7 should not pass")
}

func TestCanDataFilter_LessThan(t *testing.T) {
	filter := CanDataFilter{
		Operator: canModels.FilterAnd,
		Filters: []struct {
			Byte     uint8
			Operator canModels.CanDataFilterOperator
			Target   uint8
		}{
			{Byte: 0, Operator: canModels.DataLessThan, Target: 7},
		},
	}

	msgBelow := canModels.CanMessageTimestamped{Data: []byte{5}}
	assert.Equal(t, true, filter.Filter(msgBelow), "LessThan: 5 < 7 should pass")

	msgEqual := canModels.CanMessageTimestamped{Data: []byte{7}}
	assert.Equal(t, false, filter.Filter(msgEqual), "LessThan: 7 < 7 should not pass")
}

func TestCanDataFilter_NotEquals(t *testing.T) {
	filter := CanDataFilter{
		Operator: canModels.FilterAnd,
		Filters: []struct {
			Byte     uint8
			Operator canModels.CanDataFilterOperator
			Target   uint8
		}{
			{Byte: 0, Operator: canModels.DataNotEquals, Target: 5},
		},
	}

	msgDifferent := canModels.CanMessageTimestamped{Data: []byte{3}}
	assert.Equal(t, true, filter.Filter(msgDifferent), "NotEquals: 3 != 5 should pass")

	msgSame := canModels.CanMessageTimestamped{Data: []byte{5}}
	assert.Equal(t, false, filter.Filter(msgSame), "NotEquals: 5 != 5 should not pass")
}

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
