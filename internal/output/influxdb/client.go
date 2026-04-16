package influxdb

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/InfluxCommunity/influxdb3-go/v2/influxdb3"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

type InfluxDBClient struct {
	client          *influxdb3.Client
	ctx             context.Context
	measurementName string
	messageBlock    []canModels.CanMessageTimestamped
	wg              sync.WaitGroup
	mu              sync.Mutex
	maxBlocks       int
	maxConnections  int
	internalChannel chan []canModels.CanMessageTimestamped
	flushTime       int // ms
	l               *slog.Logger
	workerLastRan   time.Time
	count           int
	incomingChannel chan canModels.CanMessageTimestamped
	filters         map[string]canModels.FilterInterface
}

func NewClient(ctx *context.Context, cfg *canModels.Config, logger *slog.Logger) canModels.OutputClient {
	logger.Debug("starting influxdb3 client")
	client, err := influxdb3.New(influxdb3.ClientConfig{
		Host:     cfg.InfluxDB.Host,
		Token:    cfg.InfluxDB.Token,
		Database: cfg.InfluxDB.Database,
	})
	if err != nil {
		panic(err)
	}

	logger.Debug("started influxdb3 client")

	now := time.Now()
	return &InfluxDBClient{
		client:          client,
		ctx:             *ctx,
		l:               logger,
		workerLastRan:   now,
		measurementName: cfg.InfluxDB.TableName,
		maxBlocks:       cfg.InfluxDB.MaxWriteLines,
		maxConnections:  cfg.InfluxDB.MaxConnections,
		flushTime:       cfg.InfluxDB.FlushTime,
		internalChannel: make(chan []canModels.CanMessageTimestamped),
		incomingChannel: make(chan canModels.CanMessageTimestamped, cfg.MessageBufferSize),
		wg:              sync.WaitGroup{},
		filters:         make(map[string]canModels.FilterInterface),
	}
}

func (c *InfluxDBClient) Handle(canMsg canModels.CanMessageTimestamped) {
	if shouldFilter, filterName := c.shouldFilterMessage(canMsg); shouldFilter {
		c.l.Debug("message filtered, dropping message", "message", canMsg, "filterName", *filterName)
		return
	}

	c.messageBlock = append(c.messageBlock, canMsg)
	c.l.Debug("message block count", "count", len(c.messageBlock))
	c.mu.Lock()
	lastRan := c.workerLastRan
	c.mu.Unlock()
	if len(c.messageBlock) >= c.maxBlocks || time.Since(lastRan) >= time.Duration(c.flushTime)*time.Millisecond {
		c.internalChannel <- c.messageBlock
		c.messageBlock = []canModels.CanMessageTimestamped{}
	}
}

func (c *InfluxDBClient) AddFilter(name string, filter canModels.FilterInterface) error {
	if _, ok := c.filters[name]; ok {
		return fmt.Errorf("filter group already exists: %v", name)
	}
	c.l.Debug("creating new filter group", "filterName", name)
	c.filters[name] = filter
	return nil
}

func (c *InfluxDBClient) GetChannel() chan canModels.CanMessageTimestamped {
	return c.incomingChannel
}

func (c *InfluxDBClient) HandleChannel() error {
	c.l.Debug("starting channel handler")
	for canMsg := range c.incomingChannel {
		c.Handle(canMsg)
	}
	return nil
}

func (c *InfluxDBClient) GetName() string {
	return "output-influxdb3"
}

func (c *InfluxDBClient) worker(i int) {
	c.l.Debug(fmt.Sprintf("influxdb3 chunk worker %v started", i))
	for msgChunk := range c.internalChannel {
		c.l.Debug(fmt.Sprintf("influxdb3 chunk worker %v handling chunk", i))
		if err := c.write(c.convertMany(msgChunk)); err != nil {
			c.l.Error("influxdb3 write failed", "error", err)
		} else {
			now := time.Now()
			c.mu.Lock()
			c.workerLastRan = now
			c.count += len(msgChunk)
			c.mu.Unlock()
		}
		c.l.Debug(fmt.Sprintf("influxdb3 chunk worker %v finished handling chunk", i))
	}
}

func (c *InfluxDBClient) Run() error {
	guard := make(chan struct{}, c.maxConnections)

	for i := 0; i < cap(guard); i++ {
		guard <- struct{}{}
		c.wg.Add(1)
		go func(id int) {
			defer c.wg.Done()
			defer func() { <-guard }()
			c.worker(id)
		}(i)
	}

	return nil
}

func (c *InfluxDBClient) convertMsg(msg canModels.CanMessageTimestamped) InfluxDBCanMessage {
	newMsgData := make([]string, msg.Length)
	for i := 0; i < int(msg.Length); i++ {
		newMsgData[i] = strconv.Itoa(int(msg.Data[i]))
	}

	msgConverted := InfluxDBCanMessage{
		Timestamp:   time.Unix(msg.Timestamp, 0),
		ID:          fmt.Sprintf("%03x", msg.ID),
		Length:      msg.Length,
		Data:        strings.Join(newMsgData, ","),
		Remote:      boolToInt(msg.Remote),
		Transmit:    boolToInt(msg.Transmit),
		Interface:   msg.Interface,
		Measurement: c.measurementName,
	}

	return msgConverted
}

func (c *InfluxDBClient) convertMany(msgs []canModels.CanMessageTimestamped) []InfluxDBCanMessage {
	var convertedMessages []InfluxDBCanMessage
	for _, m := range msgs {
		convertedMessages = append(convertedMessages, c.convertMsg(m))
	}
	return convertedMessages
}

func (c *InfluxDBClient) write(msg []InfluxDBCanMessage) error {
	data := make([]any, len(msg))
	for i, m := range msg {
		data[i] = m
	}

	return c.client.WriteData(c.ctx, data)
}

func boolToInt(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

func (c *InfluxDBClient) shouldFilterMessage(canMsg canModels.CanMessageTimestamped) (bool, *string) {
	for name, filter := range c.filters {
		if filter.Filter(canMsg) {
			return true, &name
		}
	}
	return false, nil
}
