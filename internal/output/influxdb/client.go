package influxdb

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
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
	maxBlocks       int
	maxConnections  int
	internalChannel chan []canModels.CanMessageTimestamped
	flushTime       int // ms
	l               *slog.Logger
	incomingChannel chan canModels.CanMessageTimestamped
	filters         map[string]canModels.FilterInterface
	resolver        canModels.InterfaceResolver
}

func NewClient(ctx context.Context, cfg *canModels.Config, logger *slog.Logger, resolver canModels.InterfaceResolver) (canModels.OutputClient, error) {
	logger.Debug("starting influxdb3 client")

	clientCfg := influxdb3.ClientConfig{
		Host:     cfg.InfluxDB.Host,
		Token:    cfg.InfluxDB.Token,
		Database: cfg.InfluxDB.Database,
	}

	if cfg.InfluxDB.TLS {
		if cfg.InfluxDB.TLSCACertFile != "" {
			// Use SSLRootsFilePath to load a custom CA certificate.
			clientCfg.SSLRootsFilePath = cfg.InfluxDB.TLSCACertFile
		} else {
			// Verify the CA cert pool from the system roots via a custom http.Client.
			pool, err := x509.SystemCertPool()
			if err != nil {
				return nil, fmt.Errorf("loading system cert pool for InfluxDB TLS: %w", err)
			}
			clientCfg.HTTPClient = &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{RootCAs: pool},
				},
			}
		}
	}

	client, err := influxdb3.New(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("creating InfluxDB3 client: %w", err)
	}

	logger.Debug("started influxdb3 client")

	return &InfluxDBClient{
		client:          client,
		ctx:             ctx,
		l:               logger,
		measurementName: cfg.InfluxDB.TableName,
		maxBlocks:       cfg.InfluxDB.MaxWriteLines,
		maxConnections:  cfg.InfluxDB.MaxConnections,
		flushTime:       cfg.InfluxDB.FlushTime,
		internalChannel: make(chan []canModels.CanMessageTimestamped, cfg.InfluxDB.MaxConnections),
		incomingChannel: make(chan canModels.CanMessageTimestamped, cfg.MessageBufferSize),
		messageBlock:    make([]canModels.CanMessageTimestamped, 0, cfg.InfluxDB.MaxWriteLines),
		wg:              sync.WaitGroup{},
		filters:         make(map[string]canModels.FilterInterface),
		resolver:        resolver,
	}, nil
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

// HandleCanMessage is a no-op for InfluxDB. Batching is done entirely inside
// HandleCanMessageChannel via a ticker; per-message handling is not used.
func (c *InfluxDBClient) HandleCanMessage(_ canModels.CanMessageTimestamped) {}

func (c *InfluxDBClient) HandleCanMessageChannel() error {
	c.l.Debug("starting channel handler")

	ticker := time.NewTicker(time.Duration(c.flushTime) * time.Millisecond)
	defer ticker.Stop()

	flush := func() {
		if len(c.messageBlock) == 0 {
			return
		}
		select {
		case c.internalChannel <- c.messageBlock:
		default:
			c.l.Warn("influxdb: workers at capacity, dropping batch",
				"batch_size", len(c.messageBlock))
		}
		c.messageBlock = make([]canModels.CanMessageTimestamped, 0, c.maxBlocks)
	}

	for {
		select {
		case canMsg, ok := <-c.incomingChannel:
			if !ok {
				flush()
				close(c.internalChannel)
				c.wg.Wait()
				return nil
			}
			if shouldFilter, filterName := c.shouldFilterMessage(canMsg); shouldFilter {
				c.l.Debug("message filtered, dropping message", "message", canMsg, "filterName", *filterName)
				continue
			}
			c.messageBlock = append(c.messageBlock, canMsg)
			c.l.Debug("message block count", "count", len(c.messageBlock))
			if len(c.messageBlock) >= c.maxBlocks {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (c *InfluxDBClient) GetName() string {
	return "output-influxdb3"
}

func (c *InfluxDBClient) worker(i int) {
	c.l.Debug("influxdb3 chunk worker started", "worker", i)
	for msgChunk := range c.internalChannel {
		c.l.Debug("influxdb3 chunk worker handling chunk", "worker", i)
		if err := c.write(c.convertMany(msgChunk)); err != nil {
			c.l.Error("influxdb3 write failed", "error", err)
		}
		c.l.Debug("influxdb3 chunk worker finished handling chunk", "worker", i)
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

	interfaceName := ""
	if conn := c.resolver.ConnectionByID(msg.Interface); conn != nil {
		interfaceName = conn.GetInterfaceName()
	}

	msgConverted := InfluxDBCanMessage{
		Timestamp:   time.Unix(0, msg.Timestamp),
		ID:          fmt.Sprintf("%03x", msg.ID),
		Length:      msg.Length,
		Data:        strings.Join(newMsgData, ","),
		Remote:      boolToInt(msg.Remote),
		Transmit:    boolToInt(msg.Transmit),
		Interface:   interfaceName,
		Measurement: c.measurementName,
	}

	return msgConverted
}

func (c *InfluxDBClient) convertMany(msgs []canModels.CanMessageTimestamped) []InfluxDBCanMessage {
	convertedMessages := make([]InfluxDBCanMessage, 0, len(msgs))
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
