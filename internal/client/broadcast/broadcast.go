package broadcast

import (
	"context"
	"fmt"
	"slices"

	canModel "github.com/robbiebyrd/bb/internal/models"
)

type BroadcastInterface interface {
	Add(listener BroadcastClientListener) error
	Remove(name string) error
	Broadcast()
}

type BroadcastClientListener struct {
	Name    string
	Channel chan canModel.CanMessage
}

type BroadcastClient struct {
	ctx               *context.Context
	broadcastChannels []BroadcastClientListener
	incomingChannel   chan canModel.CanMessage
	msgCount          int
}

func NewBroadcastClient(ctx *context.Context, incomingChannel chan canModel.CanMessage) *BroadcastClient {
	return &BroadcastClient{
		ctx:             ctx,
		incomingChannel: incomingChannel,
		msgCount:        0,
	}
}

func (scc *BroadcastClient) Add(listener BroadcastClientListener) error {
	for _, c := range scc.broadcastChannels {
		if c.Name == listener.Name {
			return fmt.Errorf("the name %v is already in use", listener.Name)
		}
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
	for {
		msg := <-scc.incomingChannel
		scc.msgCount++
		if scc.msgCount%100 == 0 {
			fmt.Printf("COUNT %v\n", scc.msgCount)
		}
		for _, c := range scc.broadcastChannels {
			c.Channel <- msg
		}
	}
	return nil
}
