package logger

import (
	"context"
	"log/slog"
	"os"
)

type ctxKey string

const CorrelationIDKey ctxKey = "X-Correlation-ID"

var log *slog.Logger

func init() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{})
	log = slog.New(handler)
}

func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return log
	}
	if reqID, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return log.With("correlation_id", reqID)
	}
	return log
}

func Info(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Info(msg, args...)
}

func Error(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Error(msg, args...)
}

func Warn(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Warn(msg, args...)
}
