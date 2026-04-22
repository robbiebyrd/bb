package signaldispatch

import (
	"log/slog"
	"sync"
	"sync/atomic"

	signalfilter "github.com/robbiebyrd/cantou/internal/client/signal-filter"
	canModels "github.com/robbiebyrd/cantou/internal/models"
	"github.com/robbiebyrd/cantou/internal/parser/obd2"
)

type registeredListener struct {
	listener canModels.SignalDispatcherListener
	dropped  atomic.Uint64
}

// SignalDispatcher receives CanMessageTimestamped values, decodes them into
// CanSignalTimestamped via a DBC parser, and fans out each signal to all
// registered listeners using non-blocking sends (drops with Warn on full).
type SignalDispatcher struct {
	parser    canModels.ParserInterface
	inChannel chan canModels.CanMessageTimestamped
	mu        sync.RWMutex
	listeners []*registeredListener
	filter    signalfilter.Group
	l         *slog.Logger
}

// New creates a SignalDispatcher. bufSize is the capacity of the input channel.
// Call AddListener before Dispatch().
func New(parser canModels.ParserInterface, bufSize int, logger *slog.Logger) *SignalDispatcher {
	return &SignalDispatcher{
		parser:    parser,
		inChannel: make(chan canModels.CanMessageTimestamped, bufSize),
		l:         logger,
	}
}

// NewWithFilter creates a SignalDispatcher that applies filter to each decoded
// signal before fanning out. Signals rejected by the filter are silently dropped.
func NewWithFilter(parser canModels.ParserInterface, bufSize int, logger *slog.Logger, filter signalfilter.Group) *SignalDispatcher {
	d := New(parser, bufSize, logger)
	d.filter = filter
	return d
}

// AddListener registers a named signal output channel.
func (d *SignalDispatcher) AddListener(listener canModels.SignalDispatcherListener) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.listeners = append(d.listeners, &registeredListener{listener: listener})
}

// DroppedCount returns the total number of signals dropped for the named
// listener because its channel was full. Returns 0 if the listener is not registered.
func (d *SignalDispatcher) DroppedCount(name string) uint64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, l := range d.listeners {
		if l.listener.Name == name {
			return l.dropped.Load()
		}
	}
	return 0
}

// GetChannel returns the input channel to register with MessageDispatcher.
func (d *SignalDispatcher) GetChannel() chan canModels.CanMessageTimestamped {
	return d.inChannel
}

// Dispatch reads CAN messages, decodes signals, and fans them out to listeners.
// Runs until inChannel is closed.
func (d *SignalDispatcher) Dispatch() error {
	for msg := range d.inChannel {
		signals := d.parser.ParseSignals(msg)
		for _, sig := range signals {
			expanded := obd2.ExpandPIDsSupported(sig)
			d.mu.RLock()

			listeners := make([]*registeredListener, len(d.listeners))
			copy(listeners, d.listeners)
			d.mu.RUnlock()
			for _, s := range expanded {
				if !d.filter.Allow(s) {
					continue
				}
				for _, l := range listeners {
					select {
					case l.listener.Channel <- s:
					default:
						dropped := l.dropped.Add(1)
						if dropped == 1 || dropped%1000 == 0 {
							d.l.Warn("signal dispatcher: listener full, dropping signal",
								"listener", l.listener.Name,
								"signal", s.Signal,
								"id", s.ID,
								"dropped_total", dropped,
							)
						}
					}
				}
			}
		}
	}
	return nil
}
