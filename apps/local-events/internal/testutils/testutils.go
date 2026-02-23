package testutils

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/newstack-cloud/celerity/apps/local-events/internal/config"
)

// Env reads an environment variable with a fallback default.
func Env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// DynamoDBEndpoint returns the DynamoDB Local endpoint for integration tests.
func DynamoDBEndpoint() string { return Env("TEST_DYNAMODB_ENDPOINT", "http://localhost:18000") }

// MinIOEndpoint returns the MinIO endpoint for integration tests.
func MinIOEndpoint() string { return Env("TEST_MINIO_ENDPOINT", "http://localhost:19000") }

// MinIOAccessKey returns the MinIO access key for integration tests.
func MinIOAccessKey() string { return Env("TEST_MINIO_ACCESS_KEY", "minioadmin") }

// MinIOSecretKey returns the MinIO secret key for integration tests.
func MinIOSecretKey() string { return Env("TEST_MINIO_SECRET_KEY", "minioadmin") }

// NewRedisClient creates a go-redis client pointed at the test Valkey
// instance and verifies connectivity with a PING.
// It reuses config.RedisURL() so the same CELERITY_LOCAL_REDIS_URL env var
// controls both production and test Redis resolution.
func NewRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	redisURL := config.RedisURL()
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("failed to parse Redis URL %q: %v", redisURL, err)
	}
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("cannot connect to test Redis at %s: %v", redisURL, err)
	}
	return client
}

// NewLogger creates a zap development logger for test output.
func NewLogger(t *testing.T) *zap.Logger {
	t.Helper()
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("failed to create test logger: %v", err)
	}
	return logger
}

// FlushRedis deletes all keys in the current Redis database.
func FlushRedis(t *testing.T, client *redis.Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("failed to flush Redis: %v", err)
	}
}

// ReadStreamMessages reads up to count messages from a Redis stream,
// blocking for up to timeout. Returns the messages or fails the test
// if the timeout is exceeded before count messages arrive.
func ReadStreamMessages(
	t *testing.T,
	client *redis.Client,
	stream string,
	count int,
	timeout time.Duration,
) []redis.XMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var messages []redis.XMessage
	lastID := "0-0"

	for len(messages) < count {
		result, err := client.XRead(ctx, &redis.XReadArgs{
			Streams: []string{stream, lastID},
			Count:   int64(count - len(messages)),
			Block:   500 * time.Millisecond,
		}).Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				t.Fatalf(
					"timed out waiting for %d messages on stream %q (got %d)",
					count, stream, len(messages),
				)
			}
			t.Fatalf("XRead failed on stream %q: %v", stream, err)
		}
		for _, s := range result {
			messages = append(messages, s.Messages...)
			if len(s.Messages) > 0 {
				lastID = s.Messages[len(s.Messages)-1].ID
			}
		}
	}

	return messages[:count]
}

// UniqueStream returns a stream name scoped to the test to avoid collisions.
func UniqueStream(t *testing.T, prefix string) string {
	t.Helper()
	return fmt.Sprintf("test:%s:%s", prefix, t.Name())
}
