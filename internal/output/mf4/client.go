// Package mf4 implements an MDF4 (.mf4) output client for the cantou pipeline.
//
// Two independent files are produced, either of which can be disabled by
// leaving its path empty in the configuration:
//
//   - CAN output: an ASAM-compliant CAN_DataFrame channel group with CAN
//     payloads stored as VLSD (variable-length signal data). The resulting
//     file is readable by internal/connection/playback and by external MDF4
//     tools.
//   - Signal output: a custom "Signal" channel group with fixed numeric
//     fields (timestamp, interface, CAN ID, value) and a VLSD label string
//     of the form "message\x00signal\x00unit".
package mf4

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/robbiebyrd/cantou/internal/client/common"
	canModels "github.com/robbiebyrd/cantou/internal/models"
	mf4fmt "github.com/robbiebyrd/cantou/internal/parser/mf4"
)

type Client struct {
	canWriter      *mf4fmt.Writer
	signalWriter   *mf4fmt.Writer
	finalize       bool
	canChannel     chan canModels.CanMessageTimestamped
	signalChannel  chan canModels.CanSignalTimestamped
	filters        *common.FilterSet
	l              *slog.Logger
	canMsgCount    atomic.Uint64
	signalMsgCount atomic.Uint64
}

// NewClient opens the CAN and/or signal MDF4 files configured on cfg and
// returns a Client ready to be wired into the output pipeline.
func NewClient(
	_ context.Context,
	cfg *canModels.Config,
	logger *slog.Logger,
) (canModels.OutputClient, error) {
	var (
		canWriter *mf4fmt.Writer
		sigWriter *mf4fmt.Writer
		err       error
	)

	if cfg.MF4Logger.CanOutputFile != "" {
		canWriter, err = mf4fmt.NewCANWriter(cfg.MF4Logger.CanOutputFile)
		if err != nil {
			return nil, fmt.Errorf("opening MF4 CAN output file: %w", err)
		}
	}

	if cfg.MF4Logger.SignalOutputFile != "" {
		sigWriter, err = mf4fmt.NewSignalWriter(cfg.MF4Logger.SignalOutputFile)
		if err != nil {
			if canWriter != nil {
				_ = canWriter.Close()
			}
			return nil, fmt.Errorf("opening MF4 signal output file: %w", err)
		}
	}

	return &Client{
		canWriter:     canWriter,
		signalWriter:  sigWriter,
		finalize:      cfg.MF4Logger.Finalize,
		canChannel:    make(chan canModels.CanMessageTimestamped, cfg.MessageBufferSize),
		signalChannel: make(chan canModels.CanSignalTimestamped, cfg.MessageBufferSize),
		filters:       common.NewFilterSet(),
		l:             logger,
	}, nil
}

func (c *Client) AddFilter(name string, filter canModels.FilterInterface) error {
	c.l.Debug("creating new filter group", "filterName", name)
	return c.filters.Add(name, filter)
}

func (c *Client) HandleCanMessage(canMsg canModels.CanMessageTimestamped) {
	if c.canWriter == nil {
		return
	}
	if shouldFilter, _ := c.filters.ShouldFilter(canMsg); shouldFilter {
		return
	}
	if err := c.canWriter.AppendCAN(canMsg.Timestamp, canMsg.ID, canMsg.Transmit, canMsg.Data); err != nil {
		c.l.Error("mf4 write CAN record error", "error", err)
	}
}

func (c *Client) HandleCanMessageChannel() error {
	done := make(chan struct{})
	defer close(done)
	common.StartThroughputReporter(done, c.l, c.GetName(), "can", &c.canMsgCount, func() int { return len(c.canChannel) }, 5*time.Second)
	for canMsg := range c.canChannel {
		c.canMsgCount.Add(1)
		c.HandleCanMessage(canMsg)
	}
	if c.canWriter != nil {
		if err := c.closeCAN(); err != nil {
			c.l.Error("mf4 CAN close error", "error", err)
		}
	}
	return nil
}

func (c *Client) GetChannel() chan canModels.CanMessageTimestamped {
	return c.canChannel
}

func (c *Client) GetSignalChannel() chan canModels.CanSignalTimestamped {
	return c.signalChannel
}

func (c *Client) HandleSignal(sig canModels.CanSignalTimestamped) {
	if c.signalWriter == nil {
		return
	}
	if err := c.signalWriter.AppendSignal(
		sig.Timestamp,
		sig.ID,
		uint32(sig.Interface),
		sig.Value,
		sig.Message,
		sig.Signal,
		sig.Unit,
	); err != nil {
		c.l.Error("mf4 write signal record error", "error", err)
	}
}

func (c *Client) HandleSignalChannel() error {
	done := make(chan struct{})
	defer close(done)
	common.StartThroughputReporter(done, c.l, c.GetName(), "signal", &c.signalMsgCount, func() int { return len(c.signalChannel) }, 5*time.Second)
	for sig := range c.signalChannel {
		c.signalMsgCount.Add(1)
		c.HandleSignal(sig)
	}
	if c.signalWriter != nil {
		if err := c.closeSignal(); err != nil {
			c.l.Error("mf4 signal close error", "error", err)
		}
	}
	return nil
}

func (c *Client) GetName() string {
	return "output-mf4"
}

// closeCAN flushes and closes the CAN writer, honouring the finalize flag.
func (c *Client) closeCAN() error {
	w := c.canWriter
	c.canWriter = nil
	if !c.finalize {
		return w.CloseUnfinalized()
	}
	return w.Close()
}

func (c *Client) closeSignal() error {
	w := c.signalWriter
	c.signalWriter = nil
	if !c.finalize {
		return w.CloseUnfinalized()
	}
	return w.Close()
}
