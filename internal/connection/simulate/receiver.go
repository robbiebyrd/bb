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

var CAN_MESSAGE_MAX_DATA_LENGTH uint8 = 8

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

			scc.Channel <- canModels.CanMessage{
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
