package simulate

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

func testConfig() *canModels.Config {
	return &canModels.Config{CanInterfaceSeparator: "-", SimEmitRate: 1000000}
}

func testChannel() chan canModels.CanMessageTimestamped {
	return make(chan canModels.CanMessageTimestamped, 16)
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewSimulationCanClient_ValidParams(t *testing.T) {
	conn := NewSimulationCanClient(context.Background(), testConfig(), "sim1", testChannel(), silentLogger(), nil, nil, nil)
	require.NotNil(t, conn)
}

func TestNewSimulationCanClient_EmptyNamePanics(t *testing.T) {
	assert.Panics(t, func() {
		NewSimulationCanClient(context.Background(), testConfig(), "", testChannel(), silentLogger(), nil, nil, nil)
	})
}

func TestNewSimulationCanClient_NilChannelPanics(t *testing.T) {
	assert.Panics(t, func() {
		NewSimulationCanClient(context.Background(), testConfig(), "sim1", nil, silentLogger(), nil, nil, nil)
	})
}

func TestNewSimulationCanClient_DefaultsURIToName(t *testing.T) {
	conn := NewSimulationCanClient(context.Background(), testConfig(), "sim1", testChannel(), silentLogger(), nil, nil, nil)
	assert.Equal(t, "sim1", conn.GetURI())
}

func TestNewSimulationCanClient_DefaultsNetworkToSim(t *testing.T) {
	conn := NewSimulationCanClient(context.Background(), testConfig(), "sim1", testChannel(), silentLogger(), nil, nil, nil)
	assert.Equal(t, "sim", conn.GetNetwork())
}

func TestNewSimulationCanClient_UsesProvidedNetworkAndURI(t *testing.T) {
	network := "socketcan"
	uri := "vcan0"
	conn := NewSimulationCanClient(context.Background(), testConfig(), "my-iface", testChannel(), silentLogger(), &network, &uri, nil)
	assert.Equal(t, "socketcan", conn.GetNetwork())
	assert.Equal(t, "vcan0", conn.GetURI())
}

func TestSimulationCanClient_GettersAndSetters(t *testing.T) {
	conn := NewSimulationCanClient(context.Background(), testConfig(), "sim1", testChannel(), silentLogger(), nil, nil, nil)

	conn.SetID(7)
	assert.Equal(t, 7, conn.GetID())

	conn.SetName("renamed")
	assert.Equal(t, "renamed", conn.GetName())

	conn.SetURI("/dev/can0")
	assert.Equal(t, "/dev/can0", conn.GetURI())

	conn.SetNetwork("socketcan")
	assert.Equal(t, "socketcan", conn.GetNetwork())

	conn.SetConnection(nil)
	assert.Nil(t, conn.GetConnection())
}

func TestSimulationCanClient_DBCFilePath_SetIsNoop(t *testing.T) {
	conn := NewSimulationCanClient(context.Background(), testConfig(), "sim1", testChannel(), silentLogger(), nil, nil, nil)
	path := "/some/path.dbc"
	conn.SetDBCFilePath(&path)
	// SetDBCFilePath is a no-op on the sim client.
	assert.Nil(t, conn.GetDBCFilePath())
}

func TestSimulationCanClient_GetInterfaceName(t *testing.T) {
	network := "sim"
	uri := "vcan0"
	conn := NewSimulationCanClient(context.Background(), testConfig(), "my-sim", testChannel(), silentLogger(), &network, &uri, nil)
	assert.Equal(t, "my-sim-sim-vcan0", conn.GetInterfaceName())
}

func TestSimulationCanClient_OpenCloseIsOpen(t *testing.T) {
	conn := NewSimulationCanClient(context.Background(), testConfig(), "sim1", testChannel(), silentLogger(), nil, nil, nil)

	assert.False(t, conn.IsOpen())

	err := conn.Open()
	assert.NoError(t, err)
	assert.True(t, conn.IsOpen())

	err = conn.Close()
	assert.NoError(t, err)
	assert.False(t, conn.IsOpen())
}

func TestSimulationCanClient_Discontinue(t *testing.T) {
	conn := NewSimulationCanClient(context.Background(), testConfig(), "sim1", testChannel(), silentLogger(), nil, nil, nil)

	err := conn.Discontinue()
	assert.NoError(t, err)
}

func TestSimulationCanClient_ImplementsCanConnection(t *testing.T) {
	conn := NewSimulationCanClient(context.Background(), testConfig(), "sim1", testChannel(), silentLogger(), nil, nil, nil)
	var _ canModels.CanConnection = conn
}

func TestReceive_ExitsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan canModels.CanMessageTimestamped, 4)
	rate := 1 // 1ns between messages
	cfg := &canModels.Config{SimEmitRate: rate, MessageBufferSize: 4}
	client := NewSimulationCanClient(ctx, cfg, "test-sim", ch, silentLogger(), nil, nil, &rate)

	var wg sync.WaitGroup
	client.Receive(&wg)

	// Let it run briefly
	time.Sleep(5 * time.Millisecond)

	// Cancel context and verify goroutine exits
	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Good — goroutine exited
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Receive goroutine did not exit after context cancellation")
	}
}
