package csv

import (
	"context"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/robbiebyrd/bb/internal/client/common"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

type CSVClient struct {
	w               *csv.Writer
	includeHeaders  bool
	incomingChannel chan canModels.CanMessageTimestamped
	filters         map[string]canModels.FilterInterface
	l               *slog.Logger
	resolver        canModels.InterfaceResolver
}

func NewClient(ctx context.Context, cfg *canModels.Config, logger *slog.Logger, resolver canModels.InterfaceResolver) (canModels.OutputClient, error) {
	file, err := os.OpenFile(cfg.CSVLog.OutputFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening CSV output file: %w", err)
	}

	writer := csv.NewWriter(file)

	header := []string{"timestamp", "id", "interface", "remote", "transmit", "length", "data"}
	if err = writer.Write(header); err != nil {
		logger.Error("csv write header error", "error", err)
	}

	return &CSVClient{
		w:               writer,
		includeHeaders:  cfg.CSVLog.IncludeHeaders,
		incomingChannel: make(chan canModels.CanMessageTimestamped, cfg.MessageBufferSize),
		filters:         make(map[string]canModels.FilterInterface),
		l:               logger,
		resolver:        resolver,
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
	for canMsg := range c.incomingChannel {
		c.HandleCanMessage(canMsg)
	}
	c.w.Flush()
	if err := c.w.Error(); err != nil {
		c.l.Error("csv flush error", "error", err)
	}
	return nil
}

func (c *CSVClient) GetChannel() chan canModels.CanMessageTimestamped {
	return c.incomingChannel
}

func (c *CSVClient) GetName() string {
	return "output-csv"
}

func (c *CSVClient) Run() error {
	return nil
}
