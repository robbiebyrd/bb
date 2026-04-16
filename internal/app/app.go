package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"

	"golang.org/x/sync/errgroup"

	"github.com/robbiebyrd/bb/internal/client/broadcast"
	"github.com/robbiebyrd/bb/internal/client/logging"
	"github.com/robbiebyrd/bb/internal/config"
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

func NewApp() canModels.AppInterface {
	// Set the log level variable, so we can use it at startup in Info mode, but allow
	// changing it later based on config.
	lvl := new(slog.LevelVar)
	lvl.Set(slog.LevelInfo)

	// Create the logger with the initial log level.
	l := logging.NewJSONLogger(lvl)
	l.Info("starting application")

	l.Debug("loading config")
	cfg, cfgJson := config.Load(l)
	l.Debug("loaded config", "config", cfgJson)

	l.Info("setting log level from config", "level", cfg.LogLevel)
	switch cfg.LogLevel {
	case "debug", "DEBUG":
		lvl.Set(slog.LevelDebug)
	case "error", "ERROR":
		lvl.Set(slog.LevelError)
	case "warn", "WARN":
		lvl.Set(slog.LevelWarn)
	}

	if cfg.InfluxDB.Token == "" && cfg.InfluxDB.TokenFile != "" {
		jsonFile, err := os.Open(cfg.InfluxDB.TokenFile)
		if err != nil {
			l.Error("could not open influxdb token file", "path", cfg.InfluxDB.TokenFile, "error", err)
			panic(err)
		}
		defer jsonFile.Close()

		creds := struct {
			Token string `json:"token"`
		}{}

		err = json.NewDecoder(jsonFile).Decode(&creds)
		if err != nil {
			l.Error("could not decode influxdb token json file", "error", err)
			panic(err)
		}

		cfg.InfluxDB.Token = creds.Token
	}

	l.Debug("creating process context and wait group")
	wgClients, ctx := errgroup.WithContext(context.Background())

	l.Debug("creating channel for incoming CAN messages")
	canMsgChannel := make(chan canModels.CanMessageTimestamped, cfg.MessageBufferSize)

	l.Debug("creating broadcast client")
	broadcastClient := broadcast.NewBroadcastClient(ctx, canMsgChannel, l)

	l.Debug("creating connection manager")
	connections := cm.NewConnectionManager(ctx, &cfg, canMsgChannel, l)

	l.Info("creating can interfaces from config", "count", len(cfg.CanInterfaces))
	connections.ConnectMultiple(cfg.CanInterfaces)

	l.Info("application started")

	return &AppData{
		config:          &cfg,
		wgClients:       wgClients,
		broadcastClient: broadcastClient,
		connections:     connections,
		logger:          l,
		logLevel:        lvl,
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
