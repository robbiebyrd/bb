package simulate

import (
	"context"
	"fmt"
	"log/slog"
	mathRand "math/rand"
	"net"
	"sync"
	"time"

	commonUtils "github.com/robbiebyrd/cantou/internal/client/common"
	canModels "github.com/robbiebyrd/cantou/internal/models"
)

type SimulationCanClient struct {
	ctx         context.Context
	id          int
	name        string
	network     string
	uri         string
	channel       chan canModels.CanMessageTimestamped
	connection    net.Conn
	opened        bool
	streaming     bool
	l             *slog.Logger
	rate          int  // milliseconds (fixed); used when useRandomRate is false
	rateMin       int  // milliseconds (random range lower bound)
	rateMax       int  // milliseconds (random range upper bound)
	useRandomRate bool // true when SIM_RATE is unset and both MIN/MAX are provided
	count         int
	cfg         *canModels.Config
	dbcFilePath *string
}

const canMessageMaxDataLength = 8 // bytes

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
		if cfg.SimEmitRate > 0 {
			rate = &cfg.SimEmitRate
		} else {
			defaultRate := 10
			rate = &defaultRate
		}
	}

	if uri == nil || *uri == "" {
		uri = &name
	}

	if network == nil || *network == "" {
		defaultNetwork := "sim"
		network = &defaultNetwork
	}

	useRandomRate := cfg.SimEmitRate == 0 && cfg.SimEmitRateMin > 0 && cfg.SimEmitRateMax > cfg.SimEmitRateMin

	return &SimulationCanClient{
		ctx:           ctx,
		name:          name,
		channel:       channel,
		network:       *network,
		uri:           *uri,
		l:             logger,
		rate:          *rate,
		rateMin:       cfg.SimEmitRateMin,
		rateMax:       cfg.SimEmitRateMax,
		useRandomRate: useRandomRate,
		cfg:           cfg,
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
		// Per-goroutine rand.Rand avoids contention on the global math/rand
		// source (which takes a mutex on every call). At high emit rates the
		// global lock becomes the hot contention point.
		rng := mathRand.New(mathRand.NewSource(time.Now().UnixNano()))

		lengthOfDataPacket := [...]uint8{
			canMessageMaxDataLength / 4,
			canMessageMaxDataLength / 2,
			canMessageMaxDataLength,
		}

		for {
			// Create a slice of random bytes
			randomBytes := make([]byte, canMessageMaxDataLength)

			// Fill with pseudo-random bytes (crypto quality not needed for simulation).
			for i := range randomBytes {
				randomBytes[i] = byte(rng.Intn(256))
			}

			// Select a random length for the data packet.
			randomLength := lengthOfDataPacket[rng.Intn(len(lengthOfDataPacket))]

			select {
			case scc.channel <- canModels.CanMessageTimestamped{
				Timestamp: time.Now().UnixNano(),
				Interface: scc.GetID(),
				Transmit:  false,
				ID:        uint32(rng.Intn(2047)),
				Remote:    false,
				Length:    randomLength,
				Data:      commonUtils.PadOrTrim(randomBytes[:randomLength], int(canMessageMaxDataLength)),
			}:
			case <-scc.ctx.Done():
				return
			}

			scc.count++
			scc.l.Debug("emitted simulated can message", "count", scc.count)

			sleepMs := scc.rate
			if scc.useRandomRate {
				sleepMs = scc.rateMin + rng.Intn(scc.rateMax-scc.rateMin+1)
			}

			select {
			case <-time.After(time.Duration(sleepMs) * time.Millisecond):
			case <-scc.ctx.Done():
				return
			}
		}
	})
}
