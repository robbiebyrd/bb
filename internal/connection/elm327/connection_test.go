package elm327

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	canModels "github.com/robbiebyrd/cantou/internal/models"
)

func testConfig() *canModels.Config { return &canModels.Config{CanInterfaceSeparator: "-"} }

func newConn(opt canModels.CanInterfaceOption) interface {
	GetNetwork() string
	GetURI() string
} {
	logger := slog.New(slog.DiscardHandler)
	return NewConnection(context.Background(), testConfig(), make(chan canModels.CanMessageTimestamped, 1), logger, opt)
}

func TestNewConnection_DefaultsNetworkToELM327(t *testing.T) {
	c := newConn(canModels.CanInterfaceOption{Name: "scanner"})
	assert.Equal(t, canModels.NetworkELM327, c.GetNetwork())
}

func TestNewConnection_ImplementsCanConnection(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	c := NewConnection(context.Background(), testConfig(), make(chan canModels.CanMessageTimestamped, 1), logger,
		canModels.CanInterfaceOption{Name: "scanner", URI: "/dev/rfcomm0"})
	var _ canModels.CanConnection = c
	require.Equal(t, "scanner-elm327-/dev/rfcomm0", c.GetInterfaceName())
}
