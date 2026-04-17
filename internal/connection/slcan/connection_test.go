package slcan

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

func testConfig() *canModels.Config {
	return &canModels.Config{
		CanInterfaceSeparator: "-",
	}
}

func testChannel() chan canModels.CanMessageTimestamped {
	return make(chan canModels.CanMessageTimestamped, 100)
}

func TestNewSLCanConnection_ValidParams(t *testing.T) {
	conn := NewSLCanConnection(context.Background(), testConfig(), "test-slcan", testChannel(), nil, nil, nil)
	require.NotNil(t, conn)
}

func TestNewSLCanConnection_EmptyNamePanics(t *testing.T) {
	assert.Panics(t, func() {
		NewSLCanConnection(context.Background(), testConfig(), "", testChannel(), nil, nil, nil)
	})
}

func TestNewSLCanConnection_NilChannelPanics(t *testing.T) {
	assert.Panics(t, func() {
		NewSLCanConnection(context.Background(), testConfig(), "test-slcan", nil, nil, nil, nil)
	})
}

func TestNewSLCanConnection_DefaultsNetworkToSlcan(t *testing.T) {
	conn := NewSLCanConnection(context.Background(), testConfig(), "test-slcan", testChannel(), nil, nil, nil)
	assert.Equal(t, "slcan", conn.GetNetwork())
}

func TestNewSLCanConnection_DefaultsURIToName(t *testing.T) {
	conn := NewSLCanConnection(context.Background(), testConfig(), "test-slcan", testChannel(), nil, nil, nil)
	assert.Equal(t, "test-slcan", conn.GetURI())
}

func TestSLCanConnectionClient_GettersAndSetters(t *testing.T) {
	conn := NewSLCanConnection(context.Background(), testConfig(), "test-slcan", testChannel(), nil, nil, nil)

	conn.SetID(42)
	assert.Equal(t, 42, conn.GetID())

	conn.SetName("new-name")
	assert.Equal(t, "new-name", conn.GetName())

	conn.SetURI("/dev/ttyUSB1")
	assert.Equal(t, "/dev/ttyUSB1", conn.GetURI())

	conn.SetNetwork("slcan")
	assert.Equal(t, "slcan", conn.GetNetwork())

	filePath := "/path/to/file.dbc"
	conn.SetDBCFilePath(&filePath)
	assert.Equal(t, &filePath, conn.GetDBCFilePath())

	// GetConnection returns nil — SLCAN uses a serial adapter, not net.Conn
	assert.Nil(t, conn.GetConnection())
	conn.SetConnection(nil) // no-op

	assert.False(t, conn.IsOpen())
}

func TestSLCanConnectionClient_GetInterfaceName(t *testing.T) {
	network := "slcan"
	uri := "/dev/ttyUSB0"
	conn := NewSLCanConnection(context.Background(), testConfig(), "my-device", testChannel(), &network, &uri, nil)
	assert.Equal(t, "my-device-slcan-/dev/ttyUSB0", conn.GetInterfaceName())
}

func TestSLCanConnectionClient_ImplementsCanConnection(t *testing.T) {
	conn := NewSLCanConnection(context.Background(), testConfig(), "test-slcan", testChannel(), nil, nil, nil)
	var _ canModels.CanConnection = conn
}
