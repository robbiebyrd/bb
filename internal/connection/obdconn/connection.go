// Package obdconn provides the shared connection machinery for ELM327- and
// STN-based OBD/CAN adapters. Both adapter families speak the same line-oriented
// ASCII protocol (implemented in internal/obd) and differ only in their command
// dialect and capabilities, so the boilerplate CanConnection implementation and
// the monitor/poll/hybrid receive orchestration live here once, parameterised by
// a Driver.
package obdconn

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	commonUtils "github.com/robbiebyrd/cantou/internal/client/common"
	canModels "github.com/robbiebyrd/cantou/internal/models"
	"github.com/robbiebyrd/cantou/internal/obd"
)

// Mode selects how the adapter is used.
const (
	// ModeMonitor passively streams every frame on the bus (optionally
	// hardware-filtered). This is the natural fit for CANtou's logger.
	ModeMonitor = "monitor"
	// ModePoll actively requests a list of OBD-II PIDs on an interval. Used when
	// the data of interest is not broadcast and must be solicited.
	ModePoll = "poll"
	// ModeHybrid interleaves monitoring with periodic PID polling on a single
	// adapter. Supported only by STN devices; ELM327 falls back (see New).
	ModeHybrid = "hybrid"
)

// defaultPollInterval is used when OBD_POLL_MS is unset.
const defaultPollInterval = time.Second

// defaultBaud is the serial line speed used when PORT_BAUD is unset. For
// Bluetooth rfcomm links the value is nominal.
const defaultBaud = 115200

// Driver abstracts the device-specific command dialect. ELM327 and STN provide
// implementations.
type Driver interface {
	// Init resets and configures the adapter for raw frame logging.
	Init() error
	// SetFilters installs receive/pass filters for the given identifiers (empty
	// clears all filters so every frame is monitored).
	SetFilters(ids []uint32) error
	// Monitor streams frames until stop is signalled.
	Monitor(onFrame func(obd.Frame), onError func(error), stop <-chan struct{}) error
	// Request sends an OBD-II service request and returns the raw response frames.
	Request(serviceHex string) ([]obd.Frame, error)
	// Extended reports whether the configured protocol uses 29-bit identifiers.
	Extended() bool
	// Close closes the device and its transport.
	Close() error
}

// DriverFactory builds a Driver over a freshly opened transport.
type DriverFactory func(t obd.Transport, proto obd.Protocol, debug bool, logf obd.Logf) Driver

// periodicDriver is implemented by drivers that can offload periodic PID polling
// to device firmware (STN via STPPMA), so monitoring and polling run
// concurrently with no gap. Drivers without this capability (ELM327) are kept
// out of hybrid mode by New.
type periodicDriver interface {
	StartPeriodic(pids []string, period time.Duration) error
	StopPeriodic() error
}

// Base is the shared CanConnection implementation for OBD adapters.
type Base struct {
	ctx     context.Context
	cfg     *canModels.Config
	id      int
	name    string
	network string
	uri     string
	channel chan canModels.CanMessageTimestamped

	l              *slog.Logger
	supportsHybrid bool
	factory        DriverFactory

	// Resolved options.
	proto        obd.Protocol
	mode         string
	filters      []uint32
	pids         []string
	pidErr       error
	pollInterval time.Duration
	baud         int

	driver    Driver
	transport obd.Transport
	opened    bool
	streaming bool
	quit      chan struct{}

	dbcFilePath *string
}

// New builds a Base from an interface option. kind selects the device family
// (which determines hybrid support and the ELM327 hybrid fallback) and factory
// constructs the matching Driver. The protocol, filters, PID list, poll interval
// and baud rate are resolved from the option here so both adapter packages share
// identical parsing.
func New(
	ctx context.Context,
	cfg *canModels.Config,
	channel chan canModels.CanMessageTimestamped,
	logger *slog.Logger,
	opt canModels.CanInterfaceOption,
	kind string,
	factory DriverFactory,
) *Base {
	if opt.Name == "" {
		panic(fmt.Errorf("connection name cannot be empty"))
	}
	if channel == nil {
		panic(fmt.Errorf("message channel cannot be nil"))
	}

	uri := opt.URI
	if uri == "" {
		uri = opt.Name
	}
	network := opt.Network
	if network == "" {
		network = kind
	}

	pids, pidErr := obd.NormalizePIDRequests(opt.OBDPIDs)

	pollInterval := defaultPollInterval
	if opt.OBDPollMS > 0 {
		pollInterval = time.Duration(opt.OBDPollMS) * time.Millisecond
	}

	baud := defaultBaud
	if opt.PortBaud > 0 {
		baud = opt.PortBaud
	}

	supportsHybrid := kind == canModels.NetworkSTN

	mode := strings.ToLower(strings.TrimSpace(opt.OBDMode))
	if mode == "" {
		mode = ModeMonitor
	}

	// ELM327 cannot interleave monitor and poll; fall back rather than fail.
	if mode == ModeHybrid && !supportsHybrid {
		fallback := ModeMonitor
		if len(pids) > 0 {
			fallback = ModePoll
		}
		logger.Warn("hybrid mode is only supported on STN devices; falling back",
			"interface", opt.Name, "fallback", fallback)
		mode = fallback
	}

	return &Base{
		ctx:            ctx,
		cfg:            cfg,
		name:           opt.Name,
		network:        network,
		uri:            uri,
		channel:        channel,
		l:              logger,
		supportsHybrid: supportsHybrid,
		factory:        factory,
		proto:          obd.ProtocolFor(opt.OBDProtocol),
		mode:           mode,
		filters:        opt.HWFilters,
		pids:           pids,
		pidErr:         pidErr,
		pollInterval:   pollInterval,
		baud:           baud,
	}
}

// --- CanConnection boilerplate ---

func (b *Base) GetID() int               { return b.id }
func (b *Base) SetID(id int)             { b.id = id }
func (b *Base) GetURI() string           { return b.uri }
func (b *Base) SetURI(uri string)        { b.uri = uri }
func (b *Base) GetNetwork() string       { return b.network }
func (b *Base) SetNetwork(net string)    { b.network = net }
func (b *Base) GetName() string          { return b.name }
func (b *Base) SetName(name string)      { b.name = name }
func (b *Base) IsOpen() bool             { return b.opened }
func (b *Base) Mode() string             { return b.mode }
func (b *Base) GetDBCFilePath() *string  { return b.dbcFilePath }
func (b *Base) SetDBCFilePath(p *string) { b.dbcFilePath = p }

// GetConnection returns nil — OBD adapters use a serial transport, not a net.Conn.
func (b *Base) GetConnection() net.Conn { return nil }

// SetConnection is a no-op — OBD adapters do not use a net.Conn.
func (b *Base) SetConnection(_ net.Conn) {}

func (b *Base) GetInterfaceName() string {
	return b.name + b.cfg.CanInterfaceSeparator + b.network + b.cfg.CanInterfaceSeparator + b.uri
}

// Open opens the transport, builds the driver, and configures the adapter.
func (b *Base) Open() error {
	if b.pidErr != nil {
		return fmt.Errorf("invalid OBD PID list for %s: %w", b.name, b.pidErr)
	}
	if (b.mode == ModePoll || b.mode == ModeHybrid) && len(b.pids) == 0 {
		return fmt.Errorf("%s mode requires at least one PID (set INTERFACE_OBD_PIDS)", b.mode)
	}

	transport, err := obd.OpenSerial(obd.SerialConfig{
		Port:        b.uri,
		Baud:        b.baud,
		ReadTimeout: 50 * time.Millisecond,
	})
	if err != nil {
		return err
	}
	b.transport = transport

	debug := b.cfg.LogLevel == "debug"
	logf := func(format string, args ...any) { b.l.Debug(fmt.Sprintf(format, args...)) }
	b.driver = b.factory(transport, b.proto, debug, logf)

	if err := b.driver.Init(); err != nil {
		_ = transport.Close()
		return fmt.Errorf("initialising %s adapter %s: %w", b.network, b.uri, err)
	}
	if err := b.driver.SetFilters(b.filters); err != nil {
		_ = transport.Close()
		return fmt.Errorf("setting filters on %s: %w", b.uri, err)
	}

	b.opened = true
	b.l.Info("opened OBD adapter",
		"interface", b.name, "network", b.network, "uri", b.uri,
		"mode", b.mode, "protocol", b.proto.Label, "filters", len(b.filters), "pids", len(b.pids))
	return nil
}

// Close stops streaming and closes the device.
func (b *Base) Close() error {
	_ = b.Discontinue()
	if b.driver != nil {
		if err := b.driver.Close(); err != nil {
			return fmt.Errorf("closing %s adapter %s: %w", b.network, b.uri, err)
		}
	}
	b.opened = false
	return nil
}

// Discontinue signals the receive loop to stop.
func (b *Base) Discontinue() error {
	if b.streaming && b.quit != nil {
		close(b.quit)
	}
	b.streaming = false
	return nil
}

// Receive starts the mode-appropriate receive loop.
func (b *Base) Receive(wg *sync.WaitGroup) {
	b.quit = make(chan struct{})
	b.streaming = true

	wg.Go(func() {
		switch b.mode {
		case ModePoll:
			b.runPoll()
		case ModeHybrid:
			b.runHybrid()
		default:
			b.runMonitor()
		}
	})
}

// emit forwards a decoded frame onto the message channel, respecting shutdown.
func (b *Base) emit(frame obd.Frame) {
	msg := canModels.CanMessageTimestamped{
		Timestamp: time.Now().UnixNano(),
		Interface: b.id,
		Transmit:  false,
		ID:        frame.ID,
		Remote:    false,
		Length:    uint8(len(frame.Data)),
		Data:      commonUtils.PadOrTrim(frame.Data, 8),
	}
	select {
	case b.channel <- msg:
	case <-b.quit:
	case <-b.ctx.Done():
	}
}

func (b *Base) onError(err error) {
	b.l.Debug("frame decode error", "interface", b.name, "error", err)
}

// runMonitor passively streams frames until shutdown.
func (b *Base) runMonitor() {
	if err := b.driver.Monitor(b.emit, b.onError, b.quit); err != nil {
		b.l.Error("monitor loop ended with error", "interface", b.name, "error", err)
	}
}

// runPoll repeatedly requests the configured PID list on an interval.
func (b *Base) runPoll() {
	ticker := time.NewTicker(b.pollInterval)
	defer ticker.Stop()
	for {
		b.pollOnce()
		select {
		case <-ticker.C:
		case <-b.quit:
			return
		case <-b.ctx.Done():
			return
		}
	}
}

// pollOnce sends each PID request once and emits the response frames.
func (b *Base) pollOnce() {
	for _, pid := range b.pids {
		select {
		case <-b.quit:
			return
		case <-b.ctx.Done():
			return
		default:
		}
		frames, err := b.driver.Request(pid)
		if err != nil {
			b.l.Debug("PID request failed", "interface", b.name, "pid", pid, "error", err)
			continue
		}
		for _, f := range frames {
			b.emit(f)
		}
	}
}

// runHybrid monitors the bus while the device fires the PID requests itself, in
// firmware, on a timer (STN STPPMA). Because the polling happens on the adapter,
// monitoring is never interrupted and no frames are lost — the poll responses
// simply appear in the monitor stream. Only STN devices reach this path (ELM327
// is downgraded in New), but the capability is checked defensively.
func (b *Base) runHybrid() {
	pd, ok := b.driver.(periodicDriver)
	if !ok {
		b.l.Error("hybrid mode requires a periodic-capable driver; monitoring only", "interface", b.name)
		b.runMonitor()
		return
	}
	if err := pd.StartPeriodic(b.pids, b.pollInterval); err != nil {
		b.l.Error("failed to start firmware periodic polling; monitoring only", "interface", b.name, "error", err)
		b.runMonitor()
		return
	}

	// Stream bus traffic and the firmware-driven poll responses together.
	b.runMonitor()

	// Monitor has returned to the prompt; clear the periodic messages.
	if err := pd.StopPeriodic(); err != nil {
		b.l.Debug("failed to clear periodic messages on shutdown", "interface", b.name, "error", err)
	}
}
