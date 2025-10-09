package logging

import (
	"io"
	"log/slog"
)

func NewJSONLogger(out io.Writer) *slog.Logger {
	jsonHandler := slog.NewJSONHandler(out, nil)
	return slog.New(jsonHandler)
}
