package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"github.com/newstack-cloud/celerity/apps/local-events/internal/app"
	"github.com/newstack-cloud/celerity/apps/local-events/internal/testutils"
)

type E2ESuite struct {
	suite.Suite
	dbClient    *dynamodb.Client
	minioClient *minio.Client
	rdb         *redis.Client
	logger      *zap.Logger
	configPath  string
}

func (s *E2ESuite) SetupSuite() {
	ctx := context.Background()
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("local"),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("local", "local", ""),
		),
	)
	s.Require().NoError(err)
	s.dbClient = dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String(testutils.DynamoDBEndpoint())
	})

	s.waitForDynamoDB()

	endpoint := endpointWithoutScheme(testutils.MinIOEndpoint())
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  miniocreds.NewStaticV4(testutils.MinIOAccessKey(), testutils.MinIOSecretKey(), ""),
		Secure: false,
	})
	s.Require().NoError(err)
	s.minioClient = minioClient
}

func (s *E2ESuite) waitForDynamoDB() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		_, err := s.dbClient.ListTables(ctx, &dynamodb.ListTablesInput{})
		if err == nil {
			return
		}
		select {
		case <-ctx.Done():
			s.Require().Fail(
				"DynamoDB Local not ready after 30s",
				"endpoint: %s", testutils.DynamoDBEndpoint(),
			)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (s *E2ESuite) SetupTest() {
	s.rdb = testutils.NewRedisClient(s.T())
	s.logger = testutils.NewLogger(s.T())
}

func (s *E2ESuite) TearDownTest() {
	if s.configPath != "" {
		os.Remove(s.configPath)
		s.configPath = ""
	}
	if s.rdb != nil {
		_ = s.rdb.Close()
	}
}

// --- helpers ---

func (s *E2ESuite) writeConfig(entries []map[string]any) {
	data, err := json.Marshal(entries)
	s.Require().NoError(err)
	f, err := os.CreateTemp("", "e2e-config-*.json")
	s.Require().NoError(err)
	_, err = f.Write(data)
	s.Require().NoError(err)
	s.Require().NoError(f.Close())
	s.configPath = f.Name()
	s.T().Setenv("CELERITY_LOCAL_EVENTS_CONFIG_FILE", s.configPath)
}

func (s *E2ESuite) startRun() (errCh <-chan error, cancel func()) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	ch := make(chan error, 1)
	go func() { ch <- app.Run(ctx, s.logger) }()
	return ch, cancelCtx
}

func (s *E2ESuite) stopRun(errCh <-chan error, cancel func()) {
	cancel()
	select {
	case err := <-errCh:
		s.Require().NoError(err)
	case <-time.After(5 * time.Second):
		s.Fail("app.Run() did not return within 5s of cancel")
	}
}

func (s *E2ESuite) createTestTable(ctx context.Context) string {
	tableName := sanitizeTableName(s.T().Name())
	_, err := s.dbClient.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []dbtypes.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: dbtypes.KeyTypeHash},
		},
		AttributeDefinitions: []dbtypes.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: dbtypes.ScalarAttributeTypeS},
		},
		BillingMode: dbtypes.BillingModePayPerRequest,
		StreamSpecification: &dbtypes.StreamSpecification{
			StreamEnabled:  aws.Bool(true),
			StreamViewType: dbtypes.StreamViewTypeNewAndOldImages,
		},
	})
	s.Require().NoError(err, "failed to create test table %s", tableName)
	return tableName
}

func (s *E2ESuite) deleteTestTable(ctx context.Context, tableName string) {
	_, _ = s.dbClient.DeleteTable(ctx, &dynamodb.DeleteTableInput{
		TableName: aws.String(tableName),
	})
}

func (s *E2ESuite) putItem(ctx context.Context, tableName, pk, data string) {
	_, err := s.dbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]dbtypes.AttributeValue{
			"pk":   &dbtypes.AttributeValueMemberS{Value: pk},
			"data": &dbtypes.AttributeValueMemberS{Value: data},
		},
	})
	s.Require().NoError(err)
}

func (s *E2ESuite) createTestBucket(ctx context.Context) string {
	bucketName := "test-" + sanitizeBucketName(s.T().Name())
	if len(bucketName) > 63 {
		bucketName = bucketName[:63]
	}
	bucketName = strings.TrimRight(bucketName, "-")
	err := s.minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	s.Require().NoError(err, "failed to create test bucket %s", bucketName)
	return bucketName
}

func (s *E2ESuite) uploadObject(ctx context.Context, bucket, key, content string) {
	_, err := s.minioClient.PutObject(
		ctx, bucket, key,
		bytes.NewReader([]byte(content)), int64(len(content)),
		minio.PutObjectOptions{ContentType: "application/octet-stream"},
	)
	s.Require().NoError(err, "failed to upload object %s/%s", bucket, key)
}

func endpointWithoutScheme(endpoint string) string {
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	return endpoint
}

func sanitizeTableName(name string) string {
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.':
			result = append(result, c)
		default:
			result = append(result, '_')
		}
	}
	if len(result) < 3 {
		result = append(result, "___"[:3-len(result)]...)
	}
	if len(result) > 255 {
		result = result[:255]
	}
	return string(result)
}

func sanitizeBucketName(name string) string {
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-':
			result = append(result, c)
		case c >= 'A' && c <= 'Z':
			result = append(result, c+32)
		default:
			result = append(result, '-')
		}
	}
	s := strings.Trim(string(result), "-")
	if len(s) < 3 {
		s = s + "xxx"
	}
	if len(s) > 63 {
		s = s[:63]
	}
	return s
}

// --- test cases ---

func (s *E2ESuite) Test_e2e_topic_bridge() {
	channel := testutils.UniqueStream(s.T(), "chan")
	stream := testutils.UniqueStream(s.T(), "e2e-topic")

	s.writeConfig([]map[string]any{{
		"type":    "topic_bridge",
		"source":  map[string]any{"channel": channel},
		"targets": []map[string]any{{"stream": stream}},
	}})

	errCh, cancel := s.startRun()
	defer cancel()

	time.Sleep(300 * time.Millisecond)

	err := s.rdb.Publish(context.Background(), channel, "e2e-topic-payload").Err()
	s.Require().NoError(err)

	msgs := testutils.ReadStreamMessages(s.T(), s.rdb, stream, 1, 5*time.Second)
	s.Require().Len(msgs, 1)
	s.Assert().Equal("e2e-topic-payload", msgs[0].Values["body"])

	s.stopRun(errCh, cancel)
}

func (s *E2ESuite) Test_e2e_dynamodb_stream_bridge() {
	ctx := context.Background()
	tableName := s.createTestTable(ctx)
	defer s.deleteTestTable(ctx, tableName)

	stream := testutils.UniqueStream(s.T(), "e2e-ddb")

	s.writeConfig([]map[string]any{{
		"type": "dynamodb_stream",
		"source": map[string]any{
			"endpoint":  testutils.DynamoDBEndpoint(),
			"region":    "local",
			"tableName": tableName,
		},
		"target": map[string]any{"stream": stream},
	}})

	errCh, cancel := s.startRun()
	defer cancel()

	time.Sleep(500 * time.Millisecond)

	s.putItem(ctx, tableName, "e2e-item-1", "hello-e2e")

	msgs := testutils.ReadStreamMessages(s.T(), s.rdb, stream, 1, 15*time.Second)
	s.Require().Len(msgs, 1)
	s.Assert().Equal("INSERT", msgs[0].Values["event_name"])

	s.stopRun(errCh, cancel)
}

func (s *E2ESuite) Test_e2e_minio_notification_bridge() {
	ctx := context.Background()
	bucket := s.createTestBucket(ctx)
	stream := testutils.UniqueStream(s.T(), "e2e-minio")

	s.writeConfig([]map[string]any{{
		"type": "minio_notification",
		"source": map[string]any{
			"endpoint":   testutils.MinIOEndpoint(),
			"accessKey":  testutils.MinIOAccessKey(),
			"secretKey":  testutils.MinIOSecretKey(),
			"bucketName": bucket,
			"events":     []string{"s3:ObjectCreated:*"},
		},
		"target": map[string]any{"stream": stream},
	}})

	errCh, cancel := s.startRun()
	defer cancel()

	time.Sleep(500 * time.Millisecond)

	s.uploadObject(ctx, bucket, "e2e-file.txt", "e2e-content")

	msgs := testutils.ReadStreamMessages(s.T(), s.rdb, stream, 1, 10*time.Second)
	s.Require().Len(msgs, 1)
	s.Assert().Contains(msgs[0].Values["event_name"], "ObjectCreated")

	s.stopRun(errCh, cancel)
}

func (s *E2ESuite) Test_e2e_all_bridges_combined() {
	ctx := context.Background()

	// Set up DynamoDB table.
	tableName := s.createTestTable(ctx)
	defer s.deleteTestTable(ctx, tableName)

	// Set up MinIO bucket.
	bucket := s.createTestBucket(ctx)

	topicChannel := testutils.UniqueStream(s.T(), "chan")
	topicStream := testutils.UniqueStream(s.T(), "combined-topic")
	ddbStream := testutils.UniqueStream(s.T(), "combined-ddb")
	minioStream := testutils.UniqueStream(s.T(), "combined-minio")
	scheduleStream := testutils.UniqueStream(s.T(), "combined-sched")

	s.writeConfig([]map[string]any{
		{
			"type":    "topic_bridge",
			"source":  map[string]any{"channel": topicChannel},
			"targets": []map[string]any{{"stream": topicStream}},
		},
		{
			"type": "dynamodb_stream",
			"source": map[string]any{
				"endpoint":  testutils.DynamoDBEndpoint(),
				"region":    "local",
				"tableName": tableName,
			},
			"target": map[string]any{"stream": ddbStream},
		},
		{
			"type": "minio_notification",
			"source": map[string]any{
				"endpoint":   testutils.MinIOEndpoint(),
				"accessKey":  testutils.MinIOAccessKey(),
				"secretKey":  testutils.MinIOSecretKey(),
				"bucketName": bucket,
				"events":     []string{"s3:ObjectCreated:*"},
			},
			"target": map[string]any{"stream": minioStream},
		},
		{
			"type": "schedule",
			"schedules": []map[string]any{{
				"id":       "e2e-cron",
				"schedule": "cron(* * * * *)",
				"stream":   scheduleStream,
				"input":    map[string]any{"key": "value"},
			}},
		},
	})

	errCh, cancel := s.startRun()
	defer cancel()

	time.Sleep(500 * time.Millisecond)

	// Trigger topic bridge.
	err := s.rdb.Publish(ctx, topicChannel, "combined-payload").Err()
	s.Require().NoError(err)

	// Trigger DynamoDB stream bridge.
	s.putItem(ctx, tableName, "combined-item", "combined-data")

	// Trigger MinIO notification bridge.
	s.uploadObject(ctx, bucket, "combined-file.txt", "combined-content")

	// Verify topic bridge.
	topicMsgs := testutils.ReadStreamMessages(s.T(), s.rdb, topicStream, 1, 5*time.Second)
	s.Require().Len(topicMsgs, 1)
	s.Assert().Equal("combined-payload", topicMsgs[0].Values["body"])

	// Verify DynamoDB stream bridge.
	ddbMsgs := testutils.ReadStreamMessages(s.T(), s.rdb, ddbStream, 1, 15*time.Second)
	s.Require().Len(ddbMsgs, 1)
	s.Assert().Equal("INSERT", ddbMsgs[0].Values["event_name"])

	// Verify MinIO notification bridge.
	minioMsgs := testutils.ReadStreamMessages(s.T(), s.rdb, minioStream, 1, 10*time.Second)
	s.Require().Len(minioMsgs, 1)
	s.Assert().Contains(minioMsgs[0].Values["event_name"], "ObjectCreated")

	// Verify schedule bridge fires within 60s (cron fires every minute).
	schedMsgs := testutils.ReadStreamMessages(s.T(), s.rdb, scheduleStream, 1, 65*time.Second)
	s.Require().Len(schedMsgs, 1)

	var body map[string]any
	err = json.Unmarshal([]byte(schedMsgs[0].Values["body"].(string)), &body)
	s.Require().NoError(err)
	s.Assert().Equal("e2e-cron", body["scheduleId"])

	s.stopRun(errCh, cancel)
}

func (s *E2ESuite) Test_e2e_run_returns_error_on_bad_config() {
	s.T().Setenv("CELERITY_LOCAL_EVENTS_CONFIG_FILE", "/nonexistent/path/config.json")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := app.Run(ctx, s.logger)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "loading config")
}

func (s *E2ESuite) Test_e2e_run_returns_error_on_bad_redis_url() {
	// Write a valid config so we get past config loading.
	s.writeConfig([]map[string]any{{
		"type":    "topic_bridge",
		"source":  map[string]any{"channel": "dummy"},
		"targets": []map[string]any{{"stream": "dummy"}},
	}})
	s.T().Setenv("CELERITY_LOCAL_REDIS_URL", "not-a-valid-url")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := app.Run(ctx, s.logger)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "invalid redis URL")
}

func TestE2ESuite(t *testing.T) {
	suite.Run(t, new(E2ESuite))
}
