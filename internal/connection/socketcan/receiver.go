package socketcan

import (
	"sync"
	"time"

	goSocketCan "go.einride.tech/can/pkg/socketcan"

	commonUtils "github.com/robbiebyrd/bb/internal/client/common"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

func (scc *SocketCanConnectionClient) Receive(wg *sync.WaitGroup) {
	scc.receiver = goSocketCan.NewReceiver(scc.connection)
	scc.streaming = true

	wg.Go(func() {
		for {
			for scc.receiver.Receive() {
				frame := scc.receiver.Frame()
				now := time.Now().Unix()
				scc.channel <- canModels.CanMessage{
					Timestamp: now,
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
