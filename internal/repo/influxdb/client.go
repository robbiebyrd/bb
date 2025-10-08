package influxdb

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/InfluxCommunity/influxdb3-go/v2/influxdb3"

	canModels "github.com/robbiebyrd/bb/internal/models/can"
)

type InfluxDBClient struct {
	client          *influxdb3.Client
	ctx             context.Context
	measurementName string
	messageBlock    []canModels.CanMessage
	wg              sync.WaitGroup
	maxBlocks       int
	maxConnections  int
	connections     int
	channel         chan canModels.CanMessage
	flushTime       int // ms
	lastRun         time.Time
}

func NewClient(ctx *context.Context, cfg canModels.Config) (*InfluxDBClient, error) {
	client, err := influxdb3.New(influxdb3.ClientConfig{
		Host:     cfg.InfluxHost,
		Token:    cfg.InfluxToken,
		Database: cfg.InfluxDatabase,
	})
	if err != nil {
		return nil, err
	}

	channel := make(chan canModels.CanMessage)

	return &InfluxDBClient{
		client:          client,
		ctx:             *ctx,
		measurementName: cfg.InfluxTableName,
		wg:              sync.WaitGroup{},
		maxBlocks:       cfg.InfluxMaxWriteLines,
		maxConnections:  5,
		connections:     0,
		channel:         channel,
		flushTime:       cfg.InfluxFlushTime,
		lastRun:         time.Now(),
	}, nil
}

func (c *InfluxDBClient) Run() {
	c.wg.Go(func() {
		for {
			blockSize := min(len(c.messageBlock), c.maxBlocks)
			a := time.Since(c.lastRun)

			if blockSize > 0 && c.connections < c.maxConnections && a.Milliseconds() > int64(c.flushTime) {
				c.connections++
				go c.write(c.convertMany(c.messageBlock[:blockSize]))
				c.messageBlock = c.messageBlock[blockSize:]
				c.connections--
				c.lastRun = time.Now()
			}
		}
	})
}

func (c *InfluxDBClient) Handle(msg canModels.CanMessage) {
	fmt.Printf("queuing #%v\n", len(c.messageBlock)+1)
	c.messageBlock = append(c.messageBlock, msg)
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

func (c *InfluxDBClient) write(msg []InfluxDBCanMessage) {
	data := make([]any, len(msg))
	for i, m := range msg {
		data[i] = m
	}

	err := c.client.WriteData(c.ctx, data)
	if err != nil {
		panic(err)
	}
}

func boolToInt(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
