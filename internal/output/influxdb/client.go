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
	messageBlock    []canModels.CanMessage
	wg              sync.WaitGroup
	maxBlocks       int
	maxConnections  int
	internalChannel chan []canModels.CanMessage
	flushTime       int // ms
	l               *slog.Logger
	workerLastRan   time.Time
	count           int
	incomingChannel chan canModels.CanMessage
}

func NewClient(ctx *context.Context, cfg canModels.Config, logger *slog.Logger) canModels.OutputClient {
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
		internalChannel: make(chan []canModels.CanMessage),
		incomingChannel: make(chan canModels.CanMessage, cfg.MessageBufferSize),
		wg:              sync.WaitGroup{},
	}
}

func (c *InfluxDBClient) Handle(canMsg canModels.CanMessage) {
	c.messageBlock = append(c.messageBlock, canMsg)
	if len(c.messageBlock) >= c.maxBlocks {
		c.internalChannel <- c.messageBlock
		c.messageBlock = []canModels.CanMessage{}
	}
}

func (c *InfluxDBClient) GetChannel() chan canModels.CanMessage {
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
	c.l.Info(fmt.Sprintf("chunk worker %v started", i))
	for msgChunk := range c.internalChannel {
		c.l.Info(fmt.Sprintf("chunk worker %v handling chunk", i))
		if err := c.write(c.convertMany(msgChunk)); err != nil {
			panic(err)
		} else {
			now := time.Now()
			c.workerLastRan = now
			c.count += len(msgChunk)
		}
		c.l.Info(fmt.Sprintf("chunk worker %v finished handling chunk", i))
	}
}

func (c *InfluxDBClient) Run() error {
	guard := make(chan struct{}, c.maxConnections)

	for i := 0; i < cap(guard); i++ {
		guard <- struct{}{}
		c.wg.Add(1)
		go func(id int) {
			defer func() { <-guard }()
			go c.worker(i)
		}(i)
	}

	c.wg.Wait()

	return nil
}

func (c *InfluxDBClient) convertMsg(msg canModels.CanMessage) InfluxDBCanMessage {
	newMsgData := make([]string, len(msg.Data))
	for i, b := range msg.Data {
		newMsgData[i] = strconv.Itoa(int(b - '0'))
	}

	msgConverted := InfluxDBCanMessage{
		Timestamp:   time.Unix(msg.Timestamp, 0),
		ID:          fmt.Sprintf("0x%x", msg.ID),
		Length:      msg.Length,
		Data:        fmt.Sprintf("[%v]", strings.Join(newMsgData, ",")),
		Remote:      boolToInt(msg.Remote),
		Transmit:    boolToInt(msg.Transmit),
		Interface:   msg.Interface,
		Measurement: c.measurementName,
	}

	return msgConverted
}

func (c *InfluxDBClient) convertMany(msgs []canModels.CanMessage) []InfluxDBCanMessage {
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

	err := c.client.WriteData(c.ctx, data)
	if err != nil {
		return err
	}

	return nil
}

func boolToInt(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
