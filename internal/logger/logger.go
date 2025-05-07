package logger

import (
	"context"
	"io"
	"log/slog"

	"github.com/tsuru/rate-limit-control-plane/internal/config"
)

const (
	logLevelDebug = "debug"
	logLevelInfo  = "info"
	logLevelWarn  = "warn"
	logLevelError = "error"
)

type requestHandler struct {
	handler slog.Handler
}

func NewLogger(logContext map[string]string, w io.Writer) *slog.Logger {
	attrs := []slog.Attr{}
	for key, value := range logContext {
		attrs = append(attrs, slog.Attr{Key: key, Value: slog.StringValue(value)})
	}
	jsonHandler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: getLogLevel()}).WithAttrs(attrs)
	handler := requestHandler{jsonHandler}
	return slog.New(handler)
}

func getLogLevel() slog.Level {
	switch config.Spec.LogLevel {
	case logLevelDebug:
		return slog.LevelDebug
	case logLevelInfo:
		return slog.LevelInfo
	case logLevelWarn:
		return slog.LevelWarn
	case logLevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (h requestHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.handler.Handle(ctx, r)
}

func (h requestHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h requestHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newJsonHandler := h.handler.WithAttrs(attrs)
	return requestHandler{newJsonHandler}
}

func (h requestHandler) WithGroup(name string) slog.Handler {
	newJsonHandler := h.handler.WithGroup(name)
	return requestHandler{newJsonHandler}
}
