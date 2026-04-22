package csv

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/robbiebyrd/cantou/internal/client/common"
	canModels "github.com/robbiebyrd/cantou/internal/models"
	csvfmt "github.com/robbiebyrd/cantou/internal/parser/csv"
)

// csvFlushInterval caps how long a buffered row can sit in memory before the
// underlying csv.Writer is flushed to disk. Short enough to bound data loss
// on abrupt termination, long enough that high-rate bursts benefit from
// buffered writes.
const csvFlushInterval = 1 * time.Second

type CSVClient struct {
	canWriter      *csvfmt.CANWriter
	signalWriter   *csvfmt.SignalWriter
	canChannel     chan canModels.CanMessageTimestamped
	signalChannel  chan canModels.CanSignalTimestamped
	filters        *common.FilterSet
	l              *slog.Logger
	resolver       canModels.InterfaceResolver
	canMsgCount    atomic.Uint64
	signalMsgCount atomic.Uint64
}

func NewClient(
	_ context.Context,
	cfg *canModels.Config,
	logger *slog.Logger,
	resolver canModels.InterfaceResolver,
) (canModels.OutputClient, error) {
	var (
		canWriter    *csvfmt.CANWriter
		signalWriter *csvfmt.SignalWriter
	)

	if cfg.CSVLog.CanOutputFile != "" {
		w, err := csvfmt.NewCANWriter(cfg.CSVLog.CanOutputFile, cfg.CSVLog.IncludeHeaders)
		if err != nil {
			return nil, err
		}
		canWriter = w
	}

	if cfg.CSVLog.SignalOutputFile != "" {
		w, err := csvfmt.NewSignalWriter(cfg.CSVLog.SignalOutputFile, cfg.CSVLog.IncludeHeaders)
		if err != nil {
			if canWriter != nil {
				canWriter.Close()
			}
			return nil, err
		}
		signalWriter = w
	}

	return &CSVClient{
		canWriter:     canWriter,
		signalWriter:  signalWriter,
		canChannel:    make(chan canModels.CanMessageTimestamped, cfg.MessageBufferSize),
		signalChannel: make(chan canModels.CanSignalTimestamped, cfg.MessageBufferSize),
		filters:       common.NewFilterSet(),
		l:             logger,
		resolver:      resolver,
	}, nil
}

func (c *CSVClient) AddFilter(name string, filter canModels.FilterInterface) error {
	c.l.Debug("creating new filter group", "filterName", name)
	return c.filters.Add(name, filter)
}

func (c *CSVClient) HandleCanMessage(canMsg canModels.CanMessageTimestamped) {
	if c.canWriter == nil {
		return
	}
	if shouldFilter, _ := c.filters.ShouldFilter(canMsg); shouldFilter {
		return
	}

	interfaceName := ""
	if conn := c.resolver.ConnectionByID(canMsg.Interface); conn != nil {
		interfaceName = conn.GetInterfaceName()
	}
	if err := c.canWriter.Append(
		canMsg.Timestamp,
		uint64(canMsg.ID),
		interfaceName,
		canMsg.Remote,
		canMsg.Transmit,
		int(canMsg.Length),
		canMsg.Data,
	); err != nil {
		c.l.Error("csv write error", "error", err)
	}
}

func (c *CSVClient) HandleCanMessageChannel() error {
	defer func() {
		if c.canWriter != nil {
			if err := c.canWriter.Close(); err != nil {
				c.l.Error("csv close error", "error", err)
			}
		}
	}()
	done := make(chan struct{})
	defer close(done)
	common.StartThroughputReporter(done, c.l, c.GetName(), "can", &c.canMsgCount, func() int { return len(c.canChannel) }, 5*time.Second)

	ticker := time.NewTicker(csvFlushInterval)
	defer ticker.Stop()

	flush := func() {
		if c.canWriter == nil {
			return
		}
		if err := c.canWriter.Flush(); err != nil {
			c.l.Error("csv flush error", "error", err)
		}
	}

	for {
		select {
		case canMsg, ok := <-c.canChannel:
			if !ok {
				flush()
				return nil
			}
			c.canMsgCount.Add(1)
			c.HandleCanMessage(canMsg)
		case <-ticker.C:
			flush()
		}
	}
}

func (c *CSVClient) GetChannel() chan canModels.CanMessageTimestamped {
	return c.canChannel
}

func (c *CSVClient) GetSignalChannel() chan canModels.CanSignalTimestamped {
	return c.signalChannel
}

func (c *CSVClient) HandleSignal(sig canModels.CanSignalTimestamped) {
	if c.signalWriter == nil {
		return
	}

	interfaceName := ""
	if conn := c.resolver.ConnectionByID(sig.Interface); conn != nil {
		interfaceName = conn.GetInterfaceName()
	}
	if err := c.signalWriter.Append(
		sig.Timestamp,
		interfaceName,
		sig.Message,
		sig.Signal,
		sig.Value,
		sig.Unit,
	); err != nil {
		c.l.Error("csv signal write error", "error", err)
	}
}

func (c *CSVClient) HandleSignalChannel() error {
	defer func() {
		if c.signalWriter != nil {
			if err := c.signalWriter.Close(); err != nil {
				c.l.Error("csv signal close error", "error", err)
			}
		}
	}()
	done := make(chan struct{})
	defer close(done)
	common.StartThroughputReporter(done, c.l, c.GetName(), "signal", &c.signalMsgCount, func() int { return len(c.signalChannel) }, 5*time.Second)

	ticker := time.NewTicker(csvFlushInterval)
	defer ticker.Stop()

	flush := func() {
		if c.signalWriter == nil {
			return
		}
		if err := c.signalWriter.Flush(); err != nil {
			c.l.Error("csv signal flush error", "error", err)
		}
	}

	for {
		select {
		case sig, ok := <-c.signalChannel:
			if !ok {
				flush()
				return nil
			}
			c.signalMsgCount.Add(1)
			c.HandleSignal(sig)
		case <-ticker.C:
			flush()
		}
	}
}

func (c *CSVClient) GetName() string {
	return "output-csv"
}
