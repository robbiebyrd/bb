package csv

import (
	"context"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

type CSVClient struct {
	w               *csv.Writer
	includeHeaders  bool
	messageBlock    []canModels.CanMessageTimestamped
	incomingChannel chan canModels.CanMessageTimestamped
	filters         map[string]canModels.FilterInterface
	l               *slog.Logger
	resolver        canModels.InterfaceResolver
}

func NewClient(
	ctx *context.Context,
	cfg *canModels.Config,
	logger *slog.Logger,
	resolver canModels.InterfaceResolver,
) canModels.OutputClient {
	file, err := os.OpenFile(cfg.CSVLog.OutputFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		fmt.Printf("Error opening file %v\n", cfg.CSVLog.OutputFile)
		panic(err)
	}

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{"timestamp", "id", "interface", "remote", "transmit", "length", "data"}
	err = writer.Write(header)
	if err != nil {
		logger.Error("Could not write row to csv file", "error", err)
	}

	return &CSVClient{
		w:               writer,
		includeHeaders:  cfg.CSVLog.IncludeHeaders,
		messageBlock:    []canModels.CanMessageTimestamped{},
		incomingChannel: make(chan canModels.CanMessageTimestamped, cfg.MessageBufferSize),
		filters:         make(map[string]canModels.FilterInterface),
		l:               logger,
		resolver:        resolver,
	}
}

func (c *CSVClient) AddFilter(name string, filter canModels.FilterInterface) error {
	if _, ok := c.filters[name]; ok {
		return fmt.Errorf("filter group already exists: %v", name)
	}
	c.l.Debug("creating new filter group", "filterName", name)
	c.filters[name] = filter
	return nil
}

func (c *CSVClient) Handle(canMsg canModels.CanMessageTimestamped) {
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
	err := c.w.Write(row)
	if err != nil {
		c.l.Error("Could not write row to csv file", "error", err)
	}
	c.w.Flush()
}

func (c *CSVClient) HandleChannel() error {
	for canMsg := range c.incomingChannel {
		c.Handle(canMsg)
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
