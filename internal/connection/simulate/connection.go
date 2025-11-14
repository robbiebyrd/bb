package simulate

import (
	"context"
	cryptoRand "crypto/rand"
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
	ctx        *context.Context
	Name       string
	Network    string
	URI        string
	Channel    chan canModels.CanMessageTimestamped
	Connection net.Conn
	Receiver   canModels.ReceiverInterface
	Opened     bool
	Streaming  bool
	l          *slog.Logger
	rate       int //ms
	count      int
	cfg        *canModels.Config
}

var CAN_MESSAGE_MAX_DATA_LENGTH uint8 = 8 // bytes

func NewSimulationCanClient(ctx *context.Context, cfg *canModels.Config, name string, channel chan canModels.CanMessageTimestamped, logger *slog.Logger, network, uri *string, rate *int) *SimulationCanClient {
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
		Name:    name,
		Channel: channel,
		Network: *network,
		URI:     *uri,
		l:       logger,
		rate:    *rate,
		cfg:     cfg,
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
	return scc.GetName() + scc.cfg.CanInterfaceSeparator + scc.GetNetwork() + scc.cfg.CanInterfaceSeparator + scc.GetURI()
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

func (scc *SimulationCanClient) Receive(wg *sync.WaitGroup) {
	scc.Streaming = true

	wg.Go(func() {
		for {

			// Create a slice of random bytes
			randomBytes := make([]byte, CAN_MESSAGE_MAX_DATA_LENGTH)

			// Read random bytes into the slice
			cryptoRand.Read(randomBytes)

			// Select a random length for the data packet.
			lengthOfDataPacket := []uint8{CAN_MESSAGE_MAX_DATA_LENGTH / 4, CAN_MESSAGE_MAX_DATA_LENGTH / 2, CAN_MESSAGE_MAX_DATA_LENGTH}
			randomLength := lengthOfDataPacket[mathRand.Intn(len(lengthOfDataPacket))]

			scc.Channel <- canModels.CanMessageTimestamped{
				Timestamp: time.Now().Unix(),
				Interface: scc.GetInterfaceName(),
				Transmit:  false,
				ID:        uint32(mathRand.Intn(255)),
				Remote:    false,
				Length:    randomLength,
				Data:      commonUtils.PadOrTrim(randomBytes[:randomLength], int(CAN_MESSAGE_MAX_DATA_LENGTH)),
			}

			scc.count++
			scc.l.Debug(fmt.Sprintf("emitted simulated can message #%v", scc.count))

			time.Sleep(time.Duration(scc.rate) * time.Microsecond)
		}
	})
}
