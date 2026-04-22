package messagedispatch

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/robbiebyrd/cantou/internal/client/filter"
	canModels "github.com/robbiebyrd/cantou/internal/models"
)

func newTestClient() (*MessageDispatcher, chan canModels.CanMessageTimestamped) {
	incoming := make(chan canModels.CanMessageTimestamped, 16)
	ctx := context.Background()
	l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewMessageDispatcher(ctx, incoming, l), incoming
}

func testMsg(id uint32) canModels.CanMessageTimestamped {
	return canModels.CanMessageTimestamped{ID: id, Data: []byte{0x01}}
}

// Add — listener is registered successfully.
func TestAdd_Success(t *testing.T) {
	bc, _ := newTestClient()

	ch := make(chan canModels.CanMessageTimestamped, 1)
	err := bc.Add(canModels.MessageDispatcherListener{Name: "listener1", Channel: ch})
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
	_ = bc.Add(canModels.MessageDispatcherListener{Name: "dupe", Channel: ch})

	ch2 := make(chan canModels.CanMessageTimestamped, 1)
	err := bc.Add(canModels.MessageDispatcherListener{Name: "dupe", Channel: ch2})
	if err == nil {
		t.Fatal("expected error for duplicate name, got nil")
	}
}

// Remove — removes an existing listener.
func TestRemove_Success(t *testing.T) {
	bc, _ := newTestClient()

	ch := make(chan canModels.CanMessageTimestamped, 1)
	_ = bc.Add(canModels.MessageDispatcherListener{Name: "toRemove", Channel: ch})

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
	bc := NewMessageDispatcher(ctx, incoming, l)

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
	_ = bc.Add(canModels.MessageDispatcherListener{Name: "l1", Channel: ch1})
	_ = bc.Add(canModels.MessageDispatcherListener{Name: "l2", Channel: ch2})

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

// Broadcast — AND filter blocks a message that does not match all conditions.
func TestBroadcast_AndFilterBlocksNonMatchingMessage(t *testing.T) {
	bc, incoming := newTestClient()

	ch := make(chan canModels.CanMessageTimestamped, 4)
	// Filter requires interface == 0 AND interface == 1 — impossible, so all messages are blocked.
	filterGroup := &canModels.CanMessageFilterGroup{
		Operator: canModels.FilterAnd,
		Filters: []canModels.CanMessageFilter{
			filter.CanInterfaceFilter{Value: 0},
			filter.CanInterfaceFilter{Value: 1},
		},
	}
	_ = bc.Add(canModels.MessageDispatcherListener{Name: "filtered", Channel: ch, Filter: filterGroup})

	go bc.Broadcast() //nolint:errcheck

	// Message with Interface=0 satisfies only the first filter, not both.
	msg := canModels.CanMessageTimestamped{ID: 0x100, Interface: 0, Data: []byte{0x01}}
	incoming <- msg

	select {
	case <-ch:
		t.Fatal("AND filter: message should have been blocked but was delivered")
	case <-time.After(100 * time.Millisecond):
		// expected — message was correctly blocked
	}
}

// Broadcast — OR filter passes a message that matches at least one condition.
func TestBroadcast_OrFilterAllowsPartiallyMatchingMessage(t *testing.T) {
	bc, incoming := newTestClient()

	ch := make(chan canModels.CanMessageTimestamped, 4)
	// Filter passes if interface == 0 OR interface == 1.
	filterGroup := &canModels.CanMessageFilterGroup{
		Operator: canModels.FilterOr,
		Filters: []canModels.CanMessageFilter{
			filter.CanInterfaceFilter{Value: 0},
			filter.CanInterfaceFilter{Value: 1},
		},
	}
	_ = bc.Add(canModels.MessageDispatcherListener{Name: "filtered", Channel: ch, Filter: filterGroup})

	go bc.Broadcast() //nolint:errcheck

	// Message with Interface=1 satisfies the second filter, so it should pass.
	msg := canModels.CanMessageTimestamped{ID: 0x200, Interface: 1, Data: []byte{0x01}}
	incoming <- msg

	select {
	case got := <-ch:
		if got.ID != 0x200 {
			t.Errorf("OR filter: expected ID 0x200, got 0x%X", got.ID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("OR filter: timed out waiting for message that should have passed")
	}
}

// testFilterGroup — AND requires all filters true; OR requires at least one true.
func TestFilterGroup_AndVsOrOperatorBehavior(t *testing.T) {
	bc, _ := newTestClient()

	matchInterface0 := canModels.CanMessageTimestamped{Interface: 0, Data: []byte{0x01}}
	matchInterface1 := canModels.CanMessageTimestamped{Interface: 1, Data: []byte{0x01}}

	andListener := canModels.MessageDispatcherListener{
		Name:    "and",
		Channel: make(chan canModels.CanMessageTimestamped, 1),
		Filter: &canModels.CanMessageFilterGroup{
			Operator: canModels.FilterAnd,
			Filters: []canModels.CanMessageFilter{
				filter.CanInterfaceFilter{Value: 0},
				filter.CanInterfaceFilter{Value: 1},
			},
		},
	}

	orListener := canModels.MessageDispatcherListener{
		Name:    "or",
		Channel: make(chan canModels.CanMessageTimestamped, 1),
		Filter: &canModels.CanMessageFilterGroup{
			Operator: canModels.FilterOr,
			Filters: []canModels.CanMessageFilter{
				filter.CanInterfaceFilter{Value: 0},
				filter.CanInterfaceFilter{Value: 1},
			},
		},
	}

	// AND: interface 0 satisfies only first filter — should be blocked.
	if bc.testFilterGroup(andListener, matchInterface0) {
		t.Error("AND: message matching only one filter should not pass")
	}

	// OR: interface 0 satisfies the first filter — should pass.
	if !bc.testFilterGroup(orListener, matchInterface0) {
		t.Error("OR: message matching one filter should pass")
	}

	// OR: interface 1 satisfies the second filter — should pass.
	if !bc.testFilterGroup(orListener, matchInterface1) {
		t.Error("OR: message matching one filter should pass")
	}

	// AND: a message that matches neither filter.
	matchInterface2 := canModels.CanMessageTimestamped{Interface: 2, Data: []byte{0x01}}
	if bc.testFilterGroup(orListener, matchInterface2) {
		t.Error("OR: message matching no filters should not pass")
	}
}

// Broadcast — full listener channel is skipped without blocking.
func TestBroadcast_DropsWhenListenerChannelFull(t *testing.T) {
	bc, incoming := newTestClient()

	// Channel with capacity 1 — fill it first so subsequent sends drop.
	fullCh := make(chan canModels.CanMessageTimestamped, 1)
	fullCh <- testMsg(0x000) // pre-fill so channel is full

	okCh := make(chan canModels.CanMessageTimestamped, 4)

	_ = bc.Add(canModels.MessageDispatcherListener{Name: "full", Channel: fullCh})
	_ = bc.Add(canModels.MessageDispatcherListener{Name: "ok", Channel: okCh})

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
