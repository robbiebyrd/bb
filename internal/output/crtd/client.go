package crtd

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

type CRTDLoggerClient struct {
	w       *bufio.Writer
	l       *slog.Logger
	c       chan canModels.CanMessageTimestamped
	file    *os.File
	filters map[string]canModels.FilterInterface
}

func NewClient(
	ctx context.Context,
	cfg *canModels.Config,
	logger *slog.Logger,
) canModels.OutputClient {
	file, err := os.OpenFile(cfg.CRTDLogger.OutputFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		fmt.Printf("Error opening file %v\n", cfg.CRTDLogger.OutputFile)
		panic(err)
	}

	writer := bufio.NewWriter(file)
	_, err = fmt.Fprintln(writer, "0.000000 CXX CRTD file created by bb")
	if err != nil {
		logger.Error("Could not write header to CRTD file", "error", err)
	}
	err = writer.Flush()
	if err != nil {
		logger.Error("Could not flush CRTD file when writing header", "error", err)
	}

	return &CRTDLoggerClient{
		w:       writer,
		file:    file,
		c:       make(chan canModels.CanMessageTimestamped, cfg.MessageBufferSize),
		filters: make(map[string]canModels.FilterInterface),
		l:       logger,
	}
}

func (c *CRTDLoggerClient) AddFilter(name string, filter canModels.FilterInterface) error {
	if _, ok := c.filters[name]; ok {
		return fmt.Errorf("filter group already exists: %v", name)
	}
	c.l.Debug("creating new filter group", "filterName", name)
	c.filters[name] = filter
	return nil
}

func (c *CRTDLoggerClient) Handle(canMsg canModels.CanMessageTimestamped) {
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

	_, err := fmt.Fprintln(c.w, line)
	if err != nil {
		c.l.Error("Could not write record to CRTD file", "error", err)
	}
	err = c.w.Flush()
	if err != nil {
		c.l.Error("Could not flush CRTD file when writing header", "error", err)
	}
}

func (c *CRTDLoggerClient) HandleChannel() error {
	for canMsg := range c.c {
		c.Handle(canMsg)
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
