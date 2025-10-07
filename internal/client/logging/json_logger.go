package logging

import (
	"io"
	"log/slog"
	"os"
)

func NewJSONLogger(out io.Writer) *slog.Logger {
	if out == nil {
		out = os.Stdout
	}

	jsonHandler := slog.NewJSONHandler(out, nil)
	jlog := slog.New(jsonHandler)

	return jlog
}
