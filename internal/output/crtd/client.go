package crtd

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/robbiebyrd/bb/internal/client/common"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

type CRTDLoggerClient struct {
	dbcParser      canModels.ParserInterface
	filters        map[string]canModels.FilterInterface
	file           *os.File
	signalFile     *os.File
	canChannel     chan canModels.CanMessageTimestamped
	signalChannel  chan canModels.CanSignalTimestamped
	w              *bufio.Writer
	signalWriter   *bufio.Writer
	l              *slog.Logger
	canMsgCount    atomic.Uint64
	signalMsgCount atomic.Uint64
}

// writeHeader writes the CRTD file header to w, logging each write error
// individually so no error is silently overwritten by the next write.
func writeHeader(w *bufio.Writer, cfg *canModels.Config, logger *slog.Logger) {
	if _, err := fmt.Fprintln(w, "0.000000 CXX CRTD file created by bb"); err != nil {
		logger.Error("Could not write header to CRTD file", "error", err)
	}
	for index, canInterface := range cfg.CanInterfaces {
		_, err := fmt.Fprintf(
			w,
			"0.000000 CXX Info Type:'interface'; ID:'%d'; Name:'%s'; URI:'%s'; Network:'%s'; DBC:'%s';\n",
			index,
			canInterface.Name,
			canInterface.URI,
			canInterface.Network,
			strings.Join(canInterface.DBCFiles, ","),
		)
		if err != nil {
			logger.Error("Could not write interface header to CRTD file", "error", err)
		}
	}
	if err := w.Flush(); err != nil {
		logger.Error("Could not flush CRTD file when writing header", "error", err)
	}
}

func NewClient(
	ctx context.Context,
	cfg *canModels.Config,
	logger *slog.Logger,
) (canModels.OutputClient, error) {
	var (
		canFile      *os.File
		canWriter    *bufio.Writer
		signalFile   *os.File
		signalWriter *bufio.Writer
	)

	if cfg.CRTDLogger.CanOutputFile != "" {
		f, err := os.OpenFile(cfg.CRTDLogger.CanOutputFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return nil, fmt.Errorf("opening CRTD output file: %w", err)
		}
		canFile = f
		canWriter = bufio.NewWriter(f)
		writeHeader(canWriter, cfg, logger)
	}

	if cfg.CRTDLogger.SignalOutputFile != "" {
		f, err := os.OpenFile(cfg.CRTDLogger.SignalOutputFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return nil, fmt.Errorf("opening CRTD signal output file: %w", err)
		}
		signalFile = f
		signalWriter = bufio.NewWriter(f)
	}

	return &CRTDLoggerClient{
		w:             canWriter,
		file:          canFile,
		signalWriter:  signalWriter,
		signalFile:    signalFile,
		canChannel:    make(chan canModels.CanMessageTimestamped, cfg.MessageBufferSize),
		signalChannel: make(chan canModels.CanSignalTimestamped, cfg.MessageBufferSize),
		filters:       make(map[string]canModels.FilterInterface),
		l:             logger,
	}, nil
}

func (c *CRTDLoggerClient) AddFilter(name string, filter canModels.FilterInterface) error {
	if _, ok := c.filters[name]; ok {
		return fmt.Errorf("filter group already exists: %v", name)
	}
	c.l.Debug("creating new filter group", "filterName", name)
	c.filters[name] = filter
	return nil
}

func (c *CRTDLoggerClient) HandleCanMessage(canMsg canModels.CanMessageTimestamped) {
	if c.w == nil {
		return
	}
	if shouldFilter, _ := common.ShouldFilter(c.filters, canMsg); shouldFilter {
		return
	}

	seconds := canMsg.Timestamp / 1e9
	microseconds := (canMsg.Timestamp % 1e9) / 1e3

	var recordType string
	if canMsg.Transmit {
		recordType = "T"
	} else {
		recordType = "R"
	}
	if canMsg.ID > 0x7FF {
		recordType += "29"
	} else {
		recordType += "11"
	}

	canID := fmt.Sprintf("%X", canMsg.ID)

	dataBytes := make([]string, len(canMsg.Data))
	for i, b := range canMsg.Data {
		dataBytes[i] = fmt.Sprintf("%02X", b)
	}

	line := fmt.Sprintf("%d.%06d %d%s %s %s",
		seconds, microseconds,
		canMsg.Interface, recordType, canID,
		strings.Join(dataBytes, " "))

	if _, err := fmt.Fprintln(c.w, line); err != nil {
		c.l.Error("Could not write record to CRTD file", "error", err)
	}
}

func (c *CRTDLoggerClient) HandleCanMessageChannel() error {
	if c.file != nil {
		defer c.file.Close()
	}
	done := make(chan struct{})
	defer close(done)
	common.StartThroughputReporter(done, c.l, c.GetName(), "can", &c.canMsgCount, func() int { return len(c.canChannel) }, 5*time.Second)
	for canMsg := range c.canChannel {
		c.canMsgCount.Add(1)
		c.HandleCanMessage(canMsg)
	}
	if c.w != nil {
		if err := c.w.Flush(); err != nil {
			c.l.Error("Could not flush CRTD file on channel close", "error", err)
		}
	}
	return nil
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

	seconds := sig.Timestamp / 1e9
	microseconds := (sig.Timestamp % 1e9) / 1e3

	line := fmt.Sprintf("%d.%06d %dSIG %s/%s %s %s",
		seconds, microseconds,
		sig.Interface, sig.Message, sig.Signal,
		strconv.FormatFloat(sig.Value, 'f', -1, 64),
		sig.Unit,
	)

	if _, err := fmt.Fprintln(c.signalWriter, line); err != nil {
		c.l.Error("Could not write signal to CRTD signal file", "error", err)
	}
}

func (c *CRTDLoggerClient) HandleSignalChannel() error {
	if c.signalFile != nil {
		defer c.signalFile.Close()
	}
	done := make(chan struct{})
	defer close(done)
	common.StartThroughputReporter(done, c.l, c.GetName(), "signal", &c.signalMsgCount, func() int { return len(c.signalChannel) }, 5*time.Second)
	for sig := range c.signalChannel {
		c.signalMsgCount.Add(1)
		c.HandleSignal(sig)
	}
	if c.signalWriter != nil {
		if err := c.signalWriter.Flush(); err != nil {
			c.l.Error("Could not flush CRTD signal file on channel close", "error", err)
		}
	}
	return nil
}

func (c *CRTDLoggerClient) GetName() string {
	return "output-crtd"
}
