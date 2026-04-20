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

	"github.com/robbiebyrd/bb/internal/client/common"
	canModels "github.com/robbiebyrd/bb/internal/models"
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
	filters        map[string]canModels.FilterInterface
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

	newFilters := make(map[string]canModels.FilterInterface, len(filters))
	for _, f := range filters {
		newFilters[f.Name] = f.Filter
	}

	logger.Debug("started prometheus client")

	return &PrometheusClient{
		ctx:           ctx,
		l:             logger,
		canChannel:    make(chan canModels.CanMessageTimestamped, cfg.MessageBufferSize),
		signalChannel: make(chan canModels.CanSignalTimestamped, cfg.MessageBufferSize),
		resolver:      resolver,
		filters:       newFilters,
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
	if _, ok := c.filters[name]; ok {
		return fmt.Errorf("filter group already exists: %v", name)
	}
	c.l.Debug("creating new filter group", "filterName", name)
	c.filters[name] = filter
	return nil
}

func (c *PrometheusClient) HandleCanMessageChannel() error {
	c.l.Debug("starting prometheus CAN channel handler")

	done := make(chan struct{})
	defer close(done)
	common.StartThroughputReporter(done, c.l, c.GetName(), "can", &c.canMsgCount, func() int { return len(c.canChannel) }, 5*time.Second)

	for canMsg := range c.canChannel {
		c.canMsgCount.Add(1)
		if shouldFilter, filterName := common.ShouldFilter(c.filters, canMsg); shouldFilter {
			c.l.Debug("message filtered, dropping", "message", canMsg, "filterName", *filterName)
			continue
		}
		interfaceName := ""
		if conn := c.resolver.ConnectionByID(canMsg.Interface); conn != nil {
			interfaceName = conn.GetInterfaceName()
		}
		c.frames.With(prometheus.Labels{
			"interface": interfaceName,
			"id":        fmt.Sprintf("0x%X", canMsg.ID),
		}).Inc()
	}
	return nil
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
		c.signals.With(prometheus.Labels{
			"interface": interfaceName,
			"message":   sig.Message,
			"signal":    sig.Signal,
			"unit":      sig.Unit,
		}).Set(sig.Value)
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
