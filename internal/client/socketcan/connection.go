package socketcan

import (
	"context"
	"fmt"
	"net"

	"go.einride.tech/can"
	"go.einride.tech/can/pkg/socketcan"

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

type CanConnectionClient struct {
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

func NewSocketcanConnection(ctx *context.Context, name string, channel chan canModel.CanMessage, network, uri *string) *CanConnectionClient {
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

	return &CanConnectionClient{
		ctx:     ctx,
		Name:    name,
		Channel: channel,
		Network: *network,
		URI:     *uri,
	}
}

func (scc *CanConnectionClient) GetURI() string {
	return scc.URI
}

func (scc *CanConnectionClient) SetURI(uri string) {
	scc.URI = uri
}

func (scc *CanConnectionClient) GetNetwork() string {
	return scc.Network
}

func (scc *CanConnectionClient) SetNetwork(network string) {
	scc.Network = network
}

func (scc *CanConnectionClient) GetName() string {
	return scc.Name
}

func (scc *CanConnectionClient) SetName(name string) {
	scc.Name = name
}

func (scc *CanConnectionClient) GetConnection() net.Conn {
	return scc.Connection
}

func (scc *CanConnectionClient) SetConnection(conn net.Conn) {
	scc.Connection = conn
}

func (scc *CanConnectionClient) Open() error {
	if conn, err := socketcan.DialContext(*scc.ctx, scc.Network, scc.Name); err == nil {
		scc.Connection = conn
		scc.Opened = true
		return nil
	} else {
		panic(err)
	}
}

func (scc *CanConnectionClient) Close() error {
	err := scc.Connection.Close()
	if err != nil {
		panic(err)
	}
	scc.Opened = false
	return nil
}

func (scc *CanConnectionClient) IsOpen() bool {
	return scc.Opened
}

func (scc *CanConnectionClient) Discontinue() error {
	scc.Receiver.Close()
	scc.Streaming = false

	return nil
}
