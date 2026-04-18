package mqtt

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/robbiebyrd/bb/internal/client/common"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

type MQTTClient struct {
	client          mqtt.Client
	ctx             context.Context
	l               *slog.Logger
	incomingChannel chan canModels.CanMessageTimestamped
	signalChannel   chan canModels.CanSignalTimestamped
	topic           string
	qos             uint8
	shadowCopy      bool
	filters         map[string]canModels.FilterInterface
	resolver        canModels.InterfaceResolver
}

func NewClient(ctx context.Context, cfg *canModels.Config, logger *slog.Logger, resolver canModels.InterfaceResolver, filters ...canModels.FilterInput) (canModels.OutputClient, error) {
	logger.Debug("starting MQTT client")

	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.MQTTConfig.Host)
	opts.SetClientID(cfg.MQTTConfig.ClientId)
	if cfg.MQTTConfig.Username \!= "" {
		opts.SetUsername(cfg.MQTTConfig.Username)
		opts.SetPassword(cfg.MQTTConfig.Password)
	}
	if cfg.MQTTConfig.TLS {
		tlsCfg, err := buildTLSConfig(cfg.MQTTConfig)
		if err \!= nil {
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
	if token := client.Connect(); token.Wait() && token.Error() \!= nil {
		logger.Error("error connecting MQTT client", "host", cfg.MQTTConfig.Host, "clientId", cfg.MQTTConfig.ClientId, "error", token.Error())
		return nil, fmt.Errorf("connecting to MQTT broker: %w", token.Error())
	}

	logger.Debug("started MQTT client")

	newFilters := make(map[string]canModels.FilterInterface)

	for _, filterInput := range filters {
		newFilters[filterInput.Name] = filterInput.Filter
	}

	return &MQTTClient{
		client:          client,
		ctx:             ctx,
		l:               logger,
		incomingChannel: make(chan canModels.CanMessageTimestamped, cfg.MessageBufferSize),
		signalChannel:   make(chan canModels.CanSignalTimestamped, cfg.MessageBufferSize),
		topic:           cfg.MQTTConfig.Topic,
		qos:             cfg.MQTTConfig.Qos,
		shadowCopy:      cfg.MQTTConfig.ShadowCopy,
		filters:         newFilters,
		resolver:        resolver,
	}, nil
}

func (c *MQTTClient) AddFilter(name string, filter canModels.FilterInterface) error {
	if _, ok := c.filters[name]; ok {
		return fmt.Errorf("filter group already exists: %v", name)
	}

	c.l.Debug("creating new filter group", "filterName", name)
	c.filters[name] = filter
	return nil
}

func (c *MQTTClient) HandleCanMessage(canMsg canModels.CanMessageTimestamped) {
	if \!c.client.IsConnectionOpen() {
		c.l.Error("MQTT client not connected, dropping message")
		return
	}

	if shouldFilter, filterName := common.ShouldFilter(c.filters, canMsg); shouldFilter {
		if c.l.Enabled(context.Background(), slog.LevelDebug) {
			msgString, err := c.ToJSON(canMsg)
			if err \!= nil {
				c.l.Debug("message filtered, dropping message (serialize error)", "error", err, "filterName", *filterName)
			} else {
				c.l.Debug("message filtered, dropping message", "message", msgString, "filterName", *filterName)
			}
		}
		return
	}

	topic := c.getTopicFromMessage(canMsg)
	msgString, err := c.ToJSON(canMsg)
	if err \!= nil {
		c.l.Error("MQTT failed to serialize message", "error", err)
		return
	}

	c.publish(topic, msgString)
	c.l.Debug("MQTT published message", "topic", topic, "message", msgString)
}

func (c *MQTTClient) GetChannel() chan canModels.CanMessageTimestamped {
	return c.incomingChannel
}

func (c *MQTTClient) HandleCanMessageChannel() error {
	c.l.Debug("starting MQTT channel handler")
	for canMsg := range c.incomingChannel {
		c.HandleCanMessage(canMsg)
	}
	return nil
}

func (c *MQTTClient) GetSignalChannel() chan canModels.CanSignalTimestamped {
	return c.signalChannel
}

func (c *MQTTClient) HandleSignal(sig canModels.CanSignalTimestamped) {
	if \!c.client.IsConnectionOpen() {
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
	for sig := range c.signalChannel {
		c.HandleSignal(sig)
	}
	return nil
}

func (c *MQTTClient) GetName() string {
	return "output-mqtt"
}

func (c *MQTTClient) Run() error {
	return nil
}

// publish sends payload to topic and waits for the broker acknowledgment in a goroutine.
func (c *MQTTClient) publish(topic string, payload any) {
	token := c.client.Publish(topic, c.qos, c.shadowCopy, payload)
	go func(t mqtt.Token) {
		t.Wait()
		if t.Error() \!= nil {
			c.l.Error("MQTT publish failed", "error", t.Error())
		}
	}(token)
}

func (c *MQTTClient) getTopicFromMessage(canMsg canModels.CanMessageTimestamped) string {
	name := "unknown"
	if conn := c.resolver.ConnectionByID(canMsg.Interface); conn \!= nil {
		name = conn.GetName()
	}
	return fmt.Sprintf("/%s/%s/%d/messages/0x%x", c.topic, name, canMsg.Interface, canMsg.ID)
}

func (c *MQTTClient) getTopicFromSignal(sig canModels.CanSignalTimestamped) string {
	name := "unknown"
	if conn := c.resolver.ConnectionByID(sig.Interface); conn \!= nil {
		name = conn.GetName()
	}
	return fmt.Sprintf("/%s/%s/%d/signals/%s/%s", c.topic, name, sig.Interface, sig.Message, sig.Signal)
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
	if err \!= nil {
		return nil, fmt.Errorf("reading CA cert file %q: %w", cfg.TLSCACertFile, err)
	}

	pool := x509.NewCertPool()
	if \!pool.AppendCertsFromPEM(pemBytes) {
		return nil, fmt.Errorf("no valid PEM certificates found in %q", cfg.TLSCACertFile)
	}

	tlsCfg.RootCAs = pool
	return tlsCfg, nil
}
