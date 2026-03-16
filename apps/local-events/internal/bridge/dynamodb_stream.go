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
	// Track the last processed sequence number per shard so that restarts
	// (e.g. after a transient GetRecords error or shard rotation) resume
	// where we left off instead of re-reading from TrimHorizon.
	lastSeqByShardID := make(map[string]*string)

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
			lastSeq := p.pollShard(ctx, client, streamARN, *shard.ShardId, targetStream, logger, lastSeqByShardID[*shard.ShardId])
			if lastSeq != nil {
				lastSeqByShardID[*shard.ShardId] = lastSeq
			}
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
	afterSequenceNumber *string,
) *string {
	logger = logger.With(zap.String("shard", shardID))

	// Track the last processed sequence number so that iterator refreshes
	// and restarts (from the outer pollShards loop) resume where we left off
	// instead of re-reading from the beginning of the shard.
	lastSequenceNumber := afterSequenceNumber

	iterator, err := p.getShardIterator(ctx, client, streamARN, shardID, afterSequenceNumber)
	if err != nil {
		logger.Error("GetShardIterator failed", zap.Error(err))
		return lastSequenceNumber
	}

	iteratorCreated := time.Now()
	ticker := time.NewTicker(defaultPollInterval)
	defer ticker.Stop()

	for iterator != nil {
		select {
		case <-ctx.Done():
			logger.Info("shard poller stopped")
			return lastSequenceNumber
		case <-ticker.C:
		}

		if time.Since(iteratorCreated) > iteratorRefreshInterval {
			iterator, err = p.getShardIterator(ctx, client, streamARN, shardID, lastSequenceNumber)
			if err != nil {
				logger.Error("failed to refresh shard iterator", zap.Error(err))
				return lastSequenceNumber
			}
			iteratorCreated = time.Now()
		}

		out, err := client.GetRecords(ctx, &dynamodbstreams.GetRecordsInput{
			ShardIterator: iterator,
		})
		if err != nil {
			logger.Error("GetRecords failed", zap.Error(err))
			return lastSequenceNumber
		}

		for _, record := range out.Records {
			p.writeRecord(ctx, record, targetStream, logger)
			if record.Dynamodb != nil && record.Dynamodb.SequenceNumber != nil {
				lastSequenceNumber = record.Dynamodb.SequenceNumber
			}
		}

		iterator = out.NextShardIterator
	}

	// NextShardIterator is nil — shard is closed (uncommon in DynamoDB Local).
	logger.Info("shard closed")
	return lastSequenceNumber
}

func (p *DynamoDBStreamPoller) getShardIterator(
	ctx context.Context,
	client *dynamodbstreams.Client,
	streamARN string,
	shardID string,
	afterSequenceNumber *string,
) (*string, error) {
	input := &dynamodbstreams.GetShardIteratorInput{
		StreamArn: aws.String(streamARN),
		ShardId:   aws.String(shardID),
	}
	if afterSequenceNumber != nil {
		// Resume after the last processed record to avoid re-reading.
		input.ShardIteratorType = streamtypes.ShardIteratorTypeAfterSequenceNumber
		input.SequenceNumber = afterSequenceNumber
	} else {
		input.ShardIteratorType = streamtypes.ShardIteratorTypeTrimHorizon
	}
	out, err := client.GetShardIterator(ctx, input)
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
	// Build the body from just the StreamRecord part using DynamoDB JSON
	// format ({"S":"value"}) so the runtime's body transform can parse it.
	// json.Marshal on the Go SDK types doesn't produce the DynamoDB wire
	// format because AttributeValue is an interface without JSON tags.
	body, err := json.Marshal(marshalStreamRecord(record.Dynamodb))
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

// marshalStreamRecord converts a DynamoDB StreamRecord into a map with
// Keys, NewImage, OldImage in DynamoDB JSON attribute format.
func marshalStreamRecord(sr *streamtypes.StreamRecord) map[string]any {
	if sr == nil {
		return map[string]any{}
	}
	result := make(map[string]any)
	if sr.Keys != nil {
		result["Keys"] = marshalAttributeMap(sr.Keys)
	}
	if sr.NewImage != nil {
		result["NewImage"] = marshalAttributeMap(sr.NewImage)
	}
	if sr.OldImage != nil {
		result["OldImage"] = marshalAttributeMap(sr.OldImage)
	}
	return result
}

// marshalAttributeMap converts a DynamoDB attribute map to the standard
// DynamoDB JSON format (e.g. {"userId": {"S": "123"}}).
func marshalAttributeMap(attrs map[string]streamtypes.AttributeValue) map[string]any {
	result := make(map[string]any, len(attrs))
	for k, v := range attrs {
		result[k] = marshalAttributeValue(v)
	}
	return result
}

// marshalAttributeValue converts a single DynamoDB AttributeValue interface
// to the DynamoDB JSON wire format (e.g. {"S": "hello"}, {"N": "42"}).
func marshalAttributeValue(av streamtypes.AttributeValue) map[string]any {
	switch v := av.(type) {
	case *streamtypes.AttributeValueMemberS:
		return map[string]any{"S": v.Value}
	case *streamtypes.AttributeValueMemberN:
		return map[string]any{"N": v.Value}
	case *streamtypes.AttributeValueMemberBOOL:
		return map[string]any{"BOOL": v.Value}
	case *streamtypes.AttributeValueMemberNULL:
		return map[string]any{"NULL": true}
	case *streamtypes.AttributeValueMemberB:
		return map[string]any{"B": v.Value}
	case *streamtypes.AttributeValueMemberL:
		items := make([]any, len(v.Value))
		for i, item := range v.Value {
			items[i] = marshalAttributeValue(item)
		}
		return map[string]any{"L": items}
	case *streamtypes.AttributeValueMemberM:
		return map[string]any{"M": marshalAttributeMap(v.Value)}
	case *streamtypes.AttributeValueMemberSS:
		return map[string]any{"SS": v.Value}
	case *streamtypes.AttributeValueMemberNS:
		return map[string]any{"NS": v.Value}
	case *streamtypes.AttributeValueMemberBS:
		return map[string]any{"BS": v.Value}
	default:
		return nil
	}
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
