package simulate

import (
	cryptoRand "crypto/rand"
	mathRand "math/rand"
	"sync"
	"time"

	"go.einride.tech/can/pkg/socketcan"

	commonUtils "github.com/robbiebyrd/bb/internal/client/common"
	"github.com/robbiebyrd/bb/internal/models/can"
)

func (scc *SimulationCanClient) Receive(wg *sync.WaitGroup) bool {
	scc.Receiver = socketcan.NewReceiver(scc.Connection)
	scc.Streaming = true

	wg.Go(func() {
		for {
			scc.simulateCanMessage()
		}
	})
	return true
}

func (scc *SimulationCanClient) simulateCanMessage() {
	// Simulate receiving a CAN frame every second
	var maxLength uint8 = 8

	// Create a slice of random bytes
	randomBytes := make([]byte, maxLength)

	// Read random bytes into the slice
	cryptoRand.Read(randomBytes)

	// Select a random length for the data packet.
	lengthOfDataPacket := []uint8{maxLength / 4, maxLength / 2, maxLength}
	randomLength := lengthOfDataPacket[mathRand.Intn(len(lengthOfDataPacket))]

	scc.Channel <- can.CanMessage{
		Timestamp: time.Now().Unix(),
		Interface: commonUtils.InterfaceName(scc),
		Transmit:  false,
		ID:        uint32(mathRand.Intn(255)),
		Remote:    false,
		Length:    randomLength,
		Data:      commonUtils.PadOrTrim(randomBytes[:randomLength], int(maxLength)),
	}

	time.Sleep(10 * time.Millisecond)
}

func generateRandomInt(r *mathRand.Rand, min, max int) int {
	return r.Intn(max-min+1) + min
}
