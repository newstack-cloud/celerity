package bridge

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/newstack-cloud/celerity/apps/local-events/internal/config"
)

// TopicBridge subscribes to a Valkey pub/sub channel and fans out each
// published message to one or more Valkey streams (one per subscribing consumer).
type TopicBridge struct {
	rdb    *redis.Client
	logger *zap.Logger
}

// NewTopicBridge creates a new TopicBridge.
func NewTopicBridge(rdb *redis.Client, logger *zap.Logger) *TopicBridge {
	return &TopicBridge{rdb: rdb, logger: logger}
}

// Start subscribes to the source pub/sub channel and writes each received
// message to all target streams. It blocks until ctx is cancelled.
func (tb *TopicBridge) Start(
	ctx context.Context,
	source *config.TopicBridgeSource,
	targets []config.TopicBridgeTarget,
) {
	logger := tb.logger.With(
		zap.String("channel", source.Channel),
		zap.Int("targets", len(targets)),
	)

	pubsub := tb.rdb.Subscribe(ctx, source.Channel)
	defer func() { _ = pubsub.Close() }()

	logger.Info("subscribed to topic channel")

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			logger.Info("topic bridge stopped")
			return
		case msg, ok := <-ch:
			if !ok {
				logger.Info("topic channel closed")
				return
			}
			tb.fanOut(ctx, msg.Payload, targets, logger)
		}
	}
}

func (tb *TopicBridge) fanOut(
	ctx context.Context,
	payload string,
	targets []config.TopicBridgeTarget,
	logger *zap.Logger,
) {
	now := time.Now()
	for _, target := range targets {
		err := tb.rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: target.Stream,
			Values: map[string]any{
				"body":         payload,
				"timestamp":    fmt.Sprintf("%d", now.Unix()),
				"message_type": "0",
			},
		}).Err()
		if err != nil {
			logger.Error("failed to write to target stream",
				zap.String("stream", target.Stream),
				zap.Error(err),
			)
			continue
		}
		logger.Debug("topic message relayed",
			zap.String("stream", target.Stream),
		)
	}
}
