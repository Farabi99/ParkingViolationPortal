package shutdown

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"parking-portal/pkg/logger"
)

func Graceful(ctx context.Context, timeout time.Duration, fns ...func(context.Context) error) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig

	logger.Info(ctx, "Received shutdown signal", "signal", s.String())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, fn := range fns {
		if err := fn(shutdownCtx); err != nil {
			logger.Error(ctx, "Error during shutdown", "error", err)
		}
	}
	logger.Info(ctx, "Graceful shutdown complete")
}
