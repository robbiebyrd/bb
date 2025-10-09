package csv

import (
	"context"
	"log/slog"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

type CSVClient struct {
	outputPath     string
	includeHeaders bool
	messageBlock   []canModels.CanMessage
}

func NewClient(ctx *context.Context, cfg canModels.Config, logger *slog.Logger, channel chan canModels.CanMessage) (canModels.DBClient, error) {
	return &CSVClient{
		outputPath:     "",
		includeHeaders: false,
		messageBlock:   []canModels.CanMessage{},
	}, nil
}

func (c *CSVClient) Handle(msg canModels.CanMessage) {
	c.messageBlock = append(c.messageBlock, msg)
}

func (c *CSVClient) HandleChannel(channel chan canModels.CanMessage) {
	for msg := range channel {
		c.Handle(msg)
	}
}

func (c *CSVClient) Run() error {
	return nil
}
