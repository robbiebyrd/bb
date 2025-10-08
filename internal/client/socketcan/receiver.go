package socketcan

import (
	"sync"
	"time"

	goCan "go.einride.tech/can/pkg/socketcan"

	commonUtils "github.com/robbiebyrd/bb/internal/client/common"
	"github.com/robbiebyrd/bb/internal/models/can"
)

func (scc *SocketCanConnectionClient) Receive(wg *sync.WaitGroup) {
	scc.Receiver = goCan.NewReceiver(scc.Connection)
	scc.Streaming = true

	wg.Go(func() {
		for {
			for scc.Receiver.Receive() {
				frame := scc.Receiver.Frame()
				scc.Channel <- can.CanMessage{
					Timestamp: time.Now().Unix(),
					Interface: scc.GetInterfaceName(),
					Transmit:  false,
					ID:        frame.ID,
					Remote:    frame.IsRemote,
					Length:    frame.Length,
					Data:      commonUtils.PadOrTrim(frame.Data[:], 8),
				}
			}
		}
	})
}
