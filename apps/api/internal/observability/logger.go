package observability

import (
	"io"
	"log/slog"
	"os"
)

func NewLogger(level slog.Level) *slog.Logger {
	return NewLoggerWithWriter(level, os.Stdout)
}

func NewLoggerWithWriter(level slog.Level, writer io.Writer) *slog.Logger {
	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: level})
	return slog.New(handler).With("service", "api")
}
