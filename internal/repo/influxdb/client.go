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
	channel         chan InfluxDBCanMessage
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

	return &InfluxDBClient{
		client:          client,
		ctx:             *ctx,
		measurementName: cfg.InfluxTableName,
		wg:              sync.WaitGroup{},
	}, nil
}

func (c *InfluxDBClient) Convert(msg canModels.CanMessage) InfluxDBCanMessage {

	newMsgData := make([]string, len(msg.Data[:msg.Length]))
	for i, b := range msg.Data[:msg.Length] {
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

	fmt.Printf("Message Converted: %v\n", msgConverted)
	return msgConverted
}

func (c *InfluxDBClient) ConvertMany(msgs []canModels.CanMessage) []InfluxDBCanMessage {
	var convertedMessages []InfluxDBCanMessage
	for _, m := range msgs {
		convertedMessages = append(convertedMessages, c.Convert(m))
	}
	return convertedMessages
}

func (c *InfluxDBClient) Run() {
	select {
	case msg := <-c.channel:
		fmt.Println("received", msg)
	default: // This case runs if `messages` has no value ready
		fmt.Println("No message received")
	}

	c.wg.Go(func() {
		for {
			maxBlocks := 50
			if len(c.messageBlock) <= maxBlocks {
				maxBlocks = len(c.messageBlock)
			}
			msgs := c.messageBlock[0:maxBlocks]
			c.messageBlock = c.messageBlock[maxBlocks:]
			c.WriteMany(c.ConvertMany(msgs))
			time.Sleep(100 * time.Millisecond)
		}
	})
}

func (c *InfluxDBClient) Handle(msg canModels.CanMessage) {
	fmt.Printf("queueing #%v\n", len(c.messageBlock)+1)
	c.messageBlock = append(c.messageBlock, msg)
}

func (c *InfluxDBClient) Write(msg InfluxDBCanMessage) {
	err := c.client.WriteData(c.ctx, []any{msg})
	if err != nil {
		panic(err)
	}
}

func (c *InfluxDBClient) WriteMany(msg []InfluxDBCanMessage) {
	data := make([]any, len(msg))
	for i, m := range msg {
		data[i] = m
		fmt.Printf("Writing data: %v", m)
		err := c.client.WriteData(c.ctx, []any{m})
		if err != nil {
			panic(err)
		}
	}
}

func boolToInt(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
