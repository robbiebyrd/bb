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
	w              *csv.Writer
	includeHeaders bool
	messageBlock   []canModels.CanMessage
}

func NewClient(ctx *context.Context, cfg canModels.Config, logger *slog.Logger) canModels.DBClient {
	file, err := os.OpenFile(cfg.CSVLog.OutputFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		fmt.Printf("Error opening file with flags...")
		panic(err)
	}

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{"timestamp", "id", "interface", "remote", "transmit", "length", "data"}
	writer.Write(header)

	return &CSVClient{
		w:              writer,
		includeHeaders: cfg.CSVLog.IncludeHeaders,
		messageBlock:   []canModels.CanMessage{},
	}
}

func (c *CSVClient) Handle(msg canModels.CanMessage) {
	row := []string{strconv.FormatInt(msg.Timestamp, 10), strconv.FormatUint(uint64(msg.ID), 10), msg.Interface, strconv.FormatBool(msg.Remote), strconv.FormatBool(msg.Transmit), strconv.Itoa(int(msg.Length)), hex.EncodeToString(msg.Data)}
	c.w.Write(row)
	c.w.Flush()
}

func (c *CSVClient) HandleChannel(channel chan canModels.CanMessage) {
	for msg := range channel {
		c.Handle(msg)
	}
}

func (c *CSVClient) Run() error {
	return nil
}
