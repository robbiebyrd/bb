package csv

import (
	"context"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/robbiebyrd/bb/internal/client/common"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

type CSVClient struct {
	w              *csv.Writer
	file           *os.File
	signalWriter   *csv.Writer
	signalFile     *os.File
	includeHeaders bool
	canChannel     chan canModels.CanMessageTimestamped
	signalChannel  chan canModels.CanSignalTimestamped
	filters        map[string]canModels.FilterInterface
	l              *slog.Logger
	resolver       canModels.InterfaceResolver
	canMsgCount    atomic.Uint64
	signalMsgCount atomic.Uint64
}

func NewClient(
	ctx context.Context,
	cfg *canModels.Config,
	logger *slog.Logger,
	resolver canModels.InterfaceResolver,
) (canModels.OutputClient, error) {
	var (
		canFile      *os.File
		canWriter    *csv.Writer
		signalFile   *os.File
		signalWriter *csv.Writer
	)

	if cfg.CSVLog.CanOutputFile != "" {
		f, err := os.OpenFile(cfg.CSVLog.CanOutputFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return nil, fmt.Errorf("opening CSV CAN output file: %w", err)
		}
		canFile = f
		canWriter = csv.NewWriter(f)
		if cfg.CSVLog.IncludeHeaders {
			header := []string{"timestamp", "id", "interface", "remote", "transmit", "length", "data"}
			if err = canWriter.Write(header); err != nil {
				logger.Error("csv write CAN header error", "error", err)
			}
		}
	}

	if cfg.CSVLog.SignalOutputFile != "" {
		f, err := os.OpenFile(cfg.CSVLog.SignalOutputFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return nil, fmt.Errorf("opening CSV signal output file: %w", err)
		}
		signalFile = f
		signalWriter = csv.NewWriter(f)
		if cfg.CSVLog.IncludeHeaders {
			header := []string{"timestamp", "interface", "message", "signal", "value", "unit"}
			if err = signalWriter.Write(header); err != nil {
				logger.Error("csv write signal header error", "error", err)
			}
		}
	}

	return &CSVClient{
		w:              canWriter,
		file:           canFile,
		signalWriter:   signalWriter,
		signalFile:     signalFile,
		includeHeaders: cfg.CSVLog.IncludeHeaders,
		canChannel:     make(chan canModels.CanMessageTimestamped, cfg.MessageBufferSize),
		signalChannel:  make(chan canModels.CanSignalTimestamped, cfg.MessageBufferSize),
		filters:        make(map[string]canModels.FilterInterface),
		l:              logger,
		resolver:       resolver,
	}, nil
}

func (c *CSVClient) AddFilter(name string, filter canModels.FilterInterface) error {
	if _, ok := c.filters[name]; ok {
		return fmt.Errorf("filter group already exists: %v", name)
	}
	c.l.Debug("creating new filter group", "filterName", name)
	c.filters[name] = filter
	return nil
}

func (c *CSVClient) HandleCanMessage(canMsg canModels.CanMessageTimestamped) {
	if c.w == nil {
		return
	}
	if shouldFilter, _ := common.ShouldFilter(c.filters, canMsg); shouldFilter {
		return
	}

	interfaceName := ""
	if conn := c.resolver.ConnectionByID(canMsg.Interface); conn != nil {
		interfaceName = conn.GetInterfaceName()
	}
	row := []string{
		strconv.FormatInt(canMsg.Timestamp, 10),
		strconv.FormatUint(uint64(canMsg.ID), 10),
		interfaceName,
		strconv.FormatBool(canMsg.Remote),
		strconv.FormatBool(canMsg.Transmit),
		strconv.Itoa(int(canMsg.Length)),
		hex.EncodeToString(canMsg.Data)}
	if err := c.w.Write(row); err != nil {
		c.l.Error("csv write error", "error", err)
	}
}

func (c *CSVClient) HandleCanMessageChannel() error {
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
		c.w.Flush()
		if err := c.w.Error(); err != nil {
			c.l.Error("csv flush error", "error", err)
		}
	}
	return nil
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
	row := []string{
		strconv.FormatInt(sig.Timestamp, 10),
		interfaceName,
		sig.Message,
		sig.Signal,
		strconv.FormatFloat(sig.Value, 'f', -1, 64),
		sig.Unit,
	}
	if err := c.signalWriter.Write(row); err != nil {
		c.l.Error("csv signal write error", "error", err)
	}
}

func (c *CSVClient) HandleSignalChannel() error {
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
		c.signalWriter.Flush()
		if err := c.signalWriter.Error(); err != nil {
			c.l.Error("csv signal flush error", "error", err)
		}
	}
	return nil
}

func (c *CSVClient) GetName() string {
	return "output-csv"
}
