package bridge

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"github.com/newstack-cloud/celerity/apps/local-events/internal/config"
)

type DynamoDBStreamPollerSuite struct {
	suite.Suite
	dbClient *dynamodb.Client
	rdb      *redis.Client
	logger   *zap.Logger
}

func (s *DynamoDBStreamPollerSuite) SetupSuite() {
	ctx := context.Background()
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("local"),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("local", "local", ""),
		),
	)
	s.Require().NoError(err)
	s.dbClient = dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String(testDynamoDBEndpoint())
	})

	// Wait for DynamoDB Local to be ready before running tests.
	s.waitForDynamoDB()
}

func (s *DynamoDBStreamPollerSuite) waitForDynamoDB() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		_, err := s.dbClient.ListTables(ctx, &dynamodb.ListTablesInput{})
		if err == nil {
			return
		}
		select {
		case <-ctx.Done():
			s.Require().Fail("DynamoDB Local not ready at %s after 30s", testDynamoDBEndpoint())
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (s *DynamoDBStreamPollerSuite) SetupTest() {
	s.rdb = newTestRedisClient(s.T())
	s.logger = newTestLogger(s.T())
}

func (s *DynamoDBStreamPollerSuite) TearDownTest() {
	_ = s.rdb.Close()
}

func sanitizeTableName(name string) string {
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.':
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

func (s *DynamoDBStreamPollerSuite) createTestTable(ctx context.Context) string {
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

func (s *DynamoDBStreamPollerSuite) deleteTestTable(ctx context.Context, tableName string) {
	_, _ = s.dbClient.DeleteTable(ctx, &dynamodb.DeleteTableInput{
		TableName: aws.String(tableName),
	})
}

func (s *DynamoDBStreamPollerSuite) putItem(ctx context.Context, tableName, pk, data string) {
	_, err := s.dbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]dbtypes.AttributeValue{
			"pk":   &dbtypes.AttributeValueMemberS{Value: pk},
			"data": &dbtypes.AttributeValueMemberS{Value: data},
		},
	})
	s.Require().NoError(err)
}

func (s *DynamoDBStreamPollerSuite) deleteItem(ctx context.Context, tableName, pk string) {
	_, err := s.dbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]dbtypes.AttributeValue{
			"pk": &dbtypes.AttributeValueMemberS{Value: pk},
		},
	})
	s.Require().NoError(err)
}

// startPoller launches the DynamoDBStreamPoller in a goroutine and returns
// a cancel function that stops it and waits for the goroutine to finish.
func (s *DynamoDBStreamPollerSuite) startPoller(
	tableName string,
	stream string,
) (cancel func()) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		poller := NewDynamoDBStreamPoller(s.rdb, s.logger)
		poller.Start(ctx, &config.DynamoDBStreamSource{
			Endpoint:  testDynamoDBEndpoint(),
			Region:    "local",
			TableName: tableName,
		}, &config.StreamTarget{Stream: stream})
	}()
	return func() {
		cancelCtx()
		wg.Wait()
	}
}

func (s *DynamoDBStreamPollerSuite) Test_insert_record_appears_in_stream() {
	ctx := context.Background()
	tableName := s.createTestTable(ctx)
	defer s.deleteTestTable(ctx, tableName)

	stream := uniqueStream(s.T(), "ddb")
	stop := s.startPoller(tableName, stream)
	defer stop()

	s.putItem(ctx, tableName, "item-1", "hello")

	msgs := readStreamMessages(s.T(), s.rdb, stream, 1, 15*time.Second)
	s.Require().Len(msgs, 1)

	s.Assert().Equal("INSERT", msgs[0].Values["event_name"])
	s.Assert().Equal("0", msgs[0].Values["message_type"])

	// Body should be valid JSON.
	var body map[string]any
	err := json.Unmarshal([]byte(msgs[0].Values["body"].(string)), &body)
	s.Assert().NoError(err, "body should be valid JSON")
}

func (s *DynamoDBStreamPollerSuite) Test_record_update_produces_modify_event() {
	ctx := context.Background()
	tableName := s.createTestTable(ctx)
	defer s.deleteTestTable(ctx, tableName)

	stream := uniqueStream(s.T(), "ddb")
	stop := s.startPoller(tableName, stream)
	defer stop()

	s.putItem(ctx, tableName, "item-1", "original")
	time.Sleep(200 * time.Millisecond)
	s.putItem(ctx, tableName, "item-1", "modified")

	msgs := readStreamMessages(s.T(), s.rdb, stream, 2, 15*time.Second)
	s.Require().Len(msgs, 2)

	s.Assert().Equal("INSERT", msgs[0].Values["event_name"])
	s.Assert().Equal("MODIFY", msgs[1].Values["event_name"])
}

func (s *DynamoDBStreamPollerSuite) Test_delete_record_produces_REMOVE_event() {
	ctx := context.Background()
	tableName := s.createTestTable(ctx)
	defer s.deleteTestTable(ctx, tableName)

	stream := uniqueStream(s.T(), "ddb")
	stop := s.startPoller(tableName, stream)
	defer stop()

	s.putItem(ctx, tableName, "item-1", "to-delete")
	time.Sleep(200 * time.Millisecond)
	s.deleteItem(ctx, tableName, "item-1")

	msgs := readStreamMessages(s.T(), s.rdb, stream, 2, 15*time.Second)
	s.Require().Len(msgs, 2)

	s.Assert().Equal("INSERT", msgs[0].Values["event_name"])
	s.Assert().Equal("REMOVE", msgs[1].Values["event_name"])
}

func (s *DynamoDBStreamPollerSuite) Test_multiple_inserts_arrive_in_order() {
	ctx := context.Background()
	tableName := s.createTestTable(ctx)
	defer s.deleteTestTable(ctx, tableName)

	stream := uniqueStream(s.T(), "ddb")
	stop := s.startPoller(tableName, stream)
	defer stop()

	for i := range 5 {
		s.putItem(ctx, tableName, "item-"+string(rune('0'+i)), "data")
		time.Sleep(50 * time.Millisecond)
	}

	msgs := readStreamMessages(s.T(), s.rdb, stream, 5, 15*time.Second)
	s.Require().Len(msgs, 5)
	for _, msg := range msgs {
		s.Assert().Equal("INSERT", msg.Values["event_name"])
	}
}

func (s *DynamoDBStreamPollerSuite) Test_poller_stops_on_context_cancel() {
	ctx := context.Background()
	tableName := s.createTestTable(ctx)
	defer s.deleteTestTable(ctx, tableName)

	stream := uniqueStream(s.T(), "ddb")

	pollerCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		poller := NewDynamoDBStreamPoller(s.rdb, s.logger)
		poller.Start(pollerCtx, &config.DynamoDBStreamSource{
			Endpoint:  testDynamoDBEndpoint(),
			Region:    "local",
			TableName: tableName,
		}, &config.StreamTarget{Stream: stream})
	}()

	// Let the poller start, then cancel.
	time.Sleep(1 * time.Second)
	cancel()

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		s.Fail("poller goroutine did not stop within 5s of context cancel")
	}
}

func TestDynamoDBStreamPollerSuite(t *testing.T) {
	suite.Run(t, new(DynamoDBStreamPollerSuite))
}
