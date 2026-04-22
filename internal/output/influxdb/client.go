package influxdb

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/InfluxCommunity/influxdb3-go/v2/influxdb3"

	"github.com/robbiebyrd/cantou/internal/client/common"
	canModels "github.com/robbiebyrd/cantou/internal/models"
)

type InfluxDBClient struct {
	client                *influxdb3.Client
	signalClient          *influxdb3.Client
	ctx                   context.Context
	measurementName       string
	signalMeasurementName string
	messageBlock          []canModels.CanMessageTimestamped
	signalBlock           []canModels.CanSignalTimestamped
	wg                    sync.WaitGroup
	signalWg              sync.WaitGroup
	maxBlocks             int
	maxConnections        int
	internalChannel       chan []canModels.CanMessageTimestamped
	signalInternalChannel chan []canModels.CanSignalTimestamped
	flushTime             int // ms
	l                     *slog.Logger
	canChannel            chan canModels.CanMessageTimestamped
	signalChannel         chan canModels.CanSignalTimestamped
	filters               *common.FilterSet
	resolver              canModels.InterfaceResolver
	canMsgCount           atomic.Uint64
	signalMsgCount        atomic.Uint64
}

func NewClient(
	ctx context.Context,
	cfg *canModels.Config,
	logger *slog.Logger,
	resolver canModels.InterfaceResolver,
	filters ...canModels.FilterInput,
) (canModels.OutputClient, error) {
	logger.Debug("starting influxdb3 client")

	clientCfg := influxdb3.ClientConfig{
		Host:     cfg.InfluxDB.Host,
		Token:    cfg.InfluxDB.Token,
		Database: cfg.InfluxDB.MessageDatabase,
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

	var signalClient *influxdb3.Client
	if cfg.InfluxDB.SignalDatabase != "" {
		signalCfg := clientCfg
		signalCfg.Database = cfg.InfluxDB.SignalDatabase
		signalClient, err = influxdb3.New(signalCfg)
		if err != nil {
			return nil, fmt.Errorf("creating InfluxDB3 signal client: %w", err)
		}
	}

	logger.Debug("started influxdb3 client")

	return &InfluxDBClient{
		client:                client,
		signalClient:          signalClient,
		ctx:                   ctx,
		l:                     logger,
		measurementName:       cfg.InfluxDB.TableName,
		signalMeasurementName: cfg.InfluxDB.SignalTableName,
		maxBlocks:             cfg.InfluxDB.MaxWriteLines,
		maxConnections:        cfg.InfluxDB.MaxConnections,
		flushTime:             cfg.InfluxDB.FlushTime,
		internalChannel: make(
			chan []canModels.CanMessageTimestamped,
			cfg.InfluxDB.MaxConnections,
		),
		signalInternalChannel: make(
			chan []canModels.CanSignalTimestamped,
			cfg.InfluxDB.MaxConnections,
		),
		canChannel:    make(chan canModels.CanMessageTimestamped, cfg.MessageBufferSize),
		signalChannel: make(chan canModels.CanSignalTimestamped, cfg.MessageBufferSize),
		messageBlock: make(
			[]canModels.CanMessageTimestamped,
			0,
			cfg.InfluxDB.MaxWriteLines,
		),
		signalBlock: make(
			[]canModels.CanSignalTimestamped,
			0,
			cfg.InfluxDB.MaxWriteLines,
		),
		wg:       sync.WaitGroup{},
		filters:  common.NewFilterSetFromInputs(filters),
		resolver: resolver,
	}, nil
}

func (c *InfluxDBClient) AddFilter(name string, filter canModels.FilterInterface) error {
	c.l.Debug("creating new filter group", "filterName", name)
	return c.filters.Add(name, filter)
}

func (c *InfluxDBClient) GetChannel() chan canModels.CanMessageTimestamped {
	return c.canChannel
}

// HandleCanMessage is a no-op for InfluxDB. Batching is done entirely inside
// HandleCanMessageChannel via a ticker; per-message handling is not used.
func (c *InfluxDBClient) HandleCanMessage(_ canModels.CanMessageTimestamped) {}

func (c *InfluxDBClient) HandleCanMessageChannel() error {
	c.l.Debug("starting channel handler")

	done := make(chan struct{})
	defer close(done)
	common.StartThroughputReporter(done, c.l, c.GetName(), "can", &c.canMsgCount, func() int { return len(c.canChannel) }, 5*time.Second)

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
		case <-c.ctx.Done():
			flush()
			close(c.internalChannel)
			c.wg.Wait()
			return nil
		case canMsg, ok := <-c.canChannel:
			if !ok {
				flush()
				close(c.internalChannel)
				c.wg.Wait()
				return nil
			}
			c.canMsgCount.Add(1)
			if shouldFilter, filterName := c.filters.ShouldFilter(canMsg); shouldFilter {
				c.l.Debug(
					"message filtered, dropping message",
					"message",
					canMsg,
					"filterName",
					filterName,
				)
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

	if c.signalClient != nil {
		signalGuard := make(chan struct{}, c.maxConnections)
		for i := 0; i < cap(signalGuard); i++ {
			signalGuard <- struct{}{}
			c.signalWg.Add(1)
			go func(id int) {
				defer c.signalWg.Done()
				defer func() { <-signalGuard }()
				c.signalWorker(id)
			}(i)
		}
	}

	return nil
}

func (c *InfluxDBClient) convertMsg(msg canModels.CanMessageTimestamped) InfluxDBCanMessage {
	// Format data bytes as "b1,b2,..." using a single growable buffer instead
	// of a []string + strings.Join, which would allocate a new string per byte.
	var dataBuf []byte
	if msg.Length > 0 {
		dataBuf = make([]byte, 0, int(msg.Length)*4)
		for i := 0; i < int(msg.Length); i++ {
			if i > 0 {
				dataBuf = append(dataBuf, ',')
			}
			dataBuf = strconv.AppendInt(dataBuf, int64(msg.Data[i]), 10)
		}
	}

	interfaceName := ""
	if conn := c.resolver.ConnectionByID(msg.Interface); conn != nil {
		interfaceName = conn.GetInterfaceName()
	}

	return InfluxDBCanMessage{
		Timestamp:   time.Unix(0, msg.Timestamp),
		ID:          formatCanID(msg.ID),
		Length:      msg.Length,
		Data:        string(dataBuf),
		Remote:      boolToInt(msg.Remote),
		Transmit:    boolToInt(msg.Transmit),
		Interface:   interfaceName,
		Measurement: c.measurementName,
	}
}

// formatCanID renders a CAN ID as lowercase hex, zero-padded to a minimum of
// 3 characters for standard 11-bit IDs. Replaces fmt.Sprintf("%03x", id) on the
// hot path to avoid reflection and fmt's allocations.
func formatCanID(id uint32) string {
	s := strconv.FormatUint(uint64(id), 16)
	if len(s) >= 3 {
		return s
	}
	// pad with leading zeros to length 3
	pad := [3]byte{'0', '0', '0'}
	copy(pad[3-len(s):], s)
	return string(pad[:])
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

func (c *InfluxDBClient) GetSignalChannel() chan canModels.CanSignalTimestamped {
	return c.signalChannel
}

func (c *InfluxDBClient) HandleSignal(sig canModels.CanSignalTimestamped) {
	if c.signalClient == nil {
		return
	}
	converted := c.convertSignal(sig)
	if err := c.signalClient.WriteData(c.ctx, []any{converted}); err != nil {
		c.l.Error("influxdb3 signal write failed", "error", err)
	}
}

func (c *InfluxDBClient) HandleSignalChannel() error {
	c.l.Debug("starting influxdb3 signal channel handler")

	done := make(chan struct{})
	defer close(done)
	common.StartThroughputReporter(done, c.l, c.GetName(), "signal", &c.signalMsgCount, func() int { return len(c.signalChannel) }, 5*time.Second)

	ticker := time.NewTicker(time.Duration(c.flushTime) * time.Millisecond)
	defer ticker.Stop()

	flush := func() {
		if len(c.signalBlock) == 0 {
			return
		}
		select {
		case c.signalInternalChannel <- c.signalBlock:
		default:
			c.l.Warn("influxdb: signal workers at capacity, dropping batch",
				"batch_size", len(c.signalBlock))
		}
		c.signalBlock = make([]canModels.CanSignalTimestamped, 0, c.maxBlocks)
	}

	for {
		select {
		case <-c.ctx.Done():
			flush()
			close(c.signalInternalChannel)
			c.signalWg.Wait()
			return nil
		case sig, ok := <-c.signalChannel:
			if !ok {
				flush()
				close(c.signalInternalChannel)
				c.signalWg.Wait()
				return nil
			}
			c.signalMsgCount.Add(1)
			c.signalBlock = append(c.signalBlock, sig)
			if len(c.signalBlock) >= c.maxBlocks {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (c *InfluxDBClient) signalWorker(i int) {
	c.l.Debug("influxdb3 signal worker started", "worker", i)
	for sigChunk := range c.signalInternalChannel {
		c.l.Debug("influxdb3 signal worker handling chunk", "worker", i)
		if err := c.writeSignals(sigChunk); err != nil {
			c.l.Error("influxdb3 signal write failed", "error", err)
		}
	}
}

func (c *InfluxDBClient) writeSignals(sigs []canModels.CanSignalTimestamped) error {
	if c.signalClient == nil {
		return nil
	}
	data := make([]any, len(sigs))
	for i, s := range sigs {
		data[i] = c.convertSignal(s)
	}
	return c.signalClient.WriteData(c.ctx, data)
}

func (c *InfluxDBClient) convertSignal(sig canModels.CanSignalTimestamped) InfluxDBSignalMessage {
	interfaceName := ""
	if conn := c.resolver.ConnectionByID(sig.Interface); conn != nil {
		interfaceName = conn.GetInterfaceName()
	}
	return InfluxDBSignalMessage{
		Timestamp:   time.Unix(0, sig.Timestamp),
		Interface:   interfaceName,
		Message:     sig.Message,
		Signal:      sig.Signal,
		Value:       sig.Value,
		Unit:        sig.Unit,
		Measurement: c.signalMeasurementName,
	}
}

func boolToInt(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
