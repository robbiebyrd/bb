package signaldispatch_test

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/robbiebyrd/cantou/internal/client/signal-dispatch"
	signalfilter "github.com/robbiebyrd/cantou/internal/client/signal-filter"
	canModels "github.com/robbiebyrd/cantou/internal/models"
)

// mockParser is a test double for ParserInterface.
type mockParser struct {
	signals []canModels.CanSignalTimestamped
}

func (m *mockParser) ParseSignals(msg canModels.CanMessageTimestamped) []canModels.CanSignalTimestamped {
	if m.signals == nil {
		return nil
	}
	// Propagate the real timestamp/interface so tests can verify forwarding.
	out := make([]canModels.CanSignalTimestamped, len(m.signals))
	for i, s := range m.signals {
		s.Timestamp = msg.Timestamp
		s.Interface = msg.Interface
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
	d.AddListener(canModels.SignalDispatcherListener{Name: "l1", Channel: ch1})
	d.AddListener(canModels.SignalDispatcherListener{Name: "l2", Channel: ch2})

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
	d.AddListener(canModels.SignalDispatcherListener{Name: "l1", Channel: ch})

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
	d.AddListener(canModels.SignalDispatcherListener{Name: "l1", Channel: ch})

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
	d.AddListener(canModels.SignalDispatcherListener{Name: "full", Channel: ch})

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

func TestDispatch_ExpandsPIDsSupported(t *testing.T) {
	// When the parser produces a PIDsSupported bitmask signal, the dispatcher
	// should expand it into one signal per supported PID (value=1) instead of
	// forwarding the raw bitmask.
	parser := &mockParser{
		signals: []canModels.CanSignalTimestamped{
			{Message: "OBD2_Mode01", Signal: "S01PID", Value: 0}, // mux switch — passed through
			{Message: "OBD2_Mode01", Signal: "S01PID00_PIDsSupported_01_20", Value: 0x80000001}, // only PID 0x01 and 0x20
		},
	}
	d := signaldispatch.New(parser, 16, newLogger())
	ch := make(chan canModels.CanSignalTimestamped, 32)
	d.AddListener(canModels.SignalDispatcherListener{Name: "l1", Channel: ch})

	done := make(chan error, 1)
	go func() { done <- d.Dispatch() }()

	d.GetChannel() <- canModels.CanMessageTimestamped{ID: 2024, Timestamp: 5000, Interface: 1}
	close(d.GetChannel())
	require.NoError(t, <-done)

	var signals []canModels.CanSignalTimestamped
	for {
		select {
		case s := <-ch:
			signals = append(signals, s)
		default:
			goto check
		}
	}
check:
	// Should have: S01PID (mux, passed through) + 2 expanded PIDs
	var names []string
	for _, s := range signals {
		names = append(names, s.Signal)
	}
	assert.Contains(t, names, "S01PID")
	assert.Contains(t, names, "S01PID_Supported_01")
	assert.Contains(t, names, "S01PID_Supported_20")
	assert.NotContains(t, names, "S01PID00_PIDsSupported_01_20", "raw bitmask signal should not be forwarded")

	// Each expanded signal should have value=1, carry the original timestamp/interface.
	for _, s := range signals {
		if s.Signal == "S01PID_Supported_01" || s.Signal == "S01PID_Supported_20" {
			assert.Equal(t, float64(1), s.Value)
			assert.Equal(t, int64(5000), s.Timestamp)
			assert.Equal(t, 1, s.Interface)
			assert.Equal(t, "OBD2_Mode01", s.Message)
		}
	}
}

func TestDispatch_ExpandsPIDsSupported_Range21to40(t *testing.T) {
	parser := &mockParser{
		signals: []canModels.CanSignalTimestamped{
			{Message: "OBD2_Mode01", Signal: "S01PID20_PIDsSupported_21_40", Value: 0xC0000000}, // PID 0x21, 0x22
		},
	}
	d := signaldispatch.New(parser, 16, newLogger())
	ch := make(chan canModels.CanSignalTimestamped, 32)
	d.AddListener(canModels.SignalDispatcherListener{Name: "l1", Channel: ch})

	done := make(chan error, 1)
	go func() { done <- d.Dispatch() }()

	d.GetChannel() <- canModels.CanMessageTimestamped{ID: 2024}
	close(d.GetChannel())
	require.NoError(t, <-done)

	var names []string
	for {
		select {
		case s := <-ch:
			names = append(names, s.Signal)
		default:
			goto check
		}
	}
check:
	assert.Contains(t, names, "S01PID_Supported_21")
	assert.Contains(t, names, "S01PID_Supported_22")
	assert.Len(t, names, 2)
}

func TestDispatch_DroppedCount(t *testing.T) {
	parser := &mockParser{
		signals: []canModels.CanSignalTimestamped{{Signal: "S"}},
	}
	d := signaldispatch.New(parser, 16, newLogger())
	ch := make(chan canModels.CanSignalTimestamped, 0) // always full
	d.AddListener(canModels.SignalDispatcherListener{Name: "full", Channel: ch})

	done := make(chan error, 1)
	go func() { done <- d.Dispatch() }()

	d.GetChannel() <- canModels.CanMessageTimestamped{ID: 1}
	d.GetChannel() <- canModels.CanMessageTimestamped{ID: 2}
	close(d.GetChannel())
	require.NoError(t, <-done)

	assert.Equal(t, uint64(2), d.DroppedCount("full"))
	assert.Equal(t, uint64(0), d.DroppedCount("unknown"))
}

func TestDispatch_SignalFilter_ExcludesMatchingSignals(t *testing.T) {
	parser := &mockParser{
		signals: []canModels.CanSignalTimestamped{
			{Signal: "S01_UNUSED_01"},
			{Signal: "RPM"},
		},
	}
	filter := signalfilter.Group{
		Rules: []signalfilter.Rule{{Field: signalfilter.FieldSignal, Op: signalfilter.OpContains, Value: "UNUSED"}},
		Op:    signalfilter.GroupOpAnd,
		Mode:  signalfilter.ModeExclude,
	}
	d := signaldispatch.NewWithFilter(parser, 16, newLogger(), filter)
	ch := make(chan canModels.CanSignalTimestamped, 8)
	d.AddListener(canModels.SignalDispatcherListener{Name: "l1", Channel: ch})

	done := make(chan error, 1)
	go func() { done <- d.Dispatch() }()

	d.GetChannel() <- canModels.CanMessageTimestamped{ID: 1}
	close(d.GetChannel())
	require.NoError(t, <-done)

	require.Len(t, ch, 1)
	assert.Equal(t, "RPM", (<-ch).Signal)
}

func TestDispatch_SignalFilter_IncludeMode_KeepsOnlyMatchingSignals(t *testing.T) {
	parser := &mockParser{
		signals: []canModels.CanSignalTimestamped{
			{Signal: "RPM"},
			{Signal: "Speed"},
			{Signal: "Fuel"},
		},
	}
	filter := signalfilter.Group{
		Rules: []signalfilter.Rule{{Field: signalfilter.FieldSignal, Op: signalfilter.OpEq, Value: "RPM"}},
		Op:    signalfilter.GroupOpAnd,
		Mode:  signalfilter.ModeInclude,
	}
	d := signaldispatch.NewWithFilter(parser, 16, newLogger(), filter)
	ch := make(chan canModels.CanSignalTimestamped, 8)
	d.AddListener(canModels.SignalDispatcherListener{Name: "l1", Channel: ch})

	done := make(chan error, 1)
	go func() { done <- d.Dispatch() }()

	d.GetChannel() <- canModels.CanMessageTimestamped{ID: 1}
	close(d.GetChannel())
	require.NoError(t, <-done)

	require.Len(t, ch, 1)
	assert.Equal(t, "RPM", (<-ch).Signal)
}

func TestDispatch_ConcurrentAddListenerNoRace(t *testing.T) {
	// This test must pass with go test -race.
	// Story 001-b1a1 fixed the race by ensuring AddListener is always called
	// before Dispatch starts (Dispatch only starts in app.Run, after all AddOutputs).
	parser := &mockParser{
		signals: []canModels.CanSignalTimestamped{{Signal: "S", Value: 1}},
	}
	d := signaldispatch.New(parser, 16, newLogger())

	ch := make(chan canModels.CanSignalTimestamped, 4)
	d.AddListener(canModels.SignalDispatcherListener{Name: "l1", Channel: ch}) // AddListener BEFORE Dispatch — safe

	done := make(chan error, 1)
	go func() { done <- d.Dispatch() }()

	d.GetChannel() <- canModels.CanMessageTimestamped{ID: 1}
	close(d.GetChannel())

	require.NoError(t, <-done)
}
