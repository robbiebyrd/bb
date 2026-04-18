package simulate

import (
	"context"
	"fmt"
	"log/slog"
	mathRand "math/rand"
	"net"
	"sync"
	"time"

	commonUtils "github.com/robbiebyrd/bb/internal/client/common"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

type SimulationCanClient struct {
	ctx         context.Context
	id          int
	name        string
	network     string
	uri         string
	channel     chan canModels.CanMessageTimestamped
	connection  net.Conn
	opened      bool
	streaming   bool
	l           *slog.Logger
	rate        int // nanoseconds
	count       int
	cfg         *canModels.Config
	dbcFilePath *string
}

const CAN_MESSAGE_MAX_DATA_LENGTH = 8 // bytes

func NewSimulationCanClient(
	ctx context.Context,
	cfg *canModels.Config,
	name string,
	channel chan canModels.CanMessageTimestamped,
	logger *slog.Logger,
	network, uri *string,
	rate *int,
) *SimulationCanClient {
	if name == "" {
		panic(fmt.Errorf("connection name cannot be empty"))
	} else if channel == nil {
		panic(fmt.Errorf("message channel cannot be nil"))
	}

	if rate == nil || *rate == 0 {
		rate = &cfg.SimEmitRate
	}

	if uri == nil || *uri == "" {
		uri = &name
	}

	if network == nil || *network == "" {
		defaultNetwork := "sim"
		network = &defaultNetwork
	}

	return &SimulationCanClient{
		ctx:     ctx,
		name:    name,
		channel: channel,
		network: *network,
		uri:     *uri,
		l:       logger,
		rate:    *rate,
		cfg:     cfg,
	}
}

func (scc *SimulationCanClient) GetID() int {
	return scc.id
}

func (scc *SimulationCanClient) SetID(id int) {
	scc.id = id
}

func (scc *SimulationCanClient) GetURI() string {
	return scc.uri
}

func (scc *SimulationCanClient) SetURI(uri string) {
	scc.uri = uri
}

func (scc *SimulationCanClient) GetDBCFilePath() *string {
	return scc.dbcFilePath
}

func (scc *SimulationCanClient) SetDBCFilePath(uri *string) {}

func (scc *SimulationCanClient) GetNetwork() string {
	return scc.network
}

func (scc *SimulationCanClient) GetInterfaceName() string {
	return scc.GetName() + scc.cfg.CanInterfaceSeparator + scc.GetNetwork() + scc.cfg.CanInterfaceSeparator + scc.GetURI()
}

func (scc *SimulationCanClient) SetNetwork(network string) {
	scc.network = network
}

func (scc *SimulationCanClient) GetName() string {
	return scc.name
}

func (scc *SimulationCanClient) SetName(name string) {
	scc.name = name
}

func (scc *SimulationCanClient) GetConnection() net.Conn {
	return scc.connection
}

func (scc *SimulationCanClient) SetConnection(conn net.Conn) {
	scc.connection = conn
}

func (scc *SimulationCanClient) Open() error {
	scc.opened = true
	return nil
}

func (scc *SimulationCanClient) Close() error {
	scc.opened = false
	return nil
}

func (scc *SimulationCanClient) IsOpen() bool {
	return scc.opened
}

func (scc *SimulationCanClient) Discontinue() error {
	scc.streaming = false
	return nil
}

func (scc *SimulationCanClient) Receive(wg *sync.WaitGroup) {
	scc.streaming = true

	wg.Go(func() {
		for {
			// Create a slice of random bytes
			randomBytes := make([]byte, CAN_MESSAGE_MAX_DATA_LENGTH)

			// Fill with pseudo-random bytes (crypto quality not needed for simulation).
			for i := range randomBytes {
				randomBytes[i] = byte(mathRand.Intn(256))
			}

			// Select a random length for the data packet.
			lengthOfDataPacket := []uint8{
				CAN_MESSAGE_MAX_DATA_LENGTH / 4,
				CAN_MESSAGE_MAX_DATA_LENGTH / 2,
				CAN_MESSAGE_MAX_DATA_LENGTH,
			}
			randomLength := lengthOfDataPacket[mathRand.Intn(len(lengthOfDataPacket))]

			select {
			case scc.channel <- canModels.CanMessageTimestamped{
				Timestamp: time.Now().UnixNano(),
				Interface: scc.GetID(),
				Transmit:  false,
				ID:        uint32(mathRand.Intn(2047)),
				Remote:    false,
				Length:    randomLength,
				Data:      commonUtils.PadOrTrim(randomBytes[:randomLength], int(CAN_MESSAGE_MAX_DATA_LENGTH)),
			}:
			case <-scc.ctx.Done():
				return
			}

			scc.count++
			scc.l.Debug("emitted simulated can message", "count", scc.count)

			select {
			case <-time.After(time.Duration(scc.rate) * time.Nanosecond):
			case <-scc.ctx.Done():
				return
			}
		}
	})
}
