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
	connections    map[int]canModels.CanConnection
	nextID         int
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
	wg.Add(1)
	return &CanConnectionManager{
		ctx:            ctx,
		connections:    make(map[int]canModels.CanConnection),
		nextID:         0,
		MessageChannel: msgChan,
		wg:             &wg,
		l:              logger,
		cfg:            cfg,
	}
}

func (cm *CanConnectionManager) Connections() []canModels.CanConnection {
	conns := make([]canModels.CanConnection, 0, len(cm.connections))
	for _, conn := range cm.connections {
		conns = append(conns, conn)
	}
	return conns
}

func (cm *CanConnectionManager) ConnectionByName(name string) canModels.CanConnection {
	for _, connection := range cm.connections {
		if connection.GetName() == name {
			return connection
		}
	}
	return nil
}

func (cm *CanConnectionManager) ConnectionByID(id int) canModels.CanConnection {
	conn, ok := cm.connections[id]
	if !ok {
		return nil
	}
	return conn
}

func (cm *CanConnectionManager) Add(conn canModels.CanConnection) int {
	if conn == nil {
		return -1
	}
	id := cm.nextID
	conn.SetID(id)
	cm.connections[id] = conn
	cm.nextID++
	return id
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
	for id, connection := range cm.connections {
		if connection.GetName() == name {
			connection.Close()
			delete(cm.connections, id)
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
