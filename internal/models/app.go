package models

import (
	"context"
	"log/slog"
)

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
}
