package simulate

import (
	"context"
	"fmt"
	"net"

	canModel "github.com/robbiebyrd/bb/internal/models/can"
)

type SimulationCanClient struct {
	ctx        *context.Context
	Name       string
	Network    string
	URI        string
	Channel    chan canModel.CanMessage
	Connection net.Conn
	Receiver   canModel.ReceiverInterface
	Opened     bool
	Streaming  bool
}

func NewSimulationCanClient(ctx *context.Context, name string, channel chan canModel.CanMessage, network, uri *string) *SimulationCanClient {
	if name == "" {
		panic(fmt.Errorf("connection name cannot be empty"))
	} else if channel == nil {
		panic(fmt.Errorf("message channel cannot be nil"))
	}

	if uri == nil {
		uri = &name
	}

	if network == nil {
		defaultNetwork := "sim"
		network = &defaultNetwork
	}

	return &SimulationCanClient{
		ctx:     ctx,
		Name:    name,
		Channel: channel,
		Network: *network,
		URI:     *uri,
	}
}

func (scc *SimulationCanClient) GetURI() string {
	return scc.URI
}

func (scc *SimulationCanClient) SetURI(uri string) {
	scc.URI = uri
}

func (scc *SimulationCanClient) GetNetwork() string {
	return scc.Network
}

func (scc *SimulationCanClient) GetInterfaceName() string {
	return scc.GetName() + ":>" + scc.GetNetwork() + ":>" + scc.GetURI()
}

func (scc *SimulationCanClient) SetNetwork(network string) {
	scc.Network = network
}

func (scc *SimulationCanClient) GetName() string {
	return scc.Name
}

func (scc *SimulationCanClient) SetName(name string) {
	scc.Name = name
}

func (scc *SimulationCanClient) GetConnection() net.Conn {
	return scc.Connection
}

func (scc *SimulationCanClient) SetConnection(conn net.Conn) {
	scc.Connection = conn
}

func (scc *SimulationCanClient) Open() error {
	scc.Opened = true
	return nil
}

func (scc *SimulationCanClient) Close() error {
	scc.Opened = false
	return nil
}

func (scc *SimulationCanClient) IsOpen() bool {
	return scc.Opened
}

func (scc *SimulationCanClient) Discontinue() error {
	scc.Streaming = false
	return nil
}
