package models

import (
	"context"
	"log/slog"
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
	GetConfig() *Config
	GetLogger() *slog.Logger
	GetLogLevel() *slog.LevelVar
	SetLogLevel(logLevel slog.Level)
	AddSignalDispatcher(d SignalDispatcherRegistrar, interfaceID int)
}
