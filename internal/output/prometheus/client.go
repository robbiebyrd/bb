package prometheus

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/robbiebyrd/cantou/internal/client/common"
	canModels "github.com/robbiebyrd/cantou/internal/models"
)

// PrometheusClient is an output client that exposes CAN bus frames and decoded
// signals as Prometheus metrics over an HTTP /metrics endpoint.
// The service acts as a Prometheus exporter — Prometheus scrapes it.
type PrometheusClient struct {
	ctx            context.Context
	l              *slog.Logger
	canChannel     chan canModels.CanMessageTimestamped
	signalChannel  chan canModels.CanSignalTimestamped
	resolver       canModels.InterfaceResolver
	filters        *common.FilterSet
	canMsgCount    atomic.Uint64
	signalMsgCount atomic.Uint64
	registry       *prometheus.Registry
	frames         *prometheus.CounterVec
	signals        *prometheus.GaugeVec
	listenAddr     string
	path           string
}

func NewClient(
	ctx context.Context,
	cfg *canModels.Config,
	logger *slog.Logger,
	resolver canModels.InterfaceResolver,
	filters ...canModels.FilterInput,
) (canModels.OutputClient, error) {
	logger.Debug("starting prometheus client")

	reg := prometheus.NewRegistry()

	// Register standard Go runtime metrics so /metrics is not empty by default.
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	frames := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "can_frames_total",
		Help: "Total CAN frames received, by interface and message ID.",
	}, []string{"interface", "id"})
	reg.MustRegister(frames)

	signals := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "can_signal_value",
		Help: "Most recent decoded DBC signal value.",
	}, []string{"interface", "message", "signal", "unit"})
	reg.MustRegister(signals)

	logger.Debug("started prometheus client")

	return &PrometheusClient{
		ctx:           ctx,
		l:             logger,
		canChannel:    make(chan canModels.CanMessageTimestamped, cfg.MessageBufferSize),
		signalChannel: make(chan canModels.CanSignalTimestamped, cfg.MessageBufferSize),
		resolver:      resolver,
		filters:       common.NewFilterSetFromInputs(filters),
		registry:      reg,
		frames:        frames,
		signals:       signals,
		listenAddr:    cfg.Prometheus.ListenAddr,
		path:          cfg.Prometheus.Path,
	}, nil
}

func (c *PrometheusClient) GetName() string {
	return "output-prometheus"
}

func (c *PrometheusClient) GetChannel() chan canModels.CanMessageTimestamped {
	return c.canChannel
}

func (c *PrometheusClient) GetSignalChannel() chan canModels.CanSignalTimestamped {
	return c.signalChannel
}

// HandleCanMessage is a no-op; all processing happens in HandleCanMessageChannel.
func (c *PrometheusClient) HandleCanMessage(_ canModels.CanMessageTimestamped) {}

// HandleSignal is a no-op; all processing happens in HandleSignalChannel.
func (c *PrometheusClient) HandleSignal(_ canModels.CanSignalTimestamped) {}

func (c *PrometheusClient) AddFilter(name string, filter canModels.FilterInterface) error {
	c.l.Debug("creating new filter group", "filterName", name)
	return c.filters.Add(name, filter)
}

func (c *PrometheusClient) HandleCanMessageChannel() error {
	c.l.Debug("starting prometheus CAN channel handler")

	done := make(chan struct{})
	defer close(done)
	common.StartThroughputReporter(done, c.l, c.GetName(), "can", &c.canMsgCount, func() int { return len(c.canChannel) }, 5*time.Second)

	for canMsg := range c.canChannel {
		c.canMsgCount.Add(1)
		if shouldFilter, filterName := c.filters.ShouldFilter(canMsg); shouldFilter {
			c.l.Debug("message filtered, dropping", "message", canMsg, "filterName", filterName)
			continue
		}
		interfaceName := ""
		if conn := c.resolver.ConnectionByID(canMsg.Interface); conn != nil {
			interfaceName = conn.GetInterfaceName()
		}
		// WithLabelValues avoids the per-call prometheus.Labels map allocation
		// that With() requires. The CounterVec internally caches the child
		// counter, so repeat lookups for the same (interface, id) are cheap.
		c.frames.WithLabelValues(interfaceName, formatCanIDHex(canMsg.ID)).Inc()
	}
	return nil
}

// formatCanIDHex renders a CAN ID as "0x<uppercase hex>" without fmt.
func formatCanIDHex(id uint32) string {
	const digits = "0123456789ABCDEF"
	if id == 0 {
		return "0x0"
	}
	var buf [10]byte // "0x" + up to 8 hex digits
	buf[0] = '0'
	buf[1] = 'x'
	i := len(buf)
	for id > 0 {
		i--
		buf[i] = digits[id&0xF]
		id >>= 4
	}
	// Shift the hex digits to sit immediately after "0x".
	copy(buf[2:], buf[i:])
	return string(buf[:2+len(buf)-i])
}

func (c *PrometheusClient) HandleSignalChannel() error {
	c.l.Debug("starting prometheus signal channel handler")

	done := make(chan struct{})
	defer close(done)
	common.StartThroughputReporter(done, c.l, c.GetName(), "signal", &c.signalMsgCount, func() int { return len(c.signalChannel) }, 5*time.Second)

	for sig := range c.signalChannel {
		c.signalMsgCount.Add(1)
		interfaceName := ""
		if conn := c.resolver.ConnectionByID(sig.Interface); conn != nil {
			interfaceName = conn.GetInterfaceName()
		}
		c.signals.WithLabelValues(interfaceName, sig.Message, sig.Signal, sig.Unit).Set(sig.Value)
	}
	return nil
}

// Run starts the HTTP server that serves the /metrics endpoint.
// It satisfies the RunnerClient interface. It blocks until ctx is cancelled.
func (c *PrometheusClient) Run() error {
	mux := http.NewServeMux()
	mux.Handle(c.path, promhttp.HandlerFor(c.registry, promhttp.HandlerOpts{}))

	srv := &http.Server{
		Addr:    c.listenAddr,
		Handler: mux,
	}

	srvErr := make(chan error, 1)
	go func() {
		c.l.Info("prometheus metrics server listening", "addr", c.listenAddr, "path", c.path)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			srvErr <- err
		}
	}()

	select {
	case <-c.ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("prometheus metrics server shutdown: %w", err)
		}
		return nil
	case err := <-srvErr:
		return fmt.Errorf("prometheus metrics server: %w", err)
	}
}
