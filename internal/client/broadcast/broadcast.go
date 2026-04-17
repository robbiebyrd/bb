package broadcast

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync/atomic"
	"time"

	"github.com/robbiebyrd/bb/internal/client/common"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

type BroadcastClientListener struct {
	Name    string
	Channel chan canModels.CanMessageTimestamped
	Filter  *canModels.CanMessageFilterGroup
}

type BroadcastClient struct {
	ctx               context.Context
	broadcastChannels []BroadcastClientListener
	incomingChannel   chan canModels.CanMessageTimestamped
	msgCount          atomic.Uint64
	l                 *slog.Logger
}

func NewBroadcastClient(
	ctx context.Context,
	incomingChannel chan canModels.CanMessageTimestamped,
	l *slog.Logger,
) *BroadcastClient {
	return &BroadcastClient{
		ctx:             ctx,
		incomingChannel: incomingChannel,
		l:               l,
	}
}

func (scc *BroadcastClient) Add(listener BroadcastClientListener) error {
	if scc.listenerExists(listener) {
		return fmt.Errorf("the name %v is already in use", listener.Name)
	}
	scc.broadcastChannels = append(scc.broadcastChannels, listener)

	return nil
}

func (scc *BroadcastClient) Remove(name string) error {
	for i, c := range scc.broadcastChannels {
		if c.Name == name {
			scc.broadcastChannels = slices.Delete(scc.broadcastChannels, i, i+1)
			return nil
		}
	}

	return fmt.Errorf("could not find name %v", name)
}

func (scc *BroadcastClient) Broadcast() error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	go func() {
		var lastCount uint64
		for range ticker.C {
			current := scc.msgCount.Load()
			rate := (current - lastCount) / 5
			lastCount = current
			scc.l.Info("broadcast throughput",
				"msgs_per_sec", rate,
				"buffer_queue", len(scc.incomingChannel),
			)
		}
	}()

	for {
		select {
		case <-scc.ctx.Done():
			return scc.ctx.Err()
		case canMsg := <-scc.incomingChannel:
			scc.msgCount.Add(1)
			for _, c := range scc.broadcastChannels {
				broadcastMsg := true
				if c.Filter != nil {
					broadcastMsg = scc.testFilterGroup(c, canMsg)
				}
				if broadcastMsg {
					select {
					case c.Channel <- canMsg:
					default:
						scc.l.Warn("broadcast: dropped message, listener channel full",
							"listener", c.Name)
					}
				}
			}
		}
	}
}

func (scc *BroadcastClient) testFilterGroup(
	c BroadcastClientListener,
	canMsg canModels.CanMessageTimestamped,
) bool {
	filterValues := []bool{}

	for _, f := range c.Filter.Filters {
		filterValues = append(filterValues, f.Filter(canMsg))
	}

	switch c.Filter.Operator {
	case canModels.FilterOr:
		return common.ArrayContainsTrue(filterValues)
	default:
		return common.ArrayAllTrue(filterValues)
	}
}

func (scc *BroadcastClient) listenerExists(listener BroadcastClientListener) bool {
	for _, c := range scc.broadcastChannels {
		if c.Name == listener.Name {
			return true
		}
	}
	return false
}
