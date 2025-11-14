package models

import (
	"context"
	"log/slog"
)

type AppInterface interface {
	AddOutput(c OutputClient)
	RemoveOutput(name string)
	RemoveOutputs(names []string)
	AddOutputs(cs []OutputClient)
	Run()
	GetConnections() ConnectionManager
	GetContext() *context.Context
	GetConfig() *Config
	GetLogger() *slog.Logger
	GetLogLevel() *slog.LevelVar
	SetLogLevel(logLevel slog.Level)
}
