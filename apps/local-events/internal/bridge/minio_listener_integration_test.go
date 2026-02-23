package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"github.com/newstack-cloud/celerity/apps/local-events/internal/config"
)

type MinIOListenerSuite struct {
	suite.Suite
	minioClient *minio.Client
	rdb         *redis.Client
	logger      *zap.Logger
}

func (s *MinIOListenerSuite) SetupSuite() {
	endpoint := endpointWithoutScheme(testMinIOEndpoint())
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  miniocreds.NewStaticV4(testMinIOAccessKey(), testMinIOSecretKey(), ""),
		Secure: false,
	})
	s.Require().NoError(err, "failed to create MinIO test client")
	s.minioClient = client
}

func (s *MinIOListenerSuite) SetupTest() {
	s.rdb = newTestRedisClient(s.T())
	s.logger = newTestLogger(s.T())
}

func (s *MinIOListenerSuite) TearDownTest() {
	_ = s.rdb.Close()
}

func sanitizeBucketName(name string) string {
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-':
			result = append(result, c)
		case c >= 'A' && c <= 'Z':
			result = append(result, c+32) // lowercase
		default:
			result = append(result, '-')
		}
	}
	s := strings.Trim(string(result), "-")
	if len(s) < 3 {
		s = fmt.Sprintf("%sxxxx", s)
	}
	if len(s) > 63 {
		s = s[:63]
	}
	return s
}

func (s *MinIOListenerSuite) createTestBucket(ctx context.Context) string {
	// Reserve 5 chars for "test-" prefix within the 63-char bucket name limit.
	bucketName := "test-" + sanitizeBucketName(s.T().Name())
	if len(bucketName) > 63 {
		bucketName = bucketName[:63]
	}
	bucketName = strings.TrimRight(bucketName, "-")
	err := s.minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	s.Require().NoError(err, "failed to create test bucket %s", bucketName)
	return bucketName
}

// startListener launches the MinIOListener in a goroutine and returns
// a cancel function that stops it and waits for the goroutine to finish.
func (s *MinIOListenerSuite) startListener(
	bucket string,
	events []string,
	stream string,
) (cancel func()) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		listener := NewMinIOListener(s.rdb, s.logger)
		listener.Start(ctx, &config.MinIONotificationSource{
			Endpoint:  testMinIOEndpoint(),
			AccessKey: testMinIOAccessKey(),
			SecretKey: testMinIOSecretKey(),
			Bucket:    bucket,
			Events:    events,
		}, &config.StreamTarget{Stream: stream})
	}()
	// Allow time for the notification subscription to be established.
	time.Sleep(500 * time.Millisecond)
	return func() {
		cancelCtx()
		wg.Wait()
	}
}

func (s *MinIOListenerSuite) uploadObject(ctx context.Context, bucket, key, content string) {
	_, err := s.minioClient.PutObject(ctx, bucket, key, bytes.NewReader([]byte(content)), int64(len(content)), minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	s.Require().NoError(err, "failed to upload object %s/%s", bucket, key)
}

func (s *MinIOListenerSuite) removeObject(ctx context.Context, bucket, key string) {
	err := s.minioClient.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
	s.Require().NoError(err, "failed to remove object %s/%s", bucket, key)
}

func (s *MinIOListenerSuite) Test_object_created_event_written_to_stream() {
	ctx := context.Background()
	bucket := s.createTestBucket(ctx)
	stream := uniqueStream(s.T(), "minio")

	stop := s.startListener(bucket, []string{"s3:ObjectCreated:*"}, stream)
	defer stop()

	s.uploadObject(ctx, bucket, "test-file.txt", "hello world")

	msgs := readStreamMessages(s.T(), s.rdb, stream, 1, 10*time.Second)
	s.Require().Len(msgs, 1)

	s.Assert().Contains(msgs[0].Values["event_name"], "ObjectCreated")
	s.Assert().Equal("0", msgs[0].Values["message_type"])

	// Body should be valid JSON.
	var body map[string]any
	err := json.Unmarshal([]byte(msgs[0].Values["body"].(string)), &body)
	s.Assert().NoError(err, "body should be valid JSON")
}

func (s *MinIOListenerSuite) Test_object_removed_event_written_to_stream() {
	ctx := context.Background()
	bucket := s.createTestBucket(ctx)
	stream := uniqueStream(s.T(), "minio")

	stop := s.startListener(bucket, []string{"s3:ObjectCreated:*", "s3:ObjectRemoved:*"}, stream)
	defer stop()

	s.uploadObject(ctx, bucket, "to-delete.txt", "delete me")
	time.Sleep(200 * time.Millisecond)
	s.removeObject(ctx, bucket, "to-delete.txt")

	msgs := readStreamMessages(s.T(), s.rdb, stream, 2, 10*time.Second)
	s.Require().Len(msgs, 2)

	s.Assert().Contains(msgs[0].Values["event_name"], "ObjectCreated")
	s.Assert().Contains(msgs[1].Values["event_name"], "ObjectRemoved")
}

func (s *MinIOListenerSuite) Test_multiple_uploads_produce_multiple_events() {
	ctx := context.Background()
	bucket := s.createTestBucket(ctx)
	stream := uniqueStream(s.T(), "minio")

	stop := s.startListener(bucket, []string{"s3:ObjectCreated:*"}, stream)
	defer stop()

	for i := 0; i < 3; i++ {
		s.uploadObject(ctx, bucket, "file-"+string(rune('0'+i))+".txt", "content")
		time.Sleep(100 * time.Millisecond)
	}

	msgs := readStreamMessages(s.T(), s.rdb, stream, 3, 10*time.Second)
	s.Require().Len(msgs, 3)

	for _, msg := range msgs {
		s.Assert().Contains(msg.Values["event_name"], "ObjectCreated")
	}
}

func (s *MinIOListenerSuite) Test_listener_stops_on_context_cancel() {
	ctx := context.Background()
	bucket := s.createTestBucket(ctx)
	stream := uniqueStream(s.T(), "minio")

	listenerCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		listener := NewMinIOListener(s.rdb, s.logger)
		listener.Start(listenerCtx, &config.MinIONotificationSource{
			Endpoint:  testMinIOEndpoint(),
			AccessKey: testMinIOAccessKey(),
			SecretKey: testMinIOSecretKey(),
			Bucket:    bucket,
			Events:    []string{"s3:ObjectCreated:*"},
		}, &config.StreamTarget{Stream: stream})
	}()

	time.Sleep(500 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// success
	case <-time.After(3 * time.Second):
		s.Fail("listener goroutine did not stop within 3s of context cancel")
	}
}

func TestMinIOListenerSuite(t *testing.T) {
	suite.Run(t, new(MinIOListenerSuite))
}
