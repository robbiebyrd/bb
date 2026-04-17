package influxdb

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

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
		ctx:             context.Background(),
		l:               silentLogger(),
		maxBlocks:       maxBlocks,
		maxConnections:  maxConnections,
		flushTime:       flushTimeMs,
		internalChannel: make(chan []canModels.CanMessageTimestamped, internalChanSize),
		incomingChannel: make(chan canModels.CanMessageTimestamped, 32),
		messageBlock:    nil,
		mu:              sync.Mutex{},
		workerLastRan:   time.Now(),
		filters:         make(map[string]canModels.FilterInterface),
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

	c.incomingChannel <- testMsg(0x100)
	c.incomingChannel <- testMsg(0x200)
	close(c.incomingChannel)

	done := make(chan error, 1)
	go func() {
		done <- c.HandleChannel()
	}()

	select {
	case <-done:
		// good — returned without blocking
	case <-time.After(200 * time.Millisecond):
		t.Fatal("HandleChannel blocked: internalChannel is unbuffered and no workers were reading")
	}
}
