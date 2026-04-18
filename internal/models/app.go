package models

import (
	"context"
)

// SignalDispatcherRegistrar is the minimal interface app.go uses to register
// signal output channels with the dispatcher. Defined here to avoid an import
// cycle between the app and signaldispatch packages.
type SignalDispatcherRegistrar interface {
	AddListener(ch chan CanSignalTimestamped)
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
