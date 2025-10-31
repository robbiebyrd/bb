package app

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/sync/errgroup"

	"github.com/robbiebyrd/bb/internal/client/broadcast"
	"github.com/robbiebyrd/bb/internal/client/logging"
	"github.com/robbiebyrd/bb/internal/config"
	cm "github.com/robbiebyrd/bb/internal/connection"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

type BerthaApp struct {
	Cfg             canModels.Config
	Log             *slog.Logger
	WgClients       *errgroup.Group
	BroadcastClient *broadcast.BroadcastClient
	Clients         []canModels.OutputClient
	Connections     canModels.ConnectionManager
	canMsgChannel   chan canModels.CanMessage
	Ctx             context.Context
}

func NewBerthaApp() *BerthaApp {

	cfg, cfgJson := config.Load()
	l := logging.NewJSONLogger(cfg.LogLevel)

	l.Info("starting application")
	l.Debug(fmt.Sprintf("loaded config: %v", cfgJson))

	l.Debug("creating process context")
	ctx := context.Background()

	l.Debug("creating channel for incoming CAN messages")
	canMsgChannel := make(chan canModels.CanMessage, cfg.MessageBufferSize)

	l.Debug("creating process wait group")
	var wgClients errgroup.Group

	l.Debug("creating broadcast client")
	broadcastClient := broadcast.NewBroadcastClient(&ctx, canMsgChannel)

	l.Debug("creating connection manager")
	connections := cm.NewConnectionManager(&ctx, canMsgChannel, l)

	l.Info(fmt.Sprintf("creating %v can interfaces from config", len(cfg.CanInterfaces)))
	connections.ConnectMultiple(cfg.CanInterfaces)

	l.Info("application started")

	return &BerthaApp{
		Cfg:             cfg,
		Log:             l,
		WgClients:       &wgClients,
		BroadcastClient: broadcastClient,
		Clients:         []canModels.OutputClient{},
		Connections:     connections,
		canMsgChannel:   canMsgChannel,
		Ctx:             ctx,
	}
}

func (b *BerthaApp) AddOutput(c canModels.OutputClient) {
	b.Log.Debug(fmt.Sprintf("running database client %v", c.GetName()))
	b.WgClients.Go(c.Run)

	b.Log.Debug(fmt.Sprintf("starting internal handler for database client %v", c.GetName()))
	b.WgClients.Go(c.HandleChannel)

	b.Log.Debug(fmt.Sprintf("adding broadcast listener for database client %v", c.GetName()))
	b.BroadcastClient.Add(broadcast.BroadcastClientListener{Name: c.GetName(), Channel: c.GetChannel()})
}

func (b *BerthaApp) RemoveOutput(name string) {
	for i, c := range b.Clients {
		if name == c.GetName() {
			b.Clients = append(b.Clients[:i], b.Clients[i+1:]...)
		}
	}
}

func (b *BerthaApp) AddOutputs(cs []canModels.OutputClient) {
	b.Log.Info(fmt.Sprintf("creating %v database clients", len(cs)))
	for _, c := range cs {
		b.AddOutput(c)
	}
	b.Log.Debug(fmt.Sprintf("created %v database clients", len(cs)))
}

func (b *BerthaApp) Run() {
	b.Log.Info("starting broadcasts")
	b.WgClients.Go(b.BroadcastClient.Broadcast)

	b.Log.Info("receiving data on connections")
	b.WgClients.Go(b.Connections.ReceiveAll)

	b.Log.Info("services running, waiting for messages")
	b.WgClients.Wait()
}
