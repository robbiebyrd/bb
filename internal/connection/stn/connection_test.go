package stn

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	canModels "github.com/robbiebyrd/cantou/internal/models"
)

func testConfig() *canModels.Config { return &canModels.Config{CanInterfaceSeparator: "-"} }

func TestNewConnection_DefaultsNetworkToSTN(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	c := NewConnection(context.Background(), testConfig(), make(chan canModels.CanMessageTimestamped, 1), logger,
		canModels.CanInterfaceOption{Name: "mxplus"})
	assert.Equal(t, canModels.NetworkSTN, c.GetNetwork())
}

func TestNewConnection_ImplementsCanConnection(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	c := NewConnection(context.Background(), testConfig(), make(chan canModels.CanMessageTimestamped, 1), logger,
		canModels.CanInterfaceOption{Name: "mxplus", Network: canModels.NetworkSTN, URI: "/dev/rfcomm0"})
	var _ canModels.CanConnection = c
	require.Equal(t, "mxplus-stn-/dev/rfcomm0", c.GetInterfaceName())
	// STN retains hybrid mode when PIDs are supplied.
	hybrid := NewConnection(context.Background(), testConfig(), make(chan canModels.CanMessageTimestamped, 1), logger,
		canModels.CanInterfaceOption{Name: "mxplus", OBD: canModels.OBDOptions{Mode: "hybrid", PIDs: []string{"010C"}}})
	assert.Equal(t, "hybrid", hybrid.Mode())
}
