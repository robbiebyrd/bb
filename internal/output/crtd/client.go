package crtd

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/robbiebyrd/cantou/internal/client/common"
	canModels "github.com/robbiebyrd/cantou/internal/models"
	crtdfmt "github.com/robbiebyrd/cantou/internal/parser/crtd"
)

// crtdFlushInterval caps how long buffered output can sit in memory before
// being flushed. Same rationale as csvFlushInterval.
const crtdFlushInterval = 1 * time.Second

type CRTDLoggerClient struct {
	canWriter      *crtdfmt.CANWriter
	signalWriter   *crtdfmt.SignalWriter
	filters        *common.FilterSet
	canChannel     chan canModels.CanMessageTimestamped
	signalChannel  chan canModels.CanSignalTimestamped
	l              *slog.Logger
	canMsgCount    atomic.Uint64
	signalMsgCount atomic.Uint64
}

func NewClient(
	_ context.Context,
	cfg *canModels.Config,
	logger *slog.Logger,
) (canModels.OutputClient, error) {
	var (
		canWriter    *crtdfmt.CANWriter
		signalWriter *crtdfmt.SignalWriter
	)

	if cfg.CRTDLogger.CanOutputFile != "" {
		w, err := crtdfmt.NewCANWriter(cfg.CRTDLogger.CanOutputFile, cfg)
		if err != nil {
			return nil, err
		}
		canWriter = w
	}

	if cfg.CRTDLogger.SignalOutputFile != "" {
		w, err := crtdfmt.NewSignalWriter(cfg.CRTDLogger.SignalOutputFile)
		if err != nil {
			if canWriter != nil {
				canWriter.Close()
			}
			return nil, err
		}
		signalWriter = w
	}

	return &CRTDLoggerClient{
		canWriter:     canWriter,
		signalWriter:  signalWriter,
		canChannel:    make(chan canModels.CanMessageTimestamped, cfg.MessageBufferSize),
		signalChannel: make(chan canModels.CanSignalTimestamped, cfg.MessageBufferSize),
		filters:       common.NewFilterSet(),
		l:             logger,
	}, nil
}

func (c *CRTDLoggerClient) AddFilter(name string, filter canModels.FilterInterface) error {
	c.l.Debug("creating new filter group", "filterName", name)
	return c.filters.Add(name, filter)
}

func (c *CRTDLoggerClient) HandleCanMessage(canMsg canModels.CanMessageTimestamped) {
	if c.canWriter == nil {
		return
	}
	if shouldFilter, _ := c.filters.ShouldFilter(canMsg); shouldFilter {
		return
	}
	if err := c.canWriter.Append(canMsg.Timestamp, canMsg.Interface, canMsg.ID, canMsg.Transmit, canMsg.Data); err != nil {
		c.l.Error("Could not write record to CRTD file", "error", err)
	}
}

func (c *CRTDLoggerClient) HandleCanMessageChannel() error {
	defer func() {
		if c.canWriter != nil {
			if err := c.canWriter.Close(); err != nil {
				c.l.Error("Could not close CRTD file", "error", err)
			}
		}
	}()
	done := make(chan struct{})
	defer close(done)
	common.StartThroughputReporter(done, c.l, c.GetName(), "can", &c.canMsgCount, func() int { return len(c.canChannel) }, 5*time.Second)

	ticker := time.NewTicker(crtdFlushInterval)
	defer ticker.Stop()

	flush := func() {
		if c.canWriter == nil {
			return
		}
		if err := c.canWriter.Flush(); err != nil {
			c.l.Error("Could not flush CRTD file", "error", err)
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

func (c *CRTDLoggerClient) GetChannel() chan canModels.CanMessageTimestamped {
	return c.canChannel
}

func (c *CRTDLoggerClient) GetSignalChannel() chan canModels.CanSignalTimestamped {
	return c.signalChannel
}

func (c *CRTDLoggerClient) HandleSignal(sig canModels.CanSignalTimestamped) {
	if c.signalWriter == nil {
		return
	}
	if err := c.signalWriter.Append(sig.Timestamp, sig.Interface, sig.Message, sig.Signal, sig.Value, sig.Unit); err != nil {
		c.l.Error("Could not write signal to CRTD signal file", "error", err)
	}
}

func (c *CRTDLoggerClient) HandleSignalChannel() error {
	defer func() {
		if c.signalWriter != nil {
			if err := c.signalWriter.Close(); err != nil {
				c.l.Error("Could not close CRTD signal file", "error", err)
			}
		}
	}()
	done := make(chan struct{})
	defer close(done)
	common.StartThroughputReporter(done, c.l, c.GetName(), "signal", &c.signalMsgCount, func() int { return len(c.signalChannel) }, 5*time.Second)

	ticker := time.NewTicker(crtdFlushInterval)
	defer ticker.Stop()

	flush := func() {
		if c.signalWriter == nil {
			return
		}
		if err := c.signalWriter.Flush(); err != nil {
			c.l.Error("Could not flush CRTD signal file", "error", err)
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

func (c *CRTDLoggerClient) GetName() string {
	return "output-crtd"
}
