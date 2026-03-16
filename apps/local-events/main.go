package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/newstack-cloud/celerity/apps/local-events/internal/app"
)

func main() {
	logger, _ := newLogger()
	defer func() { _ = logger.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := app.Run(ctx, logger); err != nil {
		logger.Fatal("fatal error", zap.Error(err))
	}
}

// newLogger creates a zap logger whose level is controlled by the
// LOG_LEVEL environment variable (debug, info, warn, error).
// Defaults to info when unset.
func newLogger() (*zap.Logger, error) {
	level := zapcore.InfoLevel
	if env := strings.ToLower(os.Getenv("LOG_LEVEL")); env != "" {
		if err := level.UnmarshalText([]byte(env)); err == nil {
			// parsed successfully
		}
	}
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(level)
	return cfg.Build()
}
