package mqtt

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

type MQTTClient struct {
	client          mqtt.Client
	ctx             context.Context
	l               *slog.Logger
	incomingChannel chan canModels.CanMessageTimestamped
	signalChannel   chan canModels.CanSignalTimestamped
	topic           string
	cfg             *canModels.Config
	filters         map[string]canModels.FilterInterface
	resolver        canModels.InterfaceResolver
}

func NewClient(ctx context.Context, cfg *canModels.Config, logger *slog.Logger, resolver canModels.InterfaceResolver, filters ...canModels.FilterInput) (canModels.OutputClient, error) {
	logger.Debug("starting MQTT client")

	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.MQTTConfig.Host)
	opts.SetClientID(cfg.MQTTConfig.ClientId)
	if cfg.MQTTConfig.Username != "" {
		opts.SetUsername(cfg.MQTTConfig.Username)
		opts.SetPassword(cfg.MQTTConfig.Password)
	}
	if cfg.MQTTConfig.TLS {
		tlsCfg := &tls.Config{
			InsecureSkipVerify: false,
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
		cfg:             cfg,
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
	if !c.client.IsConnectionOpen() {
		c.l.Error("MQTT client not connected, dropping message")
		return
	}

	if shouldFilter, filterName := c.shouldFilterMessage(canMsg); shouldFilter {
		msgString, _ := c.ToJSON(canMsg)
		c.l.Debug("message filtered, dropping message", "message", msgString, "filterName", *filterName)
		return
	}

	topic := c.getTopicFromMessage(canMsg)
	msgString, err := c.ToJSON(canMsg)
	if err != nil {
		c.l.Error("MQTT failed to serialize message", "error", err)
		return
	}

	token := c.client.Publish(topic, c.cfg.MQTTConfig.Qos, c.cfg.MQTTConfig.ShadowCopy, msgString)

	go func(t mqtt.Token, msg string) {
		t.Wait()
		if t.Error() != nil {
			c.l.Error("MQTT publish failed", "error", t.Error())
		}
	}(token, msgString)

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
	if !c.client.IsConnectionOpen() {
		c.l.Error("MQTT client not connected, dropping signal")
		return
	}

	topic := c.getTopicFromSignal(sig)
	payload := signalPayload(sig)

	token := c.client.Publish(topic, c.cfg.MQTTConfig.Qos, c.cfg.MQTTConfig.ShadowCopy, payload)

	go func(t mqtt.Token) {
		t.Wait()
		if t.Error() != nil {
			c.l.Error("MQTT publish signal failed", "error", t.Error())
		}
	}(token)

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

func (c *MQTTClient) getTopicFromMessage(canMsg canModels.CanMessageTimestamped) string {
	name := "unknown"
	if conn := c.resolver.ConnectionByID(canMsg.Interface); conn != nil {
		name = conn.GetName()
	}
	return fmt.Sprintf("/%s/%s/%d/messages/0x%X", c.topic, name, canMsg.Interface, canMsg.ID)
}

func (c *MQTTClient) getTopicFromSignal(sig canModels.CanSignalTimestamped) string {
	name := "unknown"
	if conn := c.resolver.ConnectionByID(sig.Interface); conn != nil {
		name = conn.GetName()
	}
	return fmt.Sprintf("/%s/%s/%d/signals/%s/%s", c.topic, name, sig.Interface, sig.Message, sig.Signal)
}

func (c *MQTTClient) shouldFilterMessage(canMsg canModels.CanMessageTimestamped) (bool, *string) {
	for name, filter := range c.filters {
		if filter.Filter(canMsg) {
			c.l.Debug("message filtered, skipping", "filterName", name)
			return true, &name
		}
	}
	return false, nil
}
