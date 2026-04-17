package app_test

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/robbiebyrd/bb/internal/app"
	"github.com/robbiebyrd/bb/internal/client/signaldispatch"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

// mockParser satisfies ParserInterface with no-op behaviour.
type mockParser struct{}

func (m *mockParser) Parse(_ canModels.CanMessageData) any { return nil }
func (m *mockParser) ParseSignals(_ canModels.CanMessageData, _ int64, _ int) []canModels.CanSignalTimestamped {
	return nil
}

// mockSignalClient implements SignalOutputClient so we can verify signal wiring.
type mockSignalClient struct {
	signalCh chan canModels.CanSignalTimestamped
	msgCh    chan canModels.CanMessageTimestamped
	started  chan struct{}
}

func newMockSignalClient() *mockSignalClient {
	return &mockSignalClient{
		signalCh: make(chan canModels.CanSignalTimestamped, 4),
		msgCh:    make(chan canModels.CanMessageTimestamped, 4),
		started:  make(chan struct{}, 2),
	}
}

func (m *mockSignalClient) Run() error                                      { return nil }
func (m *mockSignalClient) HandleCanMessage(_ canModels.CanMessageTimestamped) {}
func (m *mockSignalClient) HandleCanMessageChannel() error {
	m.started <- struct{}{}
	for range m.msgCh {
	}
	return nil
}
func (m *mockSignalClient) GetChannel() chan canModels.CanMessageTimestamped { return m.msgCh }
func (m *mockSignalClient) GetName() string                                  { return "mock-signal" }
func (m *mockSignalClient) AddFilter(_ string, _ canModels.FilterInterface) error { return nil }
func (m *mockSignalClient) HandleSignal(_ canModels.CanSignalTimestamped)        {}
func (m *mockSignalClient) HandleSignalChannel() error {
	m.started <- struct{}{}
	for range m.signalCh {
	}
	return nil
}
func (m *mockSignalClient) GetSignalChannel() chan canModels.CanSignalTimestamped { return m.signalCh }

func newTestApp(t *testing.T) canModels.AppInterface {
	t.Helper()
	lvl := new(slog.LevelVar)
	cfg := &canModels.Config{MessageBufferSize: 16}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	return app.NewApp(cfg, logger, lvl)
}

func TestAddOutput_SignalClientWiredWhenDispatcherSet(t *testing.T) {
	a := newTestApp(t)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	dispatcher := signaldispatch.New(&mockParser{}, 16, logger)
	a.AddSignalDispatcher(dispatcher, 0)

	client := newMockSignalClient()
	a.AddOutput(client)

	// Both HandleCanMessageChannel and HandleSignalChannel goroutines should start.
	// They block on range loops; count the started signals.
	count := 0
	timeout := make(chan struct{})
	go func() {
		for i := 0; i < 2; i++ {
			<-client.started
			count++
		}
		close(timeout)
	}()
	<-timeout

	assert.Equal(t, 2, count, "expected both HandleCanMessageChannel and HandleSignalChannel goroutines started")

	// Signal channel should be registered with the dispatcher.
	assert.NotNil(t, dispatcher.GetChannel())
}

func TestAddOutput_SignalClientNotWiredWithoutDispatcher(t *testing.T) {
	a := newTestApp(t)
	// No SetSignalDispatcher called.

	client := newMockSignalClient()
	a.AddOutput(client)

	// Only HandleCanMessageChannel starts; HandleSignalChannel must NOT start.
	select {
	case <-client.started:
		// first one is HandleCanMessageChannel — expected
	}
	// A second start would mean HandleSignalChannel was also launched — that's wrong.
	select {
	case <-client.started:
		t.Fatal("HandleSignalChannel goroutine started without a dispatcher")
	default:
		// Correct — only one goroutine started.
	}
}

// TestAddSignalDispatcher_NoRaceWithAddOutput verifies that AddSignalDispatcher
// does not start the Dispatch goroutine immediately, which would race with the
// AddListener write performed by AddOutput. Before the fix, Dispatch was started
// inside AddSignalDispatcher; AddOutput's AddListener call then wrote to
// d.listeners concurrently, causing a data race detectable with -race.
func TestAddSignalDispatcher_NoRaceWithAddOutput(t *testing.T) {
	a := newTestApp(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	dispatcher := signaldispatch.New(&mockParser{}, 16, logger)

	// Register the dispatcher — before the fix this immediately starts Dispatch.
	a.AddSignalDispatcher(dispatcher, 0)

	// AddOutput calls d.AddListener, which writes to listeners slice.
	// If Dispatch is already running (reading listeners), this is a data race.
	client := newMockSignalClient()
	a.AddOutput(client)
}
