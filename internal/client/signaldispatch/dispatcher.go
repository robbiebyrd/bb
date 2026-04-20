package signaldispatch

import (
	"fmt"
	"log/slog"

	canModels "github.com/robbiebyrd/bb/internal/models"
	"github.com/robbiebyrd/bb/internal/parser/obd2"
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
			expanded := expandPIDsSupported(sig)
			for _, s := range expanded {
				for _, ch := range d.listeners {
					select {
					case ch <- s:
					default:
						d.l.Warn("signal dispatcher: listener full, dropping signal",
							"signal", s.Signal,
							"id", s.ID,
						)
					}
				}
			}
		}
	}
	return nil
}

// expandPIDsSupported checks whether sig is a PIDs Supported bitmask signal.
// If so, it returns one signal per supported PID (value=1, named
// "S01PID_Supported_XX"). Otherwise it returns the original signal unchanged.
func expandPIDsSupported(sig canModels.CanSignalTimestamped) []canModels.CanSignalTimestamped {
	base, ok := obd2.IsPIDsSupportedSignal(sig.Signal)
	if !ok {
		return []canModels.CanSignalTimestamped{sig}
	}
	pids := obd2.DecodePIDsSupported(uint32(sig.Value), base)
	expanded := make([]canModels.CanSignalTimestamped, 0, len(pids))
	for _, pid := range pids {
		expanded = append(expanded, canModels.CanSignalTimestamped{
			Timestamp: sig.Timestamp,
			Interface: sig.Interface,
			ID:        sig.ID,
			Message:   sig.Message,
			Signal:    fmt.Sprintf("S01PID_Supported_%02X", pid),
			Value:     1,
			Unit:      sig.Unit,
		})
	}
	return expanded
}
