package socketcan

import (
	"context"
	"fmt"
	"net"

	"go.einride.tech/can"
	goSocketCan "go.einride.tech/can/pkg/socketcan"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

type ReceiverInterface interface {
	Receive() bool
	Frame() can.Frame
	Err() error
	Close() error
}

type SocketCanConnectionClient struct {
	ctx        context.Context
	name       string
	network    string
	uri        string
	channel    chan canModels.CanMessageTimestamped
	connection net.Conn
	receiver   ReceiverInterface
	opened     bool
	streaming  bool
	cfg        *canModels.Config
}

func NewSocketCanConnection(ctx context.Context, cfg *canModels.Config, name string, channel chan canModels.CanMessageTimestamped, network, uri *string) *SocketCanConnectionClient {
	if name == "" {
		panic(fmt.Errorf("connection name cannot be empty"))
	} else if channel == nil {
		panic(fmt.Errorf("message channel cannot be nil"))
	}

	if uri == nil {
		uri = &name
	}

	if network == nil {
		defaultNetwork := "can"
		network = &defaultNetwork
	}

	return &SocketCanConnectionClient{
		ctx:     ctx,
		name:    name,
		channel: channel,
		network: *network,
		uri:     *uri,
		cfg:     cfg,
	}
}

func (scc *SocketCanConnectionClient) GetURI() string {
	return scc.uri
}

func (scc *SocketCanConnectionClient) SetURI(uri string) {
	scc.uri = uri
}

func (scc *SocketCanConnectionClient) GetNetwork() string {
	return scc.network
}

func (scc *SocketCanConnectionClient) SetNetwork(network string) {
	scc.network = network
}

func (scc *SocketCanConnectionClient) GetName() string {
	return scc.name
}

func (scc *SocketCanConnectionClient) SetName(name string) {
	scc.name = name
}

func (scc *SocketCanConnectionClient) GetInterfaceName() string {
	return scc.GetName() + scc.cfg.CanInterfaceSeparator + scc.GetNetwork() + scc.cfg.CanInterfaceSeparator + scc.GetURI()
}

func (scc *SocketCanConnectionClient) GetConnection() net.Conn {
	return scc.connection
}

func (scc *SocketCanConnectionClient) SetConnection(conn net.Conn) {
	scc.connection = conn
}

func (scc *SocketCanConnectionClient) Open() error {
	conn, err := goSocketCan.DialContext(scc.ctx, scc.network, scc.name)
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", scc.network, err)
	}
	scc.connection = conn
	scc.opened = true
	return nil
}

func (scc *SocketCanConnectionClient) Close() error {
	if err := scc.connection.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", scc.name, err)
	}
	scc.opened = false
	return nil
}

func (scc *SocketCanConnectionClient) IsOpen() bool {
	return scc.opened
}

func (scc *SocketCanConnectionClient) Discontinue() error {
	if err := scc.receiver.Close(); err != nil {
		return err
	}
	scc.streaming = false
	return nil
}
