package broadcast

import (
	"context"
	"fmt"
	"slices"

	"github.com/robbiebyrd/bb/internal/client/common"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

type BroadcastInterface interface {
	Add(listener BroadcastClientListener) error
	Remove(name string) error
	Broadcast()
}

type BroadcastClientListener struct {
	Name    string
	Channel chan canModels.CanMessageTimestamped
	Filter  *canModels.CanMessageFilterGroup
}

type BroadcastClient struct {
	ctx               *context.Context
	broadcastChannels []BroadcastClientListener
	incomingChannel   chan canModels.CanMessageTimestamped
	msgCount          int
}

func NewBroadcastClient(
	ctx *context.Context,
	incomingChannel chan canModels.CanMessageTimestamped,
) *BroadcastClient {
	return &BroadcastClient{
		ctx:             ctx,
		incomingChannel: incomingChannel,
		msgCount:        0,
	}
}

func (scc *BroadcastClient) Add(listener BroadcastClientListener) error {
	if scc.listenerExists(listener) {
		return fmt.Errorf("the name %v is already in use", listener.Name)
	}
	scc.broadcastChannels = append(scc.broadcastChannels, listener)

	return nil
}

func (scc *BroadcastClient) AddMany(listeners []BroadcastClientListener) error {
	for _, listener := range listeners {
		err := scc.Add(listener)
		if err != nil {
			return err
		}
	}
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
	for {
		canMsg := <-scc.incomingChannel
		scc.msgCount++
		for _, c := range scc.broadcastChannels {
			broadcastMsg := true
			if c.Filter != nil {
				broadcastMsg = scc.testFilterGroup(c, canMsg)
			}
			if broadcastMsg {
				c.Channel <- canMsg
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
