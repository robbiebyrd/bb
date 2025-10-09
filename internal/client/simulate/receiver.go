package simulate

import (
	cryptoRand "crypto/rand"
	"fmt"
	mathRand "math/rand"
	"sync"
	"time"

	commonUtils "github.com/robbiebyrd/bb/internal/client/common"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

func (scc *SimulationCanClient) Receive(wg *sync.WaitGroup) {
	scc.Streaming = true

	wg.Go(func() {
		for {
			scc.simulateCanMessage()
		}
	})
}

func (scc *SimulationCanClient) simulateCanMessage() {
	// Simulate receiving a CAN frame x seconds
	var maxLength uint8 = 8

	// Create a slice of random bytes
	randomBytes := make([]byte, maxLength)

	// Read random bytes into the slice
	cryptoRand.Read(randomBytes)

	// Select a random length for the data packet.
	lengthOfDataPacket := []uint8{maxLength / 4, maxLength / 2, maxLength}
	randomLength := lengthOfDataPacket[mathRand.Intn(len(lengthOfDataPacket))]

	scc.Channel <- canModels.CanMessage{
		Timestamp: time.Now().Unix(),
		Interface: scc.GetInterfaceName(),
		Transmit:  false,
		ID:        uint32(mathRand.Intn(255)),
		Remote:    false,
		Length:    randomLength,
		Data:      commonUtils.PadOrTrim(randomBytes[:randomLength], int(maxLength)),
	}
	scc.count++
	scc.l.Debug(fmt.Sprintf("emitted simulated can message #%v", scc.count))

	time.Sleep(time.Duration(scc.rate) * time.Millisecond)
}
