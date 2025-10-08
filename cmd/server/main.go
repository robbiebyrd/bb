package main

import (
	"context"
	"os"

	cm "github.com/robbiebyrd/bb/internal/client"
	"github.com/robbiebyrd/bb/internal/client/logging"
	"github.com/robbiebyrd/bb/internal/config"
	"github.com/robbiebyrd/bb/internal/models/can"
	"github.com/robbiebyrd/bb/internal/repo/influxdb"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()

	MessageChannel := make(chan can.CanMessage, cfg.MessageBufferSize)

	jlog := logging.NewJSONLogger(os.Stdout)

	cm := cm.NewConnectionManager(&ctx, MessageChannel, jlog)

	dbClient, err := influxdb.NewClient(&ctx, cfg)
	if err != nil {
		panic(err)
	}

	if len(cfg.CanInterfaces) == 0 {
		panic("no CAN interfaces configured")
	}

	cm.ConnectMultiple(cfg.CanInterfaces)
	cm.ReceiveAll()

	i := 0

	go dbClient.Run()

	for msg := range MessageChannel {
		i++
		dbClient.Handle(msg)
	}
}
