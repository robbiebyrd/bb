package crtd

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/robbiebyrd/bb/internal/client/common"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

type CRTDLoggerClient struct {
	dbcParser canModels.ParserInterface
	filters   map[string]canModels.FilterInterface
	file      *os.File
	c         chan canModels.CanMessageTimestamped
	w         *bufio.Writer
	l         *slog.Logger
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
			canInterface.DBCFile,
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
	file, err := os.OpenFile(cfg.CRTDLogger.OutputFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening CRTD output file: %w", err)
	}

	writer := bufio.NewWriter(file)
	writeHeader(writer, cfg, logger)

	return &CRTDLoggerClient{
		w:       writer,
		file:    file,
		c:       make(chan canModels.CanMessageTimestamped, cfg.MessageBufferSize),
		filters: make(map[string]canModels.FilterInterface),
		l:       logger,
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
	defer c.file.Close()
	for canMsg := range c.c {
		c.HandleCanMessage(canMsg)
	}
	if err := c.w.Flush(); err != nil {
		c.l.Error("Could not flush CRTD file on channel close", "error", err)
	}
	return nil
}

func (c *CRTDLoggerClient) GetChannel() chan canModels.CanMessageTimestamped {
	return c.c
}

func (c *CRTDLoggerClient) GetName() string {
	return "output-crtd"
}

func (c *CRTDLoggerClient) Run() error {
	return nil
}
