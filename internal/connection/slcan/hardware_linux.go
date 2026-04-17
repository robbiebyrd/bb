//go:build linux

package slcan

import (
	"fmt"
	"sync"
	"time"

	"github.com/roffe/gocan"

	commonUtils "github.com/robbiebyrd/bb/internal/client/common"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

func (scc *SLCanConnectionClient) Open() error {
	adapter, err := gocan.NewSLCan(&gocan.AdapterConfig{
		Port:         scc.uri,
		PortBaudrate: defaultPortBaudrate,
		CANRate:      defaultCANRate,
		Debug:        scc.cfg.LogLevel == "debug",
	})
	if err != nil {
		return fmt.Errorf("creating slcan adapter for %s: %w", scc.uri, err)
	}
	if err := adapter.Open(scc.ctx); err != nil {
		return fmt.Errorf("opening slcan adapter %s: %w", scc.uri, err)
	}
	scc.adapter = adapter
	scc.opened = true
	return nil
}

func (scc *SLCanConnectionClient) Close() error {
	if err := scc.adapter.(gocan.Adapter).Close(); err != nil {
		return fmt.Errorf("closing slcan adapter %s: %w", scc.name, err)
	}
	scc.opened = false
	return nil
}

func (scc *SLCanConnectionClient) Receive(wg *sync.WaitGroup) {
	scc.quit = make(chan struct{})
	scc.streaming = true
	recvCh := scc.adapter.(gocan.Adapter).Recv()

	wg.Go(func() {
		for {
			select {
			case frame, ok := <-recvCh:
				if !ok {
					return
				}
				scc.channel <- canModels.CanMessageTimestamped{
					Timestamp: time.Now().UnixNano(),
					Interface: scc.GetID(),
					Transmit:  frame.FrameType.Type == gocan.ResponseTypeOutgoing,
					ID:        frame.Identifier,
					Remote:    frame.RTR,
					Length:    uint8(len(frame.Data)),
					Data:      commonUtils.PadOrTrim(frame.Data, 8),
				}
			case <-scc.quit:
				return
			case <-scc.ctx.Done():
				return
			}
		}
	})
}
