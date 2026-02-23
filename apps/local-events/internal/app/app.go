package app

import (
	"context"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/newstack-cloud/celerity/apps/local-events/internal/bridge"
	"github.com/newstack-cloud/celerity/apps/local-events/internal/config"
)

// Run loads bridge configuration, connects to Valkey, starts all configured
// bridges, and blocks until ctx is cancelled. It returns an error if
// configuration loading or Redis connectivity fails.
func Run(ctx context.Context, logger *zap.Logger) error {
	bridges, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	redisURL := config.RedisURL()
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return fmt.Errorf("invalid redis URL %q: %w", redisURL, err)
	}
	redisClient := redis.NewClient(opts)

	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("cannot connect to Valkey: %w", err)
	}
	logger.Info("connected to Valkey", zap.String("url", redisURL))

	var wg sync.WaitGroup

	for _, b := range bridges {
		switch b.Type {
		case "schedule":
			cfg := b.Schedule
			if cfg == nil || len(cfg.Schedules) == 0 {
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				trigger := bridge.NewScheduleTrigger(redisClient, logger)
				trigger.Start(ctx, cfg.Schedules)
			}()
			logger.Info("started schedule bridge", zap.Int("schedules", len(cfg.Schedules)))

		case "topic_bridge":
			cfg := b.TopicBridge
			if cfg == nil || len(cfg.Targets) == 0 {
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				tb := bridge.NewTopicBridge(redisClient, logger)
				tb.Start(ctx, &cfg.Source, cfg.Targets)
			}()
			logger.Info("started topic bridge",
				zap.String("channel", cfg.Source.Channel),
				zap.Int("targets", len(cfg.Targets)),
			)

		case "dynamodb_stream":
			cfg := b.DynamoDBStream
			if cfg == nil {
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				poller := bridge.NewDynamoDBStreamPoller(redisClient, logger)
				poller.Start(ctx, &cfg.Source, &cfg.Target)
			}()
			logger.Info("started dynamodb stream bridge",
				zap.String("table", cfg.Source.TableName),
				zap.String("stream", cfg.Target.Stream),
			)

		case "minio_notification":
			cfg := b.MinIONotification
			if cfg == nil {
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				listener := bridge.NewMinIOListener(redisClient, logger)
				listener.Start(ctx, &cfg.Source, &cfg.Target)
			}()
			logger.Info("started minio notification bridge",
				zap.String("bucket", cfg.Source.Bucket),
				zap.String("stream", cfg.Target.Stream),
			)

		default:
			logger.Warn("unknown bridge type, skipping", zap.String("type", b.Type))
		}
	}

	<-ctx.Done()
	logger.Info("shutting down")

	wg.Wait()
	_ = redisClient.Close()
	logger.Info("shutdown complete")
	return nil
}
