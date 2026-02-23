package main

import (
	"context"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/newstack-cloud/celerity/apps/local-events/internal/app"
)

func main() {
	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := app.Run(ctx, logger); err != nil {
		logger.Fatal("fatal error", zap.Error(err))
	}
}
