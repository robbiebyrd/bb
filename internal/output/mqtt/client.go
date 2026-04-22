package mqtt

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/robbiebyrd/cantou/internal/client/common"
	canModels "github.com/robbiebyrd/cantou/internal/models"
)

type MQTTClient struct {
	client         mqtt.Client
	ctx            context.Context
	l              *slog.Logger
	canChannel     chan canModels.CanMessageTimestamped
	signalChannel  chan canModels.CanSignalTimestamped
	topic          string
	qos            uint8
	shadowCopy     bool
	filters        *common.FilterSet
	resolver       canModels.InterfaceResolver
	canMsgCount    atomic.Uint64
	signalMsgCount atomic.Uint64
}

func NewClient(
	ctx context.Context,
	cfg *canModels.Config,
	logger *slog.Logger,
	resolver canModels.InterfaceResolver,
	filters ...canModels.FilterInput,
) (canModels.OutputClient, error) {
	logger.Debug("starting MQTT client")

	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.MQTTConfig.Host)
	opts.SetClientID(cfg.MQTTConfig.ClientId)
	if cfg.MQTTConfig.Username != "" {
		opts.SetUsername(cfg.MQTTConfig.Username)
		opts.SetPassword(cfg.MQTTConfig.Password)
	}
	if cfg.MQTTConfig.TLS {
		tlsCfg, err := buildTLSConfig(cfg.MQTTConfig)
		if err != nil {
			return nil, fmt.Errorf("building TLS config: %w", err)
		}
		opts.SetTLSConfig(tlsCfg)
	}
	client := mqtt.NewClient(opts)

	logger.Debug(
		"connecting MQTT client",
		"host",
		cfg.MQTTConfig.Host,
		"clientId",
		cfg.MQTTConfig.ClientId,
	)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Error(
			"error connecting MQTT client",
			"host",
			cfg.MQTTConfig.Host,
			"clientId",
			cfg.MQTTConfig.ClientId,
			"error",
			token.Error(),
		)
		return nil, fmt.Errorf("connecting to MQTT broker: %w", token.Error())
	}

	logger.Debug("started MQTT client")

	return &MQTTClient{
		client:        client,
		ctx:           ctx,
		l:             logger,
		canChannel:    make(chan canModels.CanMessageTimestamped, cfg.MessageBufferSize),
		signalChannel: make(chan canModels.CanSignalTimestamped, cfg.MessageBufferSize),
		topic:         cfg.MQTTConfig.Topic,
		qos:           cfg.MQTTConfig.Qos,
		shadowCopy:    cfg.MQTTConfig.ShadowCopy,
		filters:       common.NewFilterSetFromInputs(filters),
		resolver:      resolver,
	}, nil
}

func (c *MQTTClient) AddFilter(name string, filter canModels.FilterInterface) error {
	c.l.Debug("creating new filter group", "filterName", name)
	return c.filters.Add(name, filter)
}

func (c *MQTTClient) HandleCanMessage(canMsg canModels.CanMessageTimestamped) {
	if !c.client.IsConnectionOpen() {
		c.l.Error("MQTT client not connected, dropping message")
		return
	}

	if shouldFilter, filterName := c.filters.ShouldFilter(canMsg); shouldFilter {
		if c.l.Enabled(context.Background(), slog.LevelDebug) {
			msgString, err := c.ToJSON(canMsg)
			if err != nil {
				c.l.Debug(
					"message filtered, dropping message (serialize error)",
					"error",
					err,
					"filterName",
					filterName,
				)
			} else {
				c.l.Debug("message filtered, dropping message", "message", msgString, "filterName", filterName)
			}
		}
		return
	}

	topic := c.getTopicFromMessage(canMsg)
	payload, err := c.toJSONBytes(canMsg)
	if err != nil {
		c.l.Error("MQTT failed to serialize message", "error", err)
		return
	}

	c.publish(topic, payload)
	if c.l.Enabled(context.Background(), slog.LevelDebug) {
		c.l.Debug("MQTT published message", "topic", topic, "message", string(payload))
	}
}

func (c *MQTTClient) GetChannel() chan canModels.CanMessageTimestamped {
	return c.canChannel
}

func (c *MQTTClient) HandleCanMessageChannel() error {
	c.l.Debug("starting MQTT channel handler")
	done := make(chan struct{})
	defer close(done)
	common.StartThroughputReporter(done, c.l, c.GetName(), "can", &c.canMsgCount, func() int { return len(c.canChannel) }, 5*time.Second)
	for canMsg := range c.canChannel {
		c.canMsgCount.Add(1)
		c.HandleCanMessage(canMsg)
	}
	return nil
}

func (c *MQTTClient) GetSignalChannel() chan canModels.CanSignalTimestamped {
	return c.signalChannel
}

func (c *MQTTClient) HandleSignal(sig canModels.CanSignalTimestamped) {
	if !c.client.IsConnectionOpen() {
		c.l.Error("MQTT client not connected, dropping signal")
		return
	}

	topic := c.getTopicFromSignal(sig)
	payload := signalPayload(sig)

	c.publish(topic, payload)
	c.l.Debug("MQTT published signal", "topic", topic, "signal", sig.Signal)
}

func (c *MQTTClient) HandleSignalChannel() error {
	c.l.Debug("starting MQTT signal channel handler")
	done := make(chan struct{})
	defer close(done)
	common.StartThroughputReporter(done, c.l, c.GetName(), "signal", &c.signalMsgCount, func() int { return len(c.signalChannel) }, 5*time.Second)
	for sig := range c.signalChannel {
		c.signalMsgCount.Add(1)
		c.HandleSignal(sig)
	}
	return nil
}

func (c *MQTTClient) GetName() string {
	return "output-mqtt"
}

// publishAckTimeout bounds the time we'll wait for a publish acknowledgment
// before giving up and logging. Keeps the per-message goroutine from leaking
// indefinitely when the broker stalls.
const publishAckTimeout = 5 * time.Second

// publish sends payload to topic. For QoS 0 the token completes synchronously
// and the error is checked inline. For QoS > 0 we wait for the broker ack in
// a goroutine bounded by publishAckTimeout, which prevents per-message
// goroutine leaks if the broker becomes unresponsive.
func (c *MQTTClient) publish(topic string, payload any) {
	token := c.client.Publish(topic, c.qos, c.shadowCopy, payload)
	if c.qos == 0 {
		if err := token.Error(); err != nil {
			c.l.Error("MQTT publish failed", "error", err)
		}
		return
	}
	go func(t mqtt.Token) {
		if !t.WaitTimeout(publishAckTimeout) {
			c.l.Warn("MQTT publish ack timed out", "timeout", publishAckTimeout)
			return
		}
		if err := t.Error(); err != nil {
			c.l.Error("MQTT publish failed", "error", err)
		}
	}(token)
}

func (c *MQTTClient) getTopicFromMessage(canMsg canModels.CanMessageTimestamped) string {
	name := "unknown"
	if conn := c.resolver.ConnectionByID(canMsg.Interface); conn != nil {
		name = conn.GetName()
	}
	return fmt.Sprintf("/%s/%s/%d/messages/0x%x", c.topic, name, canMsg.Interface, canMsg.ID)
}

func (c *MQTTClient) getTopicFromSignal(sig canModels.CanSignalTimestamped) string {
	name := "unknown"
	if conn := c.resolver.ConnectionByID(sig.Interface); conn != nil {
		name = conn.GetName()
	}
	return fmt.Sprintf(
		"/%s/%s/%d/signals/%s/%s",
		c.topic,
		name,
		sig.Interface,
		sig.Message,
		sig.Signal,
	)
}

// buildTLSConfig constructs a tls.Config for the MQTT connection. When
// TLSCACertFile is set, the PEM file is loaded and added as the only trusted
// root CA (overriding system roots). When it is empty, RootCAs is left nil so
// Go falls back to the system certificate pool.
func buildTLSConfig(cfg canModels.MQTTConfig) (*tls.Config, error) {
	tlsCfg := &tls.Config{InsecureSkipVerify: false}

	if cfg.TLSCACertFile == "" {
		return tlsCfg, nil
	}

	pemBytes, err := os.ReadFile(cfg.TLSCACertFile)
	if err != nil {
		return nil, fmt.Errorf("reading CA cert file %q: %w", cfg.TLSCACertFile, err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemBytes) {
		return nil, fmt.Errorf("no valid PEM certificates found in %q", cfg.TLSCACertFile)
	}

	tlsCfg.RootCAs = pool
	return tlsCfg, nil
}
