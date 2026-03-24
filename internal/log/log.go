package log

import (
	"context"
	"io"
	"log/slog"
	"os"
)

type Logger interface {
	Debug(msg string, fields ...any)
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
}

type slogLogger struct {
	inner *slog.Logger
}

func New(verbose bool) Logger {
	return newWithWriter(verbose, os.Stderr)
}

func newWithWriter(verbose bool, w io.Writer) Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	return slogLogger{inner: slog.New(handler)}
}

func (l slogLogger) Debug(msg string, fields ...any) {
	l.inner.DebugContext(context.Background(), msg, fields...)
}

func (l slogLogger) Info(msg string, fields ...any) {
	l.inner.InfoContext(context.Background(), msg, fields...)
}

func (l slogLogger) Warn(msg string, fields ...any) {
	l.inner.WarnContext(context.Background(), msg, fields...)
}

func (l slogLogger) Error(msg string, fields ...any) {
	l.inner.ErrorContext(context.Background(), msg, fields...)
}
