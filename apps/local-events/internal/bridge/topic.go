package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/newstack-cloud/celerity/apps/local-events/internal/config"
)

// topicEnvelope is the structured JSON payload published by the SDK's
// Redis topic provider. The bridge parses this to extract individual
// fields for the target stream, mirroring how SNS preserves message
// attributes when delivering to SQS subscriptions.
type topicEnvelope struct {
	Body       string            `json:"body"`
	Subject    string            `json:"subject,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

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
	values := tb.buildStreamValues(payload, logger)
	for _, target := range targets {
		err := tb.rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: target.Stream,
			Values: values,
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

// buildStreamValues parses the SDK envelope and produces the stream field map.
// If the payload is not a valid envelope, it falls back to treating the raw
// payload as the body.
func (tb *TopicBridge) buildStreamValues(payload string, logger *zap.Logger) map[string]any {
	now := fmt.Sprintf("%d", time.Now().Unix())

	var env topicEnvelope
	if err := json.Unmarshal([]byte(payload), &env); err != nil || env.Body == "" {
		if err != nil {
			logger.Warn("payload is not a valid topic envelope, using raw body",
				zap.Error(err),
			)
		}
		return map[string]any{
			"body":         payload,
			"timestamp":    now,
			"message_type": "0",
		}
	}

	values := map[string]any{
		"body":         env.Body,
		"timestamp":    now,
		"message_type": "0",
	}
	if env.Subject != "" {
		values["subject"] = env.Subject
	}
	if len(env.Attributes) > 0 {
		attrs, err := json.Marshal(env.Attributes)
		if err != nil {
			logger.Warn("failed to re-serialize envelope attributes",
				zap.Error(err),
			)
		} else {
			values["attributes"] = string(attrs)
		}
	}
	return values
}
