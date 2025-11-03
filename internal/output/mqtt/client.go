package mqtt

import (
	"context"
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
	incomingChannel chan canModels.CanMessage
	topic           string
}

func NewClient(ctx *context.Context, cfg canModels.Config, logger *slog.Logger) canModels.OutputClient {
	logger.Debug("starting MQTT client")
	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.MQTTConfig.Host)
	opts.SetClientID(cfg.MQTTConfig.ClientId)
	opts.OnConnect = func(client mqtt.Client) {
		fmt.Println("connected to MQTT Broker")
	}
	opts.OnConnectionLost = func(client mqtt.Client, err error) {
		fmt.Printf("connection to MQTT broker lost: %v", err)
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	logger.Debug("started MQTT client")

	return &MQTTClient{
		client:          client,
		ctx:             *ctx,
		l:               logger,
		incomingChannel: make(chan canModels.CanMessage, cfg.MessageBufferSize),
		topic:           cfg.MQTTConfig.Topic,
	}
}

func (c *MQTTClient) getTopic(canMsg canModels.CanMessage) string {
	interfaceParts := strings.Split(canMsg.Interface, ":>")
	return "/" + c.topic + "/" + interfaceParts[len(interfaceParts)-1] + "/" + fmt.Sprintf("%X", canMsg.ID)
}

func (c *MQTTClient) Handle(canMsg canModels.CanMessage) {
	if c.client.IsConnectionOpen() {
		fmt.Printf("publishing MQTT message %v\n", c.getTopic(canMsg))
		c.l.Debug(fmt.Sprintf("MQTT processing message to topic %v", c.getTopic(canMsg)))
		token := c.client.Publish(c.getTopic(canMsg), 1, true, canMsg)
		token.Wait()
	} else {
		c.l.Warn("MQTT client not connected, dropping message")
	}
}

func (c *MQTTClient) GetChannel() chan canModels.CanMessage {
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
