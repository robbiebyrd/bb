package broadcast

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

func newTestClient() (*BroadcastClient, chan canModels.CanMessageTimestamped) {
	incoming := make(chan canModels.CanMessageTimestamped, 16)
	ctx := context.Background()
	l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewBroadcastClient(ctx, incoming, l), incoming
}

func testMsg(id uint32) canModels.CanMessageTimestamped {
	return canModels.CanMessageTimestamped{ID: id, Data: []byte{0x01}}
}

// Add — listener is registered successfully.
func TestAdd_Success(t *testing.T) {
	bc, _ := newTestClient()

	ch := make(chan canModels.CanMessageTimestamped, 1)
	err := bc.Add(BroadcastClientListener{Name: "listener1", Channel: ch})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(bc.broadcastChannels) != 1 {
		t.Fatalf("expected 1 listener, got %d", len(bc.broadcastChannels))
	}
}

// Add — duplicate name returns an error.
func TestAdd_DuplicateNameReturnsError(t *testing.T) {
	bc, _ := newTestClient()

	ch := make(chan canModels.CanMessageTimestamped, 1)
	_ = bc.Add(BroadcastClientListener{Name: "dupe", Channel: ch})

	ch2 := make(chan canModels.CanMessageTimestamped, 1)
	err := bc.Add(BroadcastClientListener{Name: "dupe", Channel: ch2})
	if err == nil {
		t.Fatal("expected error for duplicate name, got nil")
	}
}

// Remove — removes an existing listener.
func TestRemove_Success(t *testing.T) {
	bc, _ := newTestClient()

	ch := make(chan canModels.CanMessageTimestamped, 1)
	_ = bc.Add(BroadcastClientListener{Name: "toRemove", Channel: ch})

	err := bc.Remove("toRemove")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(bc.broadcastChannels) != 0 {
		t.Fatalf("expected 0 listeners after removal, got %d", len(bc.broadcastChannels))
	}
}

// Remove — returns error for unknown name.
func TestRemove_UnknownNameReturnsError(t *testing.T) {
	bc, _ := newTestClient()

	err := bc.Remove("ghost")
	if err == nil {
		t.Fatal("expected error for unknown listener name, got nil")
	}
}

// Broadcast — exits when the context is cancelled (no goroutine leak).
func TestBroadcast_ExitsOnContextCancel(t *testing.T) {
	incoming := make(chan canModels.CanMessageTimestamped, 4)
	ctx, cancel := context.WithCancel(context.Background())
	l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	bc := NewBroadcastClient(ctx, incoming, l)

	done := make(chan error, 1)
	go func() {
		done <- bc.Broadcast()
	}()

	cancel()

	select {
	case <-done:
		// good — Broadcast returned after context cancel
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Broadcast did not exit after context cancellation — goroutine leak")
	}
}

// Broadcast — messages are fanned out to all registered listeners.
func TestBroadcast_FansOutToAllListeners(t *testing.T) {
	bc, incoming := newTestClient()

	ch1 := make(chan canModels.CanMessageTimestamped, 4)
	ch2 := make(chan canModels.CanMessageTimestamped, 4)
	_ = bc.Add(BroadcastClientListener{Name: "l1", Channel: ch1})
	_ = bc.Add(BroadcastClientListener{Name: "l2", Channel: ch2})

	go bc.Broadcast() //nolint:errcheck

	incoming <- testMsg(0x100)

	timeout := time.After(500 * time.Millisecond)
	for _, ch := range []chan canModels.CanMessageTimestamped{ch1, ch2} {
		select {
		case msg := <-ch:
			if msg.ID != 0x100 {
				t.Errorf("expected ID 0x100, got 0x%X", msg.ID)
			}
		case <-timeout:
			t.Fatal("timed out waiting for message on listener channel")
		}
	}
}

// Broadcast — full listener channel is skipped without blocking.
func TestBroadcast_DropsWhenListenerChannelFull(t *testing.T) {
	bc, incoming := newTestClient()

	// Channel with capacity 1 — fill it first so subsequent sends drop.
	fullCh := make(chan canModels.CanMessageTimestamped, 1)
	fullCh <- testMsg(0x000) // pre-fill so channel is full

	okCh := make(chan canModels.CanMessageTimestamped, 4)

	_ = bc.Add(BroadcastClientListener{Name: "full", Channel: fullCh})
	_ = bc.Add(BroadcastClientListener{Name: "ok", Channel: okCh})

	go bc.Broadcast() //nolint:errcheck

	incoming <- testMsg(0x200)

	// The ok listener must still receive the message despite the other being full.
	select {
	case msg := <-okCh:
		if msg.ID != 0x200 {
			t.Errorf("expected ID 0x200, got 0x%X", msg.ID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for message — Broadcast() may have blocked on full channel")
	}
}
