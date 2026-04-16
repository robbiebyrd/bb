package connection

import (
	"context"
	"log/slog"
	"sync"

	"github.com/robbiebyrd/bb/internal/connection/simulate"
	"github.com/robbiebyrd/bb/internal/connection/socketcan"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

type CanConnectionManager struct {
	ctx            *context.Context
	connections    []canModels.CanConnection
	MessageChannel chan canModels.CanMessageTimestamped
	wg             *sync.WaitGroup
	l              *slog.Logger
	cfg            *canModels.Config
}

func NewConnectionManager(
	ctx *context.Context,
	cfg *canModels.Config,
	msgChan chan canModels.CanMessageTimestamped,
	logger *slog.Logger,
) canModels.ConnectionManager {
	wg := sync.WaitGroup{}
	return &CanConnectionManager{
		ctx:            ctx,
		MessageChannel: msgChan,
		wg:             &wg,
		l:              logger,
		cfg:            cfg,
	}
}

func (cm *CanConnectionManager) Connections() []canModels.CanConnection {
	return cm.connections
}

func (cm *CanConnectionManager) ConnectionByName(name string) canModels.CanConnection {
	for _, connection := range cm.connections {
		if connection.GetName() == name {
			return connection
		}
	}
	return nil
}

func (cm *CanConnectionManager) Add(conn canModels.CanConnection) {
	if conn == nil {
		return
	}
	cm.connections = append(cm.connections, conn)
}

func (cm *CanConnectionManager) Connect(options canModels.CanInterfaceOption) {
	switch options.Network {
	case "sim":
		cm.Add(simulate.NewSimulationCanClient(cm.ctx, cm.cfg, options.Name, cm.MessageChannel, cm.l, &options.Network, &options.URI, nil))
	default:
		cm.Add(socketcan.NewSocketCanConnection(cm.ctx, cm.cfg, options.Name, cm.MessageChannel, &options.Network, &options.URI))
	}
}

func (cm *CanConnectionManager) ConnectMultiple(options canModels.CanInterfaceOptions) {
	for _, option := range options {
		cm.Connect(option)
	}
}

func (cm *CanConnectionManager) DeleteConnection(name string) {
	for i, connection := range cm.connections {
		if connection.GetName() == name {
			connection.Close()
			cm.connections = append(cm.connections[:i], cm.connections[i+1:]...)
			return
		}
	}
}

func (cm *CanConnectionManager) OpenAll() error {
	for _, connection := range cm.connections {
		if err := connection.Open(); err != nil {
			return err
		}
	}
	return nil
}

func (cm *CanConnectionManager) CloseAll() error {
	for _, connection := range cm.connections {
		if err := connection.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (cm *CanConnectionManager) ReceiveAll() error {
	for _, connection := range cm.connections {
		connection.Receive(cm.wg)
	}
	cm.wg.Wait()
	return nil
}
