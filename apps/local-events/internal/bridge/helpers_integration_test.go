package bridge

import (
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/newstack-cloud/celerity/apps/local-events/internal/testutils"
)

func testDynamoDBEndpoint() string { return testutils.DynamoDBEndpoint() }
func testMinIOEndpoint() string    { return testutils.MinIOEndpoint() }
func testMinIOAccessKey() string   { return testutils.MinIOAccessKey() }
func testMinIOSecretKey() string   { return testutils.MinIOSecretKey() }

func newTestRedisClient(t *testing.T) *redis.Client { return testutils.NewRedisClient(t) }
func newTestLogger(t *testing.T) *zap.Logger        { return testutils.NewLogger(t) }

func readStreamMessages(
	t *testing.T,
	client *redis.Client,
	stream string,
	count int,
	timeout time.Duration,
) []redis.XMessage {
	return testutils.ReadStreamMessages(t, client, stream, count, timeout)
}

func uniqueStream(t *testing.T, prefix string) string {
	return testutils.UniqueStream(t, prefix)
}
