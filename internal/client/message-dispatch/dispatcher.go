package messagedispatch

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	canModels "github.com/robbiebyrd/cantou/internal/models"
)

type registeredListener struct {
	listener canModels.MessageDispatcherListener
	dropped  atomic.Uint64
}

type MessageDispatcher struct {
	ctx               context.Context
	mu                sync.RWMutex
	broadcastChannels []*registeredListener
	incomingChannel   chan canModels.CanMessageTimestamped
	msgCount          atomic.Uint64
	l                 *slog.Logger
}

func NewMessageDispatcher(
	ctx context.Context,
	incomingChannel chan canModels.CanMessageTimestamped,
	l *slog.Logger,
) *MessageDispatcher {
	return &MessageDispatcher{
		ctx:             ctx,
		incomingChannel: incomingChannel,
		l:               l,
	}
}

func (scc *MessageDispatcher) Add(listener canModels.MessageDispatcherListener) error {
	scc.mu.Lock()
	defer scc.mu.Unlock()
	for _, c := range scc.broadcastChannels {
		if c.listener.Name == listener.Name {
			return fmt.Errorf("the name %v is already in use", listener.Name)
		}
	}
	scc.broadcastChannels = append(scc.broadcastChannels, &registeredListener{listener: listener})

	return nil
}

func (scc *MessageDispatcher) Remove(name string) error {
	scc.mu.Lock()
	defer scc.mu.Unlock()
	for i, c := range scc.broadcastChannels {
		if c.listener.Name == name {
			scc.broadcastChannels = slices.Delete(scc.broadcastChannels, i, i+1)
			return nil
		}
	}

	return fmt.Errorf("could not find name %v", name)
}

func (scc *MessageDispatcher) Broadcast() error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	go func() {
		var lastCount uint64
		for {
			select {
			case <-ticker.C:
				current := scc.msgCount.Load()
				rate := (current - lastCount) / 5
				lastCount = current
				scc.l.Info("broadcast throughput",
					"msgs_per_sec", rate,
					"buffer_queue", len(scc.incomingChannel),
				)
			case <-scc.ctx.Done():
				return
			}
		}
	}()

	for {
		select {
		case <-scc.ctx.Done():
			return scc.ctx.Err()
		case canMsg := <-scc.incomingChannel:
			scc.msgCount.Add(1)
			scc.mu.RLock()
			listeners := make([]*registeredListener, len(scc.broadcastChannels))
			copy(listeners, scc.broadcastChannels)
			scc.mu.RUnlock()
			for _, c := range listeners {
				broadcastMsg := true
				if c.listener.Filter != nil {
					broadcastMsg = scc.testFilterGroup(c.listener, canMsg)
				}
				if broadcastMsg {
					select {
					case c.listener.Channel <- canMsg:
					default:
						dropped := c.dropped.Add(1)
						if dropped == 1 || dropped%1000 == 0 {
							scc.l.Warn("broadcast: dropped message, listener channel full",
								"listener", c.listener.Name,
								"dropped_total", dropped)
						}
					}
				}
			}
		}
	}
}

func (scc *MessageDispatcher) testFilterGroup(
	c canModels.MessageDispatcherListener,
	canMsg canModels.CanMessageTimestamped,
) bool {
	switch c.Filter.Operator {
	case canModels.FilterOr:
		for _, f := range c.Filter.Filters {
			if f.Filter(canMsg) {
				return true
			}
		}
		return false
	default:
		for _, f := range c.Filter.Filters {
			if !f.Filter(canMsg) {
				return false
			}
		}
		return true
	}
}

// DroppedCount returns the total number of messages that have been dropped for
// the named listener because its channel was full. Returns 0 if the listener
// is not registered.
func (scc *MessageDispatcher) DroppedCount(name string) uint64 {
	scc.mu.RLock()
	defer scc.mu.RUnlock()
	for _, c := range scc.broadcastChannels {
		if c.listener.Name == name {
			return c.dropped.Load()
		}
	}
	return 0
}
