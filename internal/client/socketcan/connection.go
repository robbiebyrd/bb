package socketcan

import (
	"context"
	"fmt"
	"net"

	"go.einride.tech/can"
	goCan "go.einride.tech/can/pkg/socketcan"

	canModel "github.com/robbiebyrd/bb/internal/models/can"
)

type IFaceTest interface {
	String() string
}
type ReceiverInterface interface {
	Receive() bool
	Frame() can.Frame
	Err() error
	Close() error
}

type SocketCanConnectionClient struct {
	ctx        *context.Context
	Name       string
	Network    string
	URI        string
	Channel    chan canModel.CanMessage
	Connection net.Conn
	Receiver   ReceiverInterface
	Opened     bool
	Streaming  bool
}

func NewSocketCanConnection(ctx *context.Context, name string, channel chan canModel.CanMessage, network, uri *string) *SocketCanConnectionClient {
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
		Name:    name,
		Channel: channel,
		Network: *network,
		URI:     *uri,
	}
}

func (scc *SocketCanConnectionClient) GetURI() string {
	return scc.URI
}

func (scc *SocketCanConnectionClient) SetURI(uri string) {
	scc.URI = uri
}

func (scc *SocketCanConnectionClient) GetNetwork() string {
	return scc.Network
}

func (scc *SocketCanConnectionClient) SetNetwork(network string) {
	scc.Network = network
}

func (scc *SocketCanConnectionClient) GetName() string {
	return scc.Name
}

func (scc *SocketCanConnectionClient) SetName(name string) {
	scc.Name = name
}

func (scc *SocketCanConnectionClient) GetInterfaceName() string {
	return scc.GetName() + ":>" + scc.GetNetwork() + ":>" + scc.GetURI()
}

func (scc *SocketCanConnectionClient) GetConnection() net.Conn {
	return scc.Connection
}

func (scc *SocketCanConnectionClient) SetConnection(conn net.Conn) {
	scc.Connection = conn
}

func (scc *SocketCanConnectionClient) Open() error {
	if conn, err := goCan.DialContext(*scc.ctx, scc.Network, scc.Name); err == nil {
		scc.Connection = conn
		scc.Opened = true
		return nil
	} else {
		panic(err)
	}
}

func (scc *SocketCanConnectionClient) Close() error {
	err := scc.Connection.Close()
	if err != nil {
		panic(err)
	}
	scc.Opened = false
	return nil
}

func (scc *SocketCanConnectionClient) IsOpen() bool {
	return scc.Opened
}

func (scc *SocketCanConnectionClient) Discontinue() error {
	scc.Receiver.Close()
	scc.Streaming = false

	return nil
}
