package influxdb

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/robbiebyrd/cantou/internal/client/common"
	canModels "github.com/robbiebyrd/cantou/internal/models"
)

// mockCanConn implements canModels.CanConnection for testing.
type mockCanConn struct {
	interfaceName string
}

func (m *mockCanConn) SetID(_ int)                {}
func (m *mockCanConn) GetName() string            { return "" }
func (m *mockCanConn) GetInterfaceName() string   { return m.interfaceName }
func (m *mockCanConn) Open() error                { return nil }
func (m *mockCanConn) Close() error               { return nil }
func (m *mockCanConn) Receive(_ *sync.WaitGroup)  {}

// mockResolver implements canModels.InterfaceResolver for testing.
type mockResolver struct {
	conns map[int]*mockCanConn
}

func (r *mockResolver) ConnectionByID(id int) canModels.CanConnection {
	if c, ok := r.conns[id]; ok {
		return c
	}
	return nil
}

// mockFilterAlwaysTrue returns true (filter/drop) for every message.
type mockFilterAlwaysTrue struct{}

func (m *mockFilterAlwaysTrue) Filter(_ canModels.CanMessageTimestamped) bool { return true }

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testMsg(id uint32) canModels.CanMessageTimestamped {
	return canModels.CanMessageTimestamped{ID: id, Data: []byte{0x01, 0x02}}
}

// newHandleClient builds an InfluxDBClient for HandleChannel tests without a
// real InfluxDB connection. internalChanSize controls the internalChannel buffer.
func newHandleClient(maxBlocks, maxConnections, flushTimeMs, internalChanSize int) *InfluxDBClient {
	return &InfluxDBClient{
		ctx:                   context.Background(),
		l:                     silentLogger(),
		maxBlocks:             maxBlocks,
		maxConnections:        maxConnections,
		flushTime:             flushTimeMs,
		internalChannel:       make(chan []canModels.CanMessageTimestamped, internalChanSize),
		signalInternalChannel: make(chan []canModels.CanSignalTimestamped, internalChanSize),
		canChannel:            make(chan canModels.CanMessageTimestamped, 32),
		signalChannel:         make(chan canModels.CanSignalTimestamped, 32),
		messageBlock:          make([]canModels.CanMessageTimestamped, 0, maxBlocks),
		signalBlock:           make([]canModels.CanSignalTimestamped, 0, maxBlocks),
		filters:               common.NewFilterSet(),
	}
}

func testSignal(id uint32) canModels.CanSignalTimestamped {
	return canModels.CanSignalTimestamped{ID: id, Signal: "RPM", Value: 1000}
}

// TestHandleChannel_FlushesOnMaxBlocks verifies that a full messageBlock is
// sent to internalChannel immediately without waiting for the ticker.
func TestHandleChannel_FlushesOnMaxBlocks(t *testing.T) {
	maxBlocks := 3
	c := newHandleClient(maxBlocks, 4, 5000, 4) // long ticker — only count flush fires

	go c.HandleCanMessageChannel() //nolint:errcheck

	for i := 0; i < maxBlocks; i++ {
		c.canChannel <- testMsg(uint32(i))
	}

	select {
	case batch := <-c.internalChannel:
		if len(batch) != maxBlocks {
			t.Errorf("expected batch of %d, got %d", maxBlocks, len(batch))
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for count-based flush")
	}

	close(c.canChannel)
}

// TestHandleChannel_FlushesOnTicker verifies that a partial messageBlock is
// flushed when the ticker fires, without requiring maxBlocks to be reached.
func TestHandleChannel_FlushesOnTicker(t *testing.T) {
	flushMs := 50
	c := newHandleClient(100, 4, flushMs, 4) // high maxBlocks — only ticker fires

	go c.HandleCanMessageChannel() //nolint:errcheck

	c.canChannel <- testMsg(0x001)

	deadline := time.Duration(flushMs)*time.Millisecond + 40*time.Millisecond
	select {
	case batch := <-c.internalChannel:
		if len(batch) != 1 {
			t.Errorf("expected batch of 1, got %d", len(batch))
		}
	case <-time.After(deadline):
		t.Fatalf("timed out after %v — ticker-based flush did not fire", deadline)
	}

	close(c.canChannel)
}

// TestHandleChannel_FlushesRemainingOnClose verifies that messages accumulated
// below maxBlocks are flushed when incomingChannel is closed.
func TestHandleChannel_FlushesRemainingOnClose(t *testing.T) {
	c := newHandleClient(100, 4, 5000, 4) // high maxBlocks, long ticker

	c.canChannel <- testMsg(0xAAA)
	c.canChannel <- testMsg(0xBBB)
	close(c.canChannel)

	done := make(chan error, 1)
	go func() {
		done <- c.HandleCanMessageChannel()
	}()

	select {
	case batch := <-c.internalChannel:
		if len(batch) != 2 {
			t.Errorf("expected remaining batch of 2, got %d", len(batch))
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for flush-on-close")
	}

	<-done
}

// TestHandleChannel_DropsWhenWorkerQueueFull verifies that HandleChannel does
// not block when internalChannel has no capacity (all workers busy, buffer full).
// The batch must be dropped with a log warning rather than stalling the consumer.
func TestHandleChannel_DropsWhenWorkerQueueFull(t *testing.T) {
	// size=0: unbuffered and no workers — any blocking send would deadlock.
	c := newHandleClient(1, 4, 5000, 0)

	c.canChannel <- testMsg(0x100) // maxBlocks=1, so this triggers a flush
	close(c.canChannel)

	done := make(chan error, 1)
	go func() {
		done <- c.HandleCanMessageChannel()
	}()

	select {
	case <-done:
		// good — returned without blocking
	case <-time.After(200 * time.Millisecond):
		t.Fatal("HandleChannel blocked on full internalChannel — should drop batch instead")
	}

	// The batch must have been dropped, not sent. Use the two-value receive to
	// distinguish a real queued batch from the zero value returned when the
	// channel is closed and empty.
	select {
	case batch, ok := <-c.internalChannel:
		if ok {
			t.Errorf("expected batch to be dropped, but it was queued in internalChannel: %v", batch)
		}
		// channel closed and empty — correct
	default:
		// correct — nothing queued
	}
}

// TestHandleChannel_DoesNotBlockWhenWorkersAreBusy verifies that HandleChannel
// does not block when no workers are reading from internalChannel. This
// requires internalChannel to be buffered with at least maxConnections slots.
func TestHandleChannel_DoesNotBlockWhenWorkersAreBusy(t *testing.T) {
	// maxBlocks=1 means every message triggers a flush.
	// With 2 messages and no workers, a buffered internalChannel (size=2)
	// absorbs both flushes. An unbuffered channel deadlocks.
	c := newHandleClient(1, 2, 5000, 2) // size=maxConnections — buffered post-fix

	c.canChannel <- testMsg(0x100)
	c.canChannel <- testMsg(0x200)
	close(c.canChannel)

	done := make(chan error, 1)
	go func() {
		done <- c.HandleCanMessageChannel()
	}()

	select {
	case <-done:
		// good — returned without blocking
	case <-time.After(200 * time.Millisecond):
		t.Fatal("HandleChannel blocked: internalChannel is unbuffered and no workers were reading")
	}
}

func TestBoolToInt(t *testing.T) {
	assert.Equal(t, uint8(1), boolToInt(true))
	assert.Equal(t, uint8(0), boolToInt(false))
}

func TestAddFilter(t *testing.T) {
	c := newHandleClient(10, 2, 100, 4)
	f := &mockFilterAlwaysTrue{}

	err := c.AddFilter("my-filter", f)
	require.NoError(t, err)

	err = c.AddFilter("my-filter", f)
	assert.Error(t, err, "duplicate filter name should return an error")
	assert.Contains(t, err.Error(), "filter group already exists")

	err = c.AddFilter("another-filter", f)
	require.NoError(t, err)
}

func TestGetChannelAndName(t *testing.T) {
	c := newHandleClient(10, 2, 100, 4)
	assert.NotNil(t, c.GetChannel())
	assert.Equal(t, "output-influxdb3", c.GetName())
}

func TestShouldFilterMessage_NoFilters(t *testing.T) {
	c := newHandleClient(10, 2, 100, 4)
	msg := testMsg(0x100)

	shouldFilter, name := c.filters.ShouldFilter(msg)
	assert.False(t, shouldFilter)
	assert.Empty(t, name)
}

func TestShouldFilterMessage_MatchingFilter(t *testing.T) {
	c := newHandleClient(10, 2, 100, 4)
	_ = c.AddFilter("drop-all", &mockFilterAlwaysTrue{})

	msg := testMsg(0x100)
	shouldFilter, name := c.filters.ShouldFilter(msg)
	assert.True(t, shouldFilter)
	assert.Equal(t, "drop-all", name)
}

func TestConvertMsg(t *testing.T) {
	resolver := &mockResolver{
		conns: map[int]*mockCanConn{
			0: {interfaceName: "can0-can-vcan0"},
		},
	}
	c := newHandleClient(10, 2, 100, 4)
	c.resolver = resolver
	c.measurementName = "can_message"

	msg := canModels.CanMessageTimestamped{
		Timestamp: 1_000_000_000, // 1 second in nanoseconds
		Interface: 0,
		ID:        0x1AB,
		Length:    2,
		Data:      []byte{0xDE, 0xAD},
		Remote:    false,
		Transmit:  true,
	}

	result := c.convertMsg(msg)

	assert.Equal(t, time.Unix(0, 1_000_000_000), result.Timestamp)
	assert.Equal(t, "1ab", result.ID)
	assert.Equal(t, uint8(2), result.Length)
	assert.Equal(t, "222,173", result.Data)
	assert.Equal(t, uint8(0), result.Remote)
	assert.Equal(t, uint8(1), result.Transmit)
	assert.Equal(t, "can0-can-vcan0", result.Interface)
	assert.Equal(t, "can_message", result.Measurement)
}

// TestConvertMsg_ExtendedID verifies that convertMsg correctly formats 29-bit
// extended CAN IDs (> 0xFFF). The %03x format must expand beyond 3 digits
// without truncation — e.g. 0x18DAF110 must produce "18daf110", not "110".
func TestConvertMsg_ExtendedID(t *testing.T) {
	tests := []struct {
		name     string
		id       uint32
		expected string
	}{
		{
			name:     "standard 11-bit ID",
			id:       0x1AB,
			expected: "1ab",
		},
		{
			name:     "extended 29-bit OBD2 functional request ID",
			id:       0x18DAF110,
			expected: "18daf110",
		},
		{
			name:     "extended 29-bit ID at max 29-bit value",
			id:       0x1FFFFFFF,
			expected: "1fffffff",
		},
		{
			name:     "extended 29-bit ID boundary (just above 0xFFF)",
			id:       0x1000,
			expected: "1000",
		},
	}

	resolver := &mockResolver{
		conns: map[int]*mockCanConn{
			0: {interfaceName: "can0-can-vcan0"},
		},
	}
	c := newHandleClient(10, 2, 100, 4)
	c.resolver = resolver
	c.measurementName = "can_message"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := canModels.CanMessageTimestamped{
				Timestamp: 1_000_000_000,
				Interface: 0,
				ID:        tt.id,
				Length:    1,
				Data:      []byte{0x00},
			}
			result := c.convertMsg(msg)
			assert.Equal(t, tt.expected, result.ID)
		})
	}
}

func TestConvertMsg_UnknownInterface(t *testing.T) {
	c := newHandleClient(10, 2, 100, 4)
	c.resolver = &mockResolver{conns: map[int]*mockCanConn{}}

	msg := canModels.CanMessageTimestamped{
		Interface: 99,
		ID:        0x001,
		Length:    1,
		Data:      []byte{0x00},
	}

	result := c.convertMsg(msg)
	assert.Equal(t, "", result.Interface, "unknown interface should produce empty interface name")
}

func TestHandleCanMessageChannel_WorkersDrainBeforeReturn(t *testing.T) {
	// A simple test: if we close the incoming channel, HandleCanMessageChannel
	// should return only after all workers have finished processing.
	// We verify by checking that the function returns without hanging.
	// Note: this test validates the shutdown sequence, not actual writes.
	ctx := context.Background()
	c := &InfluxDBClient{
		canChannel: make(chan canModels.CanMessageTimestamped, 4),
		internalChannel: make(chan []canModels.CanMessageTimestamped, 2),
		messageBlock:    make([]canModels.CanMessageTimestamped, 0, 10),
		maxBlocks:       10,
		flushTime:       50,
		l:               silentLogger(),
		ctx:             ctx,
		filters:         common.NewFilterSet(),
	}

	// Start a fake worker that reads from internalChannel and signals done.
	workerDone := make(chan struct{})
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for range c.internalChannel {
			// drain
		}
		close(workerDone)
	}()

	done := make(chan struct{})
	go func() {
		close(c.canChannel)
		_ = c.HandleCanMessageChannel()
		close(done)
	}()

	select {
	case <-done:
		// Good — HandleCanMessageChannel returned
	case <-time.After(2 * time.Second):
		t.Fatal("HandleCanMessageChannel did not return after incoming channel closed")
	}

	// Worker should also be done (internalChannel was closed)
	select {
	case <-workerDone:
	case <-time.After(time.Second):
		t.Fatal("worker goroutine did not exit after internalChannel closed")
	}
}

func TestHandleSignalChannel_FlushesOnMaxBlocks(t *testing.T) {
	maxBlocks := 3
	c := newHandleClient(maxBlocks, 4, 5000, 4) // long ticker — only count flush fires

	go c.HandleSignalChannel() //nolint:errcheck

	for i := 0; i < maxBlocks; i++ {
		c.signalChannel <- testSignal(uint32(i))
	}

	select {
	case batch := <-c.signalInternalChannel:
		assert.Len(t, batch, maxBlocks)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for count-based signal flush")
	}

	close(c.signalChannel)
}

func TestHandleSignalChannel_FlushesOnTicker(t *testing.T) {
	flushMs := 50
	c := newHandleClient(100, 4, flushMs, 4) // high maxBlocks — only ticker fires

	go c.HandleSignalChannel() //nolint:errcheck

	c.signalChannel <- testSignal(0x001)

	deadline := time.Duration(flushMs)*time.Millisecond + 40*time.Millisecond
	select {
	case batch := <-c.signalInternalChannel:
		assert.Len(t, batch, 1)
	case <-time.After(deadline):
		t.Fatalf("timed out after %v — ticker-based signal flush did not fire", deadline)
	}

	close(c.signalChannel)
}

func TestHandleSignalChannel_FlushesRemainingOnClose(t *testing.T) {
	c := newHandleClient(100, 4, 5000, 4) // high maxBlocks, long ticker

	c.signalChannel <- testSignal(0x001)
	c.signalChannel <- testSignal(0x002)
	close(c.signalChannel)

	done := make(chan error, 1)
	go func() { done <- c.HandleSignalChannel() }()

	select {
	case batch := <-c.signalInternalChannel:
		assert.Len(t, batch, 2)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for signal flush-on-close")
	}

	<-done
}

func TestHandleSignalChannel_DropsWhenWorkerQueueFull(t *testing.T) {
	// size=0: unbuffered and no workers — any blocking send would deadlock.
	c := newHandleClient(1, 4, 5000, 0)

	c.signalChannel <- testSignal(0x100) // maxBlocks=1, triggers flush
	close(c.signalChannel)

	done := make(chan error, 1)
	go func() { done <- c.HandleSignalChannel() }()

	select {
	case <-done:
		// good — returned without blocking
	case <-time.After(200 * time.Millisecond):
		t.Fatal("HandleSignalChannel blocked on full signalInternalChannel — should drop batch instead")
	}
}

func TestGetSignalChannelAndHandleSignalChannel(t *testing.T) {
	c := newHandleClient(10, 2, 100, 4)

	ch := c.GetSignalChannel()
	require.NotNil(t, ch)

	// HandleSignalChannel must drain the channel and return when it's closed.
	sig := canModels.CanSignalTimestamped{
		Timestamp: 1_000_000_000,
		Interface: 0,
		Message:   "ENGINE",
		Signal:    "RPM",
		Value:     1500.5,
		Unit:      "rpm",
	}
	ch <- sig
	close(ch)

	err := c.HandleSignalChannel()
	require.NoError(t, err)
}

func TestHandleSignal_NilSignalClient(t *testing.T) {
	// When no signal database is configured (signalClient == nil), HandleSignal
	// must be a no-op and must not panic.
	c := newHandleClient(10, 2, 100, 4)
	c.HandleSignal(canModels.CanSignalTimestamped{Signal: "RPM", Value: 1000})
}

func TestConvertSignal(t *testing.T) {
	resolver := &mockResolver{
		conns: map[int]*mockCanConn{
			0: {interfaceName: "can0-can-vcan0"},
		},
	}
	c := newHandleClient(10, 2, 100, 4)
	c.resolver = resolver
	c.signalMeasurementName = "can_signal"

	sig := canModels.CanSignalTimestamped{
		Timestamp: 1_000_000_000,
		Interface: 0,
		ID:        0x1AB,
		Message:   "ENGINE",
		Signal:    "RPM",
		Value:     1500.5,
		Unit:      "rpm",
	}

	result := c.convertSignal(sig)

	assert.Equal(t, time.Unix(0, 1_000_000_000), result.Timestamp)
	assert.Equal(t, "can0-can-vcan0", result.Interface)
	assert.Equal(t, "ENGINE", result.Message)
	assert.Equal(t, "RPM", result.Signal)
	assert.InDelta(t, 1500.5, result.Value, 0.001)
	assert.Equal(t, "rpm", result.Unit)
	assert.Equal(t, "can_signal", result.Measurement)
}

func TestConvertSignal_UnknownInterface(t *testing.T) {
	c := newHandleClient(10, 2, 100, 4)
	c.resolver = &mockResolver{conns: map[int]*mockCanConn{}}

	result := c.convertSignal(canModels.CanSignalTimestamped{Interface: 99, Signal: "RPM"})
	assert.Equal(t, "", result.Interface)
}

func TestConvertMany(t *testing.T) {
	c := newHandleClient(10, 2, 100, 4)
	c.resolver = &mockResolver{conns: map[int]*mockCanConn{}}

	msgs := []canModels.CanMessageTimestamped{
		testMsg(0x100),
		testMsg(0x200),
		testMsg(0x300),
	}

	results := c.convertMany(msgs)
	require.Len(t, results, 3)
	assert.Equal(t, "100", results[0].ID)
	assert.Equal(t, "200", results[1].ID)
	assert.Equal(t, "300", results[2].ID)
}
