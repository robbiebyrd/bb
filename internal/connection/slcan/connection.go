package slcan

import (
	"context"
	"fmt"
	"net"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

// defaultPortBaudrate is the standard serial baudrate for SLCAN devices such as CANable.
const defaultPortBaudrate = 115200

// defaultCANRate is the CAN bus speed in kbit/s used when none is specified.
const defaultCANRate = 500.0

type SLCanConnectionClient struct {
	ctx         context.Context
	id          int
	name        string
	network     string
	uri         string
	channel     chan canModels.CanMessageTimestamped
	adapter     any // gocan.Adapter on linux; unused on other platforms
	opened      bool
	streaming   bool
	quit        chan struct{}
	cfg         *canModels.Config
	dbcFilePath *string
}

func NewSLCanConnection(
	ctx context.Context,
	cfg *canModels.Config,
	name string,
	channel chan canModels.CanMessageTimestamped,
	network, uri, dbcFilePath *string,
) *SLCanConnectionClient {
	if name == "" {
		panic(fmt.Errorf("connection name cannot be empty"))
	} else if channel == nil {
		panic(fmt.Errorf("message channel cannot be nil"))
	}

	if uri == nil {
		uri = &name
	}

	if network == nil {
		defaultNetwork := "slcan"
		network = &defaultNetwork
	}

	return &SLCanConnectionClient{
		ctx:         ctx,
		name:        name,
		channel:     channel,
		network:     *network,
		uri:         *uri,
		cfg:         cfg,
		dbcFilePath: dbcFilePath,
	}
}

func (scc *SLCanConnectionClient) GetID() int {
	return scc.id
}

func (scc *SLCanConnectionClient) SetID(id int) {
	scc.id = id
}

func (scc *SLCanConnectionClient) GetURI() string {
	return scc.uri
}

func (scc *SLCanConnectionClient) SetURI(uri string) {
	scc.uri = uri
}

func (scc *SLCanConnectionClient) GetDBCFilePath() *string {
	return scc.dbcFilePath
}

func (scc *SLCanConnectionClient) SetDBCFilePath(filePath *string) {
	scc.dbcFilePath = filePath
}

func (scc *SLCanConnectionClient) GetNetwork() string {
	return scc.network
}

func (scc *SLCanConnectionClient) SetNetwork(network string) {
	scc.network = network
}

func (scc *SLCanConnectionClient) GetName() string {
	return scc.name
}

func (scc *SLCanConnectionClient) SetName(name string) {
	scc.name = name
}

func (scc *SLCanConnectionClient) GetInterfaceName() string {
	return scc.name + scc.cfg.CanInterfaceSeparator + scc.network + scc.cfg.CanInterfaceSeparator + scc.uri
}

// GetConnection returns nil — SLCAN uses a serial adapter, not a net.Conn.
func (scc *SLCanConnectionClient) GetConnection() net.Conn {
	return nil
}

// SetConnection is a no-op — SLCAN does not use a net.Conn.
func (scc *SLCanConnectionClient) SetConnection(_ net.Conn) {}

func (scc *SLCanConnectionClient) IsOpen() bool {
	return scc.opened
}

func (scc *SLCanConnectionClient) Discontinue() error {
	if scc.quit != nil {
		close(scc.quit)
	}
	scc.streaming = false
	return nil
}
