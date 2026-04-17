package signaldispatch_test

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/robbiebyrd/bb/internal/client/signaldispatch"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

// mockParser is a test double for ParserInterface.
type mockParser struct {
	signals []canModels.CanSignalTimestamped
}

func (m *mockParser) Parse(_ canModels.CanMessageData) any { return nil }

func (m *mockParser) ParseSignals(msg canModels.CanMessageData, timestamp int64, iface int) []canModels.CanSignalTimestamped {
	if m.signals == nil {
		return nil
	}
	// Propagate the real timestamp/interface so tests can verify forwarding.
	out := make([]canModels.CanSignalTimestamped, len(m.signals))
	for i, s := range m.signals {
		s.Timestamp = timestamp
		s.Interface = iface
		s.ID = msg.ID
		out[i] = s
	}
	return out
}

func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, nil))
}

func TestDispatch_FansOutSignals(t *testing.T) {
	parser := &mockParser{
		signals: []canModels.CanSignalTimestamped{
			{Signal: "RPM", Value: 3000, Unit: "rpm"},
			{Signal: "Speed", Value: 80, Unit: "km/h"},
		},
	}

	d := signaldispatch.New(parser, 16, newLogger())

	ch1 := make(chan canModels.CanSignalTimestamped, 8)
	ch2 := make(chan canModels.CanSignalTimestamped, 8)
	d.AddListener(ch1)
	d.AddListener(ch2)

	done := make(chan error, 1)
	go func() { done <- d.Dispatch() }()

	d.GetChannel() <- canModels.CanMessageTimestamped{ID: 0x141, Timestamp: 1000, Interface: 2}
	close(d.GetChannel())

	require.NoError(t, <-done)

	for _, ch := range []chan canModels.CanSignalTimestamped{ch1, ch2} {
		var names []string
		for sig := range ch {
			names = append(names, sig.Signal)
			if len(names) == 2 {
				break
			}
		}
		assert.ElementsMatch(t, []string{"RPM", "Speed"}, names)
	}
}

func TestDispatch_PropagatesTimestampAndInterface(t *testing.T) {
	parser := &mockParser{
		signals: []canModels.CanSignalTimestamped{{Signal: "RPM"}},
	}
	d := signaldispatch.New(parser, 16, newLogger())
	ch := make(chan canModels.CanSignalTimestamped, 4)
	d.AddListener(ch)

	go d.Dispatch() //nolint:errcheck
	d.GetChannel() <- canModels.CanMessageTimestamped{ID: 0x200, Timestamp: 9999, Interface: 5}
	close(d.GetChannel())

	sig := <-ch
	assert.Equal(t, int64(9999), sig.Timestamp)
	assert.Equal(t, 5, sig.Interface)
	assert.Equal(t, uint32(0x200), sig.ID)
}

func TestDispatch_UnknownMessageNoSend(t *testing.T) {
	parser := &mockParser{signals: nil} // returns nil for all messages
	d := signaldispatch.New(parser, 16, newLogger())
	ch := make(chan canModels.CanSignalTimestamped, 4)
	d.AddListener(ch)

	done := make(chan error, 1)
	go func() { done <- d.Dispatch() }()

	d.GetChannel() <- canModels.CanMessageTimestamped{ID: 0xDEAD}
	close(d.GetChannel())

	require.NoError(t, <-done)
	assert.Empty(t, ch)
}

func TestDispatch_DropsWhenListenerFull(t *testing.T) {
	parser := &mockParser{
		signals: []canModels.CanSignalTimestamped{{Signal: "S"}},
	}
	d := signaldispatch.New(parser, 16, newLogger())
	ch := make(chan canModels.CanSignalTimestamped, 0) // zero-capacity: always full
	d.AddListener(ch)

	done := make(chan error, 1)
	go func() { done <- d.Dispatch() }()

	// Send two messages — both should be dropped without blocking.
	d.GetChannel() <- canModels.CanMessageTimestamped{ID: 1}
	d.GetChannel() <- canModels.CanMessageTimestamped{ID: 2}
	close(d.GetChannel())

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Dispatch blocked on full listener channel")
	}
}

func TestDispatch_ExitsWhenChannelClosed(t *testing.T) {
	parser := &mockParser{}
	d := signaldispatch.New(parser, 4, newLogger())

	done := make(chan error, 1)
	go func() { done <- d.Dispatch() }()

	close(d.GetChannel())

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Dispatch did not exit after channel close")
	}
}
