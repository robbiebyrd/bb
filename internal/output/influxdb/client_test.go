package influxdb

import (
	"context"
	"io"
	"log/slog"
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
		messageBlock:    make([]canModels.CanMessageTimestamped, 0, maxBlocks),
		filters:         make(map[string]canModels.FilterInterface),
	}
}

// TestHandleChannel_FlushesOnMaxBlocks verifies that a full messageBlock is
// sent to internalChannel immediately without waiting for the ticker.
func TestHandleChannel_FlushesOnMaxBlocks(t *testing.T) {
	maxBlocks := 3
	c := newHandleClient(maxBlocks, 4, 5000, 4) // long ticker — only count flush fires

	go c.HandleChannel() //nolint:errcheck

	for i := 0; i < maxBlocks; i++ {
		c.incomingChannel <- testMsg(uint32(i))
	}

	select {
	case batch := <-c.internalChannel:
		if len(batch) != maxBlocks {
			t.Errorf("expected batch of %d, got %d", maxBlocks, len(batch))
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for count-based flush")
	}

	close(c.incomingChannel)
}

// TestHandleChannel_FlushesOnTicker verifies that a partial messageBlock is
// flushed when the ticker fires, without requiring maxBlocks to be reached.
func TestHandleChannel_FlushesOnTicker(t *testing.T) {
	flushMs := 50
	c := newHandleClient(100, 4, flushMs, 4) // high maxBlocks — only ticker fires

	go c.HandleChannel() //nolint:errcheck

	c.incomingChannel <- testMsg(0x001)

	deadline := time.Duration(flushMs)*time.Millisecond + 40*time.Millisecond
	select {
	case batch := <-c.internalChannel:
		if len(batch) != 1 {
			t.Errorf("expected batch of 1, got %d", len(batch))
		}
	case <-time.After(deadline):
		t.Fatalf("timed out after %v — ticker-based flush did not fire", deadline)
	}

	close(c.incomingChannel)
}

// TestHandleChannel_FlushesRemainingOnClose verifies that messages accumulated
// below maxBlocks are flushed when incomingChannel is closed.
func TestHandleChannel_FlushesRemainingOnClose(t *testing.T) {
	c := newHandleClient(100, 4, 5000, 4) // high maxBlocks, long ticker

	c.incomingChannel <- testMsg(0xAAA)
	c.incomingChannel <- testMsg(0xBBB)
	close(c.incomingChannel)

	done := make(chan error, 1)
	go func() {
		done <- c.HandleChannel()
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

	c.incomingChannel <- testMsg(0x100) // maxBlocks=1, so this triggers a flush
	close(c.incomingChannel)

	done := make(chan error, 1)
	go func() {
		done <- c.HandleChannel()
	}()

	select {
	case <-done:
		// good — returned without blocking
	case <-time.After(200 * time.Millisecond):
		t.Fatal("HandleChannel blocked on full internalChannel — should drop batch instead")
	}

	// The batch must have been dropped, not sent.
	select {
	case <-c.internalChannel:
		t.Error("expected batch to be dropped, but it was queued in internalChannel")
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
