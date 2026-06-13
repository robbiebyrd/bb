package obdconn

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	canModels "github.com/robbiebyrd/cantou/internal/models"
	"github.com/robbiebyrd/cantou/internal/obd"
)

// fakeTransport is a no-op obd.Transport. Reads behave like an idle serial port
// (timeout → no data, no error).
type fakeTransport struct{}

func (fakeTransport) Read(p []byte) (int, error)  { time.Sleep(time.Millisecond); return 0, nil }
func (fakeTransport) Write(p []byte) (int, error) { return len(p), nil }
func (fakeTransport) Close() error                { return nil }

// reconnectDriver returns io.EOF from its first Monitor call (simulating a
// dropped link), then blocks until stop on subsequent calls.
type reconnectDriver struct{ monitors *int32 }

func (d reconnectDriver) Init() error                         { return nil }
func (d reconnectDriver) SetFilters([]uint32) error           { return nil }
func (d reconnectDriver) Request(string) ([]obd.Frame, error) { return nil, nil }
func (d reconnectDriver) Extended() bool                      { return false }
func (d reconnectDriver) Close() error                        { return nil }
func (d reconnectDriver) Monitor(_ func(obd.Frame), _ func(error), stop <-chan struct{}) error {
	if atomic.AddInt32(d.monitors, 1) == 1 {
		return io.EOF
	}
	<-stop
	return nil
}

func TestBase_ReconnectsAfterLinkDrop(t *testing.T) {
	var connects, monitors int32

	logger := slog.New(slog.DiscardHandler)
	factory := func(obd.Transport, obd.Protocol, bool, obd.Logf) Driver {
		return reconnectDriver{monitors: &monitors}
	}
	b := New(context.Background(), testConfig(), testChannel(), logger,
		canModels.CanInterfaceOption{Name: "mx", Network: canModels.NetworkSTN, URI: "rfcomm://AA:BB:CC:DD:EE:FF"},
		canModels.NetworkSTN, factory)
	b.openTransport = func(string, int, time.Duration) (obd.Transport, error) {
		atomic.AddInt32(&connects, 1)
		return fakeTransport{}, nil
	}

	require.NoError(t, b.Open()) // initial connect (connects == 1)

	var wg sync.WaitGroup
	b.Receive(&wg)

	// The first Monitor returns EOF, so the loop must reconnect at least once.
	require.Eventually(t, func() bool { return atomic.LoadInt32(&connects) >= 2 },
		2*time.Second, 5*time.Millisecond, "expected a reconnect after the link dropped")

	require.NoError(t, b.Close())
	wg.Wait()

	assert.GreaterOrEqual(t, atomic.LoadInt32(&monitors), int32(2))
}

func TestBase_CleanStopDoesNotReconnect(t *testing.T) {
	var connects int32

	logger := slog.New(slog.DiscardHandler)
	// A driver whose Monitor blocks until stop, then returns nil (clean stop).
	factory := func(obd.Transport, obd.Protocol, bool, obd.Logf) Driver {
		return blockingDriver{}
	}
	b := New(context.Background(), testConfig(), testChannel(), logger,
		canModels.CanInterfaceOption{Name: "mx", Network: canModels.NetworkSTN},
		canModels.NetworkSTN, factory)
	b.openTransport = func(string, int, time.Duration) (obd.Transport, error) {
		atomic.AddInt32(&connects, 1)
		return fakeTransport{}, nil
	}

	require.NoError(t, b.Open())
	var wg sync.WaitGroup
	b.Receive(&wg)
	time.Sleep(20 * time.Millisecond)
	require.NoError(t, b.Close())
	wg.Wait()

	// Only the initial connect; a clean stop must not trigger a reconnect.
	assert.Equal(t, int32(1), atomic.LoadInt32(&connects))
}

type blockingDriver struct{}

func (blockingDriver) Init() error                         { return nil }
func (blockingDriver) SetFilters([]uint32) error           { return nil }
func (blockingDriver) Request(string) ([]obd.Frame, error) { return nil, nil }
func (blockingDriver) Extended() bool                      { return false }
func (blockingDriver) Close() error                        { return nil }
func (blockingDriver) Monitor(_ func(obd.Frame), _ func(error), stop <-chan struct{}) error {
	<-stop
	return nil
}
