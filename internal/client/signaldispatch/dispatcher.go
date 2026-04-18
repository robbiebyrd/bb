package signaldispatch

import (
	"log/slog"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

// SignalDispatcher receives CanMessageTimestamped values, decodes them into
// CanSignalTimestamped via a DBC parser, and fans out each signal to all
// registered listeners using non-blocking sends (drops with Warn on full).
type SignalDispatcher struct {
	parser    canModels.ParserInterface
	inChannel chan canModels.CanMessageTimestamped
	listeners []chan canModels.CanSignalTimestamped
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

// AddListener registers a signal output channel. Must be called before Dispatch.
func (d *SignalDispatcher) AddListener(ch chan canModels.CanSignalTimestamped) {
	d.listeners = append(d.listeners, ch)
}

// GetChannel returns the input channel to register with BroadcastClient.
func (d *SignalDispatcher) GetChannel() chan canModels.CanMessageTimestamped {
	return d.inChannel
}

// Dispatch reads CAN messages, decodes signals, and fans them out to listeners.
// Runs until inChannel is closed.
func (d *SignalDispatcher) Dispatch() error {
	for msg := range d.inChannel {
		signals := d.parser.ParseSignals(msg)
		for _, sig := range signals {
			for _, ch := range d.listeners {
				select {
				case ch <- sig:
				default:
					d.l.Warn("signal dispatcher: listener full, dropping signal",
						"signal", sig.Signal,
						"id", sig.ID,
					)
				}
			}
		}
	}
	return nil
}
