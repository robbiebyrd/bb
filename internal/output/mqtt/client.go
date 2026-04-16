package mqtt

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

type MQTTClient struct {
	client          mqtt.Client
	ctx             context.Context
	l               *slog.Logger
	incomingChannel chan canModels.CanMessageTimestamped
	topic           string
	cfg             *canModels.Config
	filters         map[string]canModels.FilterInterface
}

func NewClient(ctx context.Context, cfg *canModels.Config, logger *slog.Logger, filters ...canModels.FilterInput) (canModels.OutputClient, error) {
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

	logger.Debug("connecting MQTT client", "host", cfg.MQTTConfig.Host, "clientId", cfg.MQTTConfig.ClientId)
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
		topic:           cfg.MQTTConfig.Topic,
		cfg:             cfg,
		filters:         newFilters,
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

func (c *MQTTClient) Handle(canMsg canModels.CanMessageTimestamped) {
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

func (c *MQTTClient) HandleChannel() error {
	c.l.Debug("starting MQTT channel handler")
	for canMsg := range c.incomingChannel {
		c.Handle(canMsg)
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
	interfaceParts := strings.Split(canMsg.Interface, c.cfg.CanInterfaceSeparator)
	return "/" + c.topic + "/" + interfaceParts[len(interfaceParts)-1] + "/0x" + fmt.Sprintf("%X", canMsg.ID)
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
