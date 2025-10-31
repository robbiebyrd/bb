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
	messageBlock    []canModels.CanMessage
	incomingChannel chan canModels.CanMessage
}

func NewClient(ctx *context.Context, cfg canModels.Config, logger *slog.Logger) canModels.OutputClient {
	file, err := os.OpenFile(cfg.CSVLog.OutputFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		fmt.Printf("Error opening file %v\n")
		panic(err)
	}

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{"timestamp", "id", "interface", "remote", "transmit", "length", "data"}
	writer.Write(header)

	return &CSVClient{
		w:               writer,
		includeHeaders:  cfg.CSVLog.IncludeHeaders,
		messageBlock:    []canModels.CanMessage{},
		incomingChannel: make(chan canModels.CanMessage, cfg.MessageBufferSize),
	}
}

func (c *CSVClient) Handle(canMsg canModels.CanMessage) {
	row := []string{
		strconv.FormatInt(canMsg.Timestamp, 10),
		strconv.FormatUint(uint64(canMsg.ID), 10),
		canMsg.Interface,
		strconv.FormatBool(canMsg.Remote),
		strconv.FormatBool(canMsg.Transmit),
		strconv.Itoa(int(canMsg.Length)),
		hex.EncodeToString(canMsg.Data)}
	c.w.Write(row)
	c.w.Flush()
}

func (c *CSVClient) HandleChannel() error {
	for canMsg := range c.incomingChannel {
		c.Handle(canMsg)
	}
	return nil
}

func (c *CSVClient) GetChannel() chan canModels.CanMessage {
	return c.incomingChannel
}

func (c *CSVClient) GetName() string {
	return "output-csv"
}

func (c *CSVClient) Run() error {
	return nil
}
