package logging

import (
	"log/slog"
	"os"
)

func NewJSONLogger(logLevel string) *slog.Logger {
	logLevelTranslated := slog.LevelInfo
	switch logLevel {
	case "debug", "DEBUG":
		logLevelTranslated = slog.LevelDebug
	case "error", "ERROR":
		logLevelTranslated = slog.LevelError
	case "warn", "WARN":
		logLevelTranslated = slog.LevelWarn
	}

	return slog.New(
		slog.NewJSONHandler(
			os.Stdout, &slog.HandlerOptions{Level: logLevelTranslated},
		),
	)
}
