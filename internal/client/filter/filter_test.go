package filter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

func TestFilterClient_NewFilterClient(t *testing.T) {
	ctx := context.Background()
	fc := NewFilterClient(&ctx)
	assert.NotNil(t, fc)
}

func TestFilterClient_NoFilters_AndDefaultsToTrue(t *testing.T) {
	ctx := context.Background()
	fc := NewFilterClient(&ctx)

	msg := canModels.CanMessageTimestamped{ID: 0x100}
	// With no filters, ArrayAllTrue([]) = true (no false values found).
	assert.True(t, fc.Filter(msg))
}

func TestFilterClient_AndOperator_AllMatch(t *testing.T) {
	ctx := context.Background()
	fc := NewFilterClient(&ctx)
	fc.Mode(canModels.FilterAnd)

	// Add two filters that both pass.
	_ = fc.Add(CanTransmitFilter{Value: canModels.TXOnly})
	_ = fc.Add(CanRemoteFilter{Value: canModels.ExcludeRemote})

	msg := canModels.CanMessageTimestamped{Transmit: true, Remote: false}
	assert.True(t, fc.Filter(msg), "AND: all conditions match should return true")
}

func TestFilterClient_AndOperator_OneMismatch(t *testing.T) {
	ctx := context.Background()
	fc := NewFilterClient(&ctx)
	fc.Mode(canModels.FilterAnd)

	_ = fc.Add(CanTransmitFilter{Value: canModels.TXOnly})
	_ = fc.Add(CanRemoteFilter{Value: canModels.ExcludeRemote})

	// Transmit=false fails the TXOnly filter.
	msg := canModels.CanMessageTimestamped{Transmit: false, Remote: false}
	assert.False(t, fc.Filter(msg), "AND: one mismatch should return false")
}

func TestFilterClient_OrOperator_OneMatch(t *testing.T) {
	ctx := context.Background()
	fc := NewFilterClient(&ctx)
	fc.Mode(canModels.FilterOr)

	_ = fc.Add(CanTransmitFilter{Value: canModels.TXOnly})
	_ = fc.Add(CanRemoteFilter{Value: canModels.RemoteOnly})

	// Only Transmit=true matches TXOnly; Remote=false fails RemoteOnly.
	msg := canModels.CanMessageTimestamped{Transmit: true, Remote: false}
	assert.True(t, fc.Filter(msg), "OR: one match should return true")
}

func TestFilterClient_OrOperator_NoMatch(t *testing.T) {
	ctx := context.Background()
	fc := NewFilterClient(&ctx)
	fc.Mode(canModels.FilterOr)

	_ = fc.Add(CanTransmitFilter{Value: canModels.TXOnly})
	_ = fc.Add(CanRemoteFilter{Value: canModels.RemoteOnly})

	// Transmit=false fails TXOnly; Remote=false fails RemoteOnly.
	msg := canModels.CanMessageTimestamped{Transmit: false, Remote: false}
	assert.False(t, fc.Filter(msg), "OR: no conditions match should return false")
}

func TestFilterClient_Add_AppendsFilter(t *testing.T) {
	ctx := context.Background()
	fc := NewFilterClient(&ctx)

	err := fc.Add(CanTransmitFilter{Value: canModels.TXOnly})
	assert.NoError(t, err)

	err = fc.Add(CanRemoteFilter{Value: canModels.RemoteOnly})
	assert.NoError(t, err)
}

func TestFilterClient_Mode_SwitchesOperator(t *testing.T) {
	ctx := context.Background()
	fc := NewFilterClient(&ctx)

	_ = fc.Add(CanTransmitFilter{Value: canModels.TXOnly})
	_ = fc.Add(CanRemoteFilter{Value: canModels.RemoteOnly})

	// Only one filter matches: Transmit=true, Remote=false.
	msg := canModels.CanMessageTimestamped{Transmit: true, Remote: false}

	fc.Mode(canModels.FilterAnd)
	assert.False(t, fc.Filter(msg), "AND: RemoteOnly fails so overall false")

	fc.Mode(canModels.FilterOr)
	assert.True(t, fc.Filter(msg), "OR: TXOnly passes so overall true")
}
