package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"golang.org/x/sync/errgroup"

	cm "github.com/robbiebyrd/bb/internal/client"
	"github.com/robbiebyrd/bb/internal/config"
	canModels "github.com/robbiebyrd/bb/internal/models"
	"github.com/robbiebyrd/bb/internal/repo/influxdb"
)

func main() {
	var wgClients errgroup.Group
	ctx := context.Background()

	cfg, cfgJson := config.Load()

	var programLevel = new(slog.LevelVar)
	l := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: programLevel}))
	programLevel.Set(slog.LevelInfo)

	l.Info("starting application")
	l.Debug(fmt.Sprintf("loaded config: %v", cfgJson))

	l.Debug("creating channel for incoming CAN messages")
	canMsgChannel := make(chan canModels.CanMessage, cfg.MessageBufferSize)

	l.Info("creating connection manager")
	connections := cm.NewConnectionManager(&ctx, canMsgChannel, l)

	l.Info("creating database clients")
	l.Info("creating influxdb3 client")
	dbClient := influxdb.NewClient(&ctx, cfg, l)

	l.Info(fmt.Sprintf("creating %v can interfaces", len(cfg.CanInterfaces)))
	connections.ConnectMultiple(cfg.CanInterfaces)

	l.Info("starting processes")

	l.Info("running clients")
	wgClients.Go(dbClient.Run)

	l.Info("receiving data on connections")
	wgClients.Go(connections.ReceiveAll)

	l.Info("handling messages")
	dbClient.HandleChannel(canMsgChannel)
}
