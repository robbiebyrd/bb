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
				now := time.Now().UnixNano()
				scc.channel <- canModels.CanMessageTimestamped{
					Timestamp: now,
					Interface: scc.GetID(),
					Transmit:  false,
					ID:        frame.ID,
					Remote:    frame.IsRemote,
					Length:    frame.Length,
					Data:      commonUtils.PadOrTrim(frame.Data[:], 8),
				}
			}
			// Inner loop exited — check for graceful stop
			if !scc.streaming {
				return
			}
			// Backoff before reconnect attempt
			time.Sleep(100 * time.Millisecond)
		}
	})
}
