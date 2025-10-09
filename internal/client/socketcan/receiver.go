package socketcan

import (
	"sync"
	"time"

	goSocketCan "go.einride.tech/can/pkg/socketcan"

	commonUtils "github.com/robbiebyrd/bb/internal/client/common"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

func (scc *SocketCanConnectionClient) Receive(wg *sync.WaitGroup) {
	scc.Receiver = goSocketCan.NewReceiver(scc.Connection)
	scc.Streaming = true

	wg.Go(func() {
		for {
			for scc.Receiver.Receive() {
				frame := scc.Receiver.Frame()
				scc.Channel <- canModels.CanMessage{
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
