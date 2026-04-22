package models

import (
	"context"
)

// MessageDispatcherListener and SignalDispatcherListener are defined here to
// avoid import cycles — app.go constructs them, but the concrete dispatcher
// types live in separate packages.

type MessageDispatcherListener struct {
	Name    string
	Channel chan CanMessageTimestamped
	Filter  *CanMessageFilterGroup
}

type SignalDispatcherListener struct {
	Name    string
	Channel chan CanSignalTimestamped
}

type SignalDispatcherRegistrar interface {
	AddListener(listener SignalDispatcherListener)
	GetChannel() chan CanMessageTimestamped
	Dispatch() error
}

type AppInterface interface {
	AddOutput(c OutputClient)
	AddOutputs(cs []OutputClient)
	Run() error
	GetConnections() ConnectionManager
	GetContext() context.Context
	AddSignalDispatcher(d SignalDispatcherRegistrar, interfaceID int)
}
