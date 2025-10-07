package socketcan

import (
	"sync"
	"time"

	"go.einride.tech/can/pkg/socketcan"

	commonUtils "github.com/robbiebyrd/bb/internal/client/common"
	"github.com/robbiebyrd/bb/internal/models/can"
)

func (scc *CanConnectionClient) Receive(wg *sync.WaitGroup) bool {
	scc.Receiver = socketcan.NewReceiver(scc.Connection)
	scc.Streaming = true

	wg.Go(func() {
		for {
			for scc.Receiver.Receive() {
				frame := scc.Receiver.Frame()
				scc.Channel <- can.CanMessage{
					Timestamp: time.Now().Unix(),
					Interface: commonUtils.InterfaceName(scc),
					Transmit:  false,
					ID:        frame.ID,
					Remote:    frame.IsRemote,
					Length:    frame.Length,
					Data:      commonUtils.PadOrTrim(frame.Data[:], 8),
				}
			}
		}
	})
	return true
}
