package app

import (
	"context"
	"log/slog"

	"golang.org/x/sync/errgroup"

	"github.com/robbiebyrd/bb/internal/client/broadcast"
	cm "github.com/robbiebyrd/bb/internal/connection"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

type AppData struct {
	config          *canModels.Config
	wgClients       *errgroup.Group
	broadcastClient *broadcast.BroadcastClient
	connections     canModels.ConnectionManager
	logger          *slog.Logger
	logLevel        *slog.LevelVar
	canMsgChannel   chan canModels.CanMessageTimestamped
	ctx             context.Context
}

// NewApp creates the application with the given config, logger, and log level.
// Config loading, log level setup, and token resolution must be done by the caller
// before invoking NewApp.
func NewApp(cfg *canModels.Config, logger *slog.Logger, logLevel *slog.LevelVar) canModels.AppInterface {
	logger.Debug("creating process context and wait group")
	wgClients, ctx := errgroup.WithContext(context.Background())

	logger.Debug("creating channel for incoming CAN messages")
	canMsgChannel := make(chan canModels.CanMessageTimestamped, cfg.MessageBufferSize)

	logger.Debug("creating broadcast client")
	broadcastClient := broadcast.NewBroadcastClient(ctx, canMsgChannel, logger)

	logger.Debug("creating connection manager")
	connections := cm.NewConnectionManager(ctx, cfg, canMsgChannel, logger)

	logger.Info("creating can interfaces from config", "count", len(cfg.CanInterfaces))
	connections.ConnectMultiple(cfg.CanInterfaces)

	logger.Info("application started")

	return &AppData{
		config:          cfg,
		wgClients:       wgClients,
		broadcastClient: broadcastClient,
		connections:     connections,
		logger:          logger,
		logLevel:        logLevel,
		canMsgChannel:   canMsgChannel,
		ctx:             ctx,
	}
}

func (b *AppData) AddOutput(c canModels.OutputClient) {
	b.logger.Debug("running output client", "name", c.GetName())
	b.wgClients.Go(c.Run)

	b.logger.Debug("starting internal handler for output client", "name", c.GetName())
	b.wgClients.Go(c.HandleChannel)

	b.logger.Debug("adding broadcast listener for output client", "name", c.GetName())
	b.broadcastClient.Add(broadcast.BroadcastClientListener{Name: c.GetName(), Channel: c.GetChannel()})
}

func (b *AppData) AddOutputs(cs []canModels.OutputClient) {
	b.logger.Info("creating output clients", "count", len(cs))
	for _, c := range cs {
		b.AddOutput(c)
	}
	b.logger.Debug("created output clients", "count", len(cs))
}

func (b *AppData) Run() error {
	b.logger.Info("starting broadcasts")
	b.wgClients.Go(b.broadcastClient.Broadcast)

	b.logger.Info("receiving data on connections")
	b.wgClients.Go(b.connections.ReceiveAll)

	b.logger.Info("services running, waiting for messages")
	return b.wgClients.Wait()
}

func (b *AppData) GetConnections() canModels.ConnectionManager {
	return b.connections
}

func (b *AppData) GetContext() context.Context {
	return b.ctx
}

func (b *AppData) GetConfig() *canModels.Config {
	return b.config
}

func (b *AppData) GetLogger() *slog.Logger {
	return b.logger
}

func (b *AppData) GetLogLevel() *slog.LevelVar {
	return b.logLevel
}

func (b *AppData) SetLogLevel(logLevel slog.Level) {
	b.logLevel.Set(logLevel)
}
