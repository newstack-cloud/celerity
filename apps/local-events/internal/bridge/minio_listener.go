package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/notification"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/newstack-cloud/celerity/apps/local-events/internal/config"
)

const reconnectBackoff = 5 * time.Second

// MinIOListener listens for bucket notifications from MinIO and writes
// events to a Valkey stream.
type MinIOListener struct {
	rdb    *redis.Client
	logger *zap.Logger
}

// NewMinIOListener creates a new MinIOListener.
func NewMinIOListener(rdb *redis.Client, logger *zap.Logger) *MinIOListener {
	return &MinIOListener{rdb: rdb, logger: logger}
}

// Start connects to MinIO and listens for bucket notifications, writing each
// event to the target Valkey stream. It blocks until ctx is cancelled,
// reconnecting with backoff on failure.
func (l *MinIOListener) Start(
	ctx context.Context,
	source *config.MinIONotificationSource,
	target *config.StreamTarget,
) {
	logger := l.logger.With(
		zap.String("bucket", source.Bucket),
		zap.String("stream", target.Stream),
	)

	for {
		err := l.listen(ctx, source, target.Stream, logger)
		if ctx.Err() != nil {
			logger.Info("minio listener stopped")
			return
		}
		logger.Warn("minio listener disconnected, reconnecting",
			zap.Error(err),
			zap.Duration("backoff", reconnectBackoff),
		)
		if sleepOrDone(ctx, reconnectBackoff) {
			return
		}
	}
}

func (l *MinIOListener) listen(
	ctx context.Context,
	source *config.MinIONotificationSource,
	targetStream string,
	logger *zap.Logger,
) error {
	client, err := minio.New(endpointWithoutScheme(source.Endpoint), &minio.Options{
		Creds:  miniocreds.NewStaticV4(source.AccessKey, source.SecretKey, ""),
		Secure: false,
	})
	if err != nil {
		return fmt.Errorf("creating minio client: %w", err)
	}

	// ListenBucketNotification returns a channel that blocks until events arrive.
	notifyCh := client.ListenBucketNotification(ctx, source.Bucket, "", "", source.Events)

	logger.Info("listening for bucket notifications")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case info, ok := <-notifyCh:
			if !ok {
				return fmt.Errorf("notification channel closed")
			}
			if info.Err != nil {
				return fmt.Errorf("notification error: %w", info.Err)
			}
			for _, record := range info.Records {
				l.writeEvent(ctx, record, targetStream, logger)
			}
		}
	}
}

func (l *MinIOListener) writeEvent(
	ctx context.Context,
	record notification.Event,
	targetStream string,
	logger *zap.Logger,
) {
	body, err := json.Marshal(record)
	if err != nil {
		logger.Error("failed to marshal notification event", zap.Error(err))
		return
	}

	now := time.Now()

	err = l.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: targetStream,
		Values: map[string]any{
			"body":         string(body),
			"timestamp":    fmt.Sprintf("%d", now.Unix()),
			"message_type": "0",
			"event_name":   record.EventName,
		},
	}).Err()
	if err != nil {
		logger.Error("failed to write event to stream", zap.Error(err))
		return
	}

	logger.Debug("bucket event written",
		zap.String("event", record.EventName),
		zap.String("stream", targetStream),
	)
}

// endpointWithoutScheme strips the http:// or https:// prefix for minio-go,
// which expects a bare host:port.
func endpointWithoutScheme(endpoint string) string {
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	return endpoint
}
