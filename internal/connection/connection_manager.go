package connection

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/robbiebyrd/cantou/internal/connection/playback"
	"github.com/robbiebyrd/cantou/internal/connection/simulate"
	"github.com/robbiebyrd/cantou/internal/connection/slcan"
	"github.com/robbiebyrd/cantou/internal/connection/socketcan"
	canModels "github.com/robbiebyrd/cantou/internal/models"
)

type CanConnectionManager struct {
	ctx            context.Context
	mu             sync.RWMutex
	connections    map[int]canModels.CanConnection
	nextID         int
	MessageChannel chan canModels.CanMessageTimestamped
	wg             *sync.WaitGroup
	l              *slog.Logger
	cfg            *canModels.Config
}

func NewConnectionManager(
	ctx context.Context,
	cfg *canModels.Config,
	msgChan chan canModels.CanMessageTimestamped,
	logger *slog.Logger,
) canModels.ConnectionManager {
	wg := sync.WaitGroup{}
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
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	conns := make([]canModels.CanConnection, 0, len(cm.connections))
	for _, conn := range cm.connections {
		conns = append(conns, conn)
	}
	return conns
}

func (cm *CanConnectionManager) ConnectionByName(name string) canModels.CanConnection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	for _, connection := range cm.connections {
		if connection.GetName() == name {
			return connection
		}
	}
	return nil
}

func (cm *CanConnectionManager) ConnectionByID(id int) canModels.CanConnection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
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
	cm.mu.Lock()
	defer cm.mu.Unlock()
	id := cm.nextID
	conn.SetID(id)
	cm.connections[id] = conn
	cm.nextID++
	return id
}

func (cm *CanConnectionManager) Connect(options canModels.CanInterfaceOption) {
	switch options.Network {
	case "sim":
		cm.Add(
			simulate.NewSimulationCanClient(
				cm.ctx,
				cm.cfg,
				options.Name,
				cm.MessageChannel,
				cm.l,
				&options.Network,
				&options.URI,
				nil,
			),
		)
	case "slcan":
		dbcStr := strings.Join(options.DBCFiles, ",")
		cm.Add(
			slcan.NewSLCanConnection(
				cm.ctx,
				cm.cfg,
				options.Name,
				cm.MessageChannel,
				&options.Network,
				&options.URI,
				&dbcStr,
			),
		)
	case "playback":
		dbcStr := strings.Join(options.DBCFiles, ",")
		cm.Add(
			playback.NewPlaybackCanClient(
				cm.ctx,
				cm.cfg,
				options.Name,
				cm.MessageChannel,
				options.URI,
				options.Loop,
				cm.l,
				&options.Network,
				nil,
				&dbcStr,
			),
		)
	default:
		dbcStr := strings.Join(options.DBCFiles, ",")
		cm.Add(
			socketcan.NewSocketCanConnection(
				cm.ctx,
				cm.cfg,
				options.Name,
				cm.MessageChannel,
				&options.Network,
				&options.URI,
				&dbcStr,
			),
		)
	}
}

func (cm *CanConnectionManager) ConnectMultiple(options canModels.CanInterfaceOptions) {
	for _, option := range options {
		cm.Connect(option)
	}
}

func (cm *CanConnectionManager) DeleteConnection(name string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	for id, connection := range cm.connections {
		if connection.GetName() == name {
			connection.Close()
			delete(cm.connections, id)
			return
		}
	}
}

func (cm *CanConnectionManager) OpenAll() error {
	for _, connection := range cm.snapshot() {
		if err := connection.Open(); err != nil {
			return err
		}
	}
	return nil
}

func (cm *CanConnectionManager) CloseAll() error {
	for _, connection := range cm.snapshot() {
		if err := connection.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (cm *CanConnectionManager) ReceiveAll() error {
	for _, connection := range cm.snapshot() {
		connection.Receive(cm.wg)
	}
	cm.wg.Wait()
	return nil
}

func (cm *CanConnectionManager) snapshot() []canModels.CanConnection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	conns := make([]canModels.CanConnection, 0, len(cm.connections))
	for _, conn := range cm.connections {
		conns = append(conns, conn)
	}
	return conns
}
