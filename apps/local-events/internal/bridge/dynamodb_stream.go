package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
	streamtypes "github.com/aws/aws-sdk-go-v2/service/dynamodbstreams/types"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/newstack-cloud/celerity/apps/local-events/internal/config"
)

const (
	defaultPollInterval = 1 * time.Second
	// DynamoDB shard iterators expire after 15 minutes.
	iteratorRefreshInterval = 14 * time.Minute
)

// DynamoDBStreamPoller polls DynamoDB Streams on a local DynamoDB instance
// and writes change records to a Valkey stream.
type DynamoDBStreamPoller struct {
	rdb    *redis.Client
	logger *zap.Logger
}

// NewDynamoDBStreamPoller creates a new DynamoDBStreamPoller.
func NewDynamoDBStreamPoller(rdb *redis.Client, logger *zap.Logger) *DynamoDBStreamPoller {
	return &DynamoDBStreamPoller{rdb: rdb, logger: logger}
}

// Start polls a DynamoDB Local table's stream and writes records to the
// target Valkey stream. It blocks until ctx is cancelled.
func (p *DynamoDBStreamPoller) Start(
	ctx context.Context,
	source *config.DynamoDBStreamSource,
	target *config.StreamTarget,
) {
	logger := p.logger.With(
		zap.String("table", source.TableName),
		zap.String("stream", target.Stream),
	)

	cfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(source.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("local", "local", ""),
		),
	)
	if err != nil {
		logger.Error("failed to load AWS config", zap.Error(err))
		return
	}

	dbClient := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String(source.Endpoint)
	})
	streamsClient := dynamodbstreams.NewFromConfig(
		cfg,
		func(o *dynamodbstreams.Options) {
			o.BaseEndpoint = aws.String(source.Endpoint)
		},
	)

	streamARN, err := p.waitForStreamARN(ctx, dbClient, source.TableName, logger)
	if err != nil {
		logger.Error("failed to get stream ARN", zap.Error(err))
		return
	}
	logger.Info("found stream ARN", zap.String("arn", streamARN))

	p.pollShards(ctx, streamsClient, streamARN, target.Stream, logger)
}

// waitForStreamARN polls DescribeTable until the table has an active stream.
func (p *DynamoDBStreamPoller) waitForStreamARN(
	ctx context.Context,
	client *dynamodb.Client,
	tableName string,
	logger *zap.Logger,
) (string, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		out, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
			TableName: aws.String(tableName),
		})
		if err != nil {
			logger.Warn("DescribeTable failed, retrying", zap.Error(err))
		} else if out.Table.LatestStreamArn != nil {
			return *out.Table.LatestStreamArn, nil
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}
	}
}

// pollShards discovers shards and polls each one for records.
func (p *DynamoDBStreamPoller) pollShards(
	ctx context.Context,
	client *dynamodbstreams.Client,
	streamARN string,
	targetStream string,
	logger *zap.Logger,
) {
	for {
		shards, err := p.describeShards(ctx, client, streamARN)
		if err != nil {
			logger.Error("DescribeStream failed", zap.Error(err))
			if sleepOrDone(ctx, 5*time.Second) {
				return
			}
			continue
		}

		if len(shards) == 0 {
			if sleepOrDone(ctx, defaultPollInterval) {
				return
			}
			continue
		}

		// DynamoDB Local typically has a single shard per table.
		for _, shard := range shards {
			if shard.ShardId == nil {
				continue
			}
			p.pollShard(ctx, client, streamARN, *shard.ShardId, targetStream, logger)
			if ctx.Err() != nil {
				return
			}
		}
	}
}

func (p *DynamoDBStreamPoller) describeShards(
	ctx context.Context,
	client *dynamodbstreams.Client,
	streamARN string,
) ([]streamtypes.Shard, error) {
	out, err := client.DescribeStream(ctx, &dynamodbstreams.DescribeStreamInput{
		StreamArn: aws.String(streamARN),
	})
	if err != nil {
		return nil, err
	}
	return out.StreamDescription.Shards, nil
}

func (p *DynamoDBStreamPoller) pollShard(
	ctx context.Context,
	client *dynamodbstreams.Client,
	streamARN string,
	shardID string,
	targetStream string,
	logger *zap.Logger,
) {
	logger = logger.With(zap.String("shard", shardID))

	iterator, err := p.getShardIterator(ctx, client, streamARN, shardID)
	if err != nil {
		logger.Error("GetShardIterator failed", zap.Error(err))
		return
	}

	iteratorCreated := time.Now()
	ticker := time.NewTicker(defaultPollInterval)
	defer ticker.Stop()

	for iterator != nil {
		select {
		case <-ctx.Done():
			logger.Info("shard poller stopped")
			return
		case <-ticker.C:
		}

		if time.Since(iteratorCreated) > iteratorRefreshInterval {
			iterator, err = p.getShardIterator(ctx, client, streamARN, shardID)
			if err != nil {
				logger.Error("failed to refresh shard iterator", zap.Error(err))
				return
			}
			iteratorCreated = time.Now()
		}

		out, err := client.GetRecords(ctx, &dynamodbstreams.GetRecordsInput{
			ShardIterator: iterator,
		})
		if err != nil {
			logger.Error("GetRecords failed", zap.Error(err))
			return
		}

		for _, record := range out.Records {
			p.writeRecord(ctx, record, targetStream, logger)
		}

		iterator = out.NextShardIterator
	}

	// NextShardIterator is nil — shard is closed (uncommon in DynamoDB Local).
	logger.Info("shard closed")
}

func (p *DynamoDBStreamPoller) getShardIterator(
	ctx context.Context,
	client *dynamodbstreams.Client,
	streamARN string,
	shardID string,
) (*string, error) {
	out, err := client.GetShardIterator(ctx, &dynamodbstreams.GetShardIteratorInput{
		StreamArn:         aws.String(streamARN),
		ShardId:           aws.String(shardID),
		ShardIteratorType: streamtypes.ShardIteratorTypeTrimHorizon,
	})
	if err != nil {
		return nil, err
	}
	return out.ShardIterator, nil
}

func (p *DynamoDBStreamPoller) writeRecord(
	ctx context.Context,
	record streamtypes.Record,
	targetStream string,
	logger *zap.Logger,
) {
	body, err := json.Marshal(record)
	if err != nil {
		logger.Error("failed to marshal stream record", zap.Error(err))
		return
	}

	eventName := eventNameFromRecord(record)
	now := time.Now()

	err = p.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: targetStream,
		Values: map[string]any{
			"body":         string(body),
			"timestamp":    fmt.Sprintf("%d", now.Unix()),
			"message_type": "0",
			"event_name":   eventName,
		},
	}).Err()
	if err != nil {
		logger.Error("failed to write record to stream", zap.Error(err))
		return
	}

	logger.Debug("stream record written",
		zap.String("event", eventName),
		zap.String("stream", targetStream),
	)
}

func eventNameFromRecord(record streamtypes.Record) string {
	if record.EventName == "" {
		return "UNKNOWN"
	}
	// streamtypes.OperationType values: Insert, Modify, Remove.
	switch record.EventName {
	case streamtypes.OperationTypeInsert:
		return "INSERT"
	case streamtypes.OperationTypeModify:
		return "MODIFY"
	case streamtypes.OperationTypeRemove:
		return "REMOVE"
	default:
		return string(record.EventName)
	}
}

// sleepOrDone returns true if the context is done, false if the sleep completed.
func sleepOrDone(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return true
	case <-time.After(d):
		return false
	}
}
