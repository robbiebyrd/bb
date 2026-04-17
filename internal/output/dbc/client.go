package dbc

import (
	"context"
	"fmt"
	"log/slog"

	dbcParser "github.com/robbiebyrd/bb/internal/parser/dbc"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

type DBCOutputClient struct {
	parser          canModels.ParserInterface
	incomingChannel chan canModels.CanMessageTimestamped
	filters         map[string]canModels.FilterInterface
	l               *slog.Logger
}

func NewClient(ctx context.Context, cfg *canModels.Config, logger *slog.Logger, dbcFilePath string) canModels.OutputClient {
	return &DBCOutputClient{
		parser:          dbcParser.NewDBCParserClient(logger, dbcFilePath),
		incomingChannel: make(chan canModels.CanMessageTimestamped, cfg.MessageBufferSize),
		filters:         make(map[string]canModels.FilterInterface),
		l:               logger,
	}
}

func (c *DBCOutputClient) HandleCanMessage(canMsg canModels.CanMessageTimestamped) {
	if shouldFilter, filterName := c.shouldFilterMessage(canMsg); shouldFilter {
		c.l.Debug("message filtered, dropping message", "message", canMsg, "filterName", *filterName)
		return
	}

	result := c.parser.Parse(canModels.CanMessageData{
		Interface: canMsg.Interface,
		ID:        canMsg.ID,
		Transmit:  canMsg.Transmit,
		Remote:    canMsg.Remote,
		Length:    canMsg.Length,
		Data:      canMsg.Data,
	})
	if result != nil {
		c.l.Info("decoded CAN frame", "interface", canMsg.Interface, "id", canMsg.ID, "decoded", result)
	}
}

func (c *DBCOutputClient) HandleCanMessageChannel() error {
	for canMsg := range c.incomingChannel {
		c.HandleCanMessage(canMsg)
	}
	return nil
}

func (c *DBCOutputClient) GetChannel() chan canModels.CanMessageTimestamped {
	return c.incomingChannel
}

func (c *DBCOutputClient) GetName() string {
	return "output-dbc"
}

func (c *DBCOutputClient) Run() error {
	return nil
}

func (c *DBCOutputClient) AddFilter(name string, filter canModels.FilterInterface) error {
	if _, ok := c.filters[name]; ok {
		return fmt.Errorf("filter group already exists: %v", name)
	}
	c.l.Debug("creating new filter group", "filterName", name)
	c.filters[name] = filter
	return nil
}

func (c *DBCOutputClient) shouldFilterMessage(canMsg canModels.CanMessageTimestamped) (bool, *string) {
	for name, filter := range c.filters {
		if filter.Filter(canMsg) {
			return true, &name
		}
	}
	return false, nil
}
