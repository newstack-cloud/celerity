package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ConfigSuite struct {
	suite.Suite
}

func (s *ConfigSuite) writeConfig(dir, content string) string {
	path := filepath.Join(dir, "config.json")
	err := os.WriteFile(path, []byte(content), 0644)
	s.Require().NoError(err)
	return path
}

// --- Load tests ---

func (s *ConfigSuite) Test_Load_all_bridge_types() {
	dir := s.T().TempDir()
	path := s.writeConfig(dir, `[
		{
			"type": "schedule",
			"schedules": [
				{"id": "s1", "schedule": "rate(5 minutes)", "stream": "sched-stream", "input": {"key": "val"}}
			]
		},
		{
			"type": "topic_bridge",
			"source": {"channel": "my-channel"},
			"targets": [{"stream": "t1"}, {"stream": "t2"}]
		},
		{
			"type": "dynamodb_stream",
			"source": {"endpoint": "http://localhost:8000", "region": "us-east-1", "tableName": "my-table"},
			"target": {"stream": "ddb-stream"}
		},
		{
			"type": "minio_notification",
			"source": {"endpoint": "http://localhost:9000", "accessKey": "ak", "secretKey": "sk", "bucketName": "bkt", "events": ["s3:ObjectCreated:*"]},
			"target": {"stream": "minio-stream"}
		}
	]`)
	s.T().Setenv("CELERITY_LOCAL_EVENTS_CONFIG_FILE", path)

	bridges, err := Load()
	s.Require().NoError(err)
	s.Require().Len(bridges, 4)

	// Schedule
	s.Assert().Equal("schedule", bridges[0].Type)
	s.Require().NotNil(bridges[0].Schedule)
	s.Assert().Len(bridges[0].Schedule.Schedules, 1)
	s.Assert().Equal("s1", bridges[0].Schedule.Schedules[0].ID)
	s.Assert().Equal("rate(5 minutes)", bridges[0].Schedule.Schedules[0].Schedule)
	s.Assert().Equal("sched-stream", bridges[0].Schedule.Schedules[0].Stream)

	// Topic bridge
	s.Assert().Equal("topic_bridge", bridges[1].Type)
	s.Require().NotNil(bridges[1].TopicBridge)
	s.Assert().Equal("my-channel", bridges[1].TopicBridge.Source.Channel)
	s.Assert().Len(bridges[1].TopicBridge.Targets, 2)

	// DynamoDB stream
	s.Assert().Equal("dynamodb_stream", bridges[2].Type)
	s.Require().NotNil(bridges[2].DynamoDBStream)
	s.Assert().Equal("my-table", bridges[2].DynamoDBStream.Source.TableName)
	s.Assert().Equal("ddb-stream", bridges[2].DynamoDBStream.Target.Stream)

	// MinIO notification
	s.Assert().Equal("minio_notification", bridges[3].Type)
	s.Require().NotNil(bridges[3].MinIONotification)
	s.Assert().Equal("bkt", bridges[3].MinIONotification.Source.Bucket)
	s.Assert().Equal("minio-stream", bridges[3].MinIONotification.Target.Stream)
	s.Assert().Equal([]string{"s3:ObjectCreated:*"}, bridges[3].MinIONotification.Source.Events)
}

func (s *ConfigSuite) Test_Load_unknown_bridge_type_preserved() {
	dir := s.T().TempDir()
	path := s.writeConfig(dir, `[{"type": "future_type", "foo": "bar"}]`)
	s.T().Setenv("CELERITY_LOCAL_EVENTS_CONFIG_FILE", path)

	bridges, err := Load()
	s.Require().NoError(err)
	s.Require().Len(bridges, 1)
	s.Assert().Equal("future_type", bridges[0].Type)
	s.Assert().Nil(bridges[0].Schedule)
	s.Assert().Nil(bridges[0].TopicBridge)
	s.Assert().Nil(bridges[0].DynamoDBStream)
	s.Assert().Nil(bridges[0].MinIONotification)
}

func (s *ConfigSuite) Test_Load_empty_array() {
	dir := s.T().TempDir()
	path := s.writeConfig(dir, `[]`)
	s.T().Setenv("CELERITY_LOCAL_EVENTS_CONFIG_FILE", path)

	bridges, err := Load()
	s.Require().NoError(err)
	s.Assert().Empty(bridges)
}

func (s *ConfigSuite) Test_Load_missing_file_returns_error() {
	s.T().Setenv("CELERITY_LOCAL_EVENTS_CONFIG_FILE", "/nonexistent/path.json")

	_, err := Load()
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "reading config file")
}

func (s *ConfigSuite) Test_Load_invalid_json_returns_error() {
	dir := s.T().TempDir()
	path := s.writeConfig(dir, `not valid json`)
	s.T().Setenv("CELERITY_LOCAL_EVENTS_CONFIG_FILE", path)

	_, err := Load()
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "parsing config file")
}

func (s *ConfigSuite) Test_Load_invalid_type_field_returns_error() {
	dir := s.T().TempDir()
	path := s.writeConfig(dir, `[{"type": 123}]`)
	s.T().Setenv("CELERITY_LOCAL_EVENTS_CONFIG_FILE", path)

	_, err := Load()
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "parsing bridge 0 type")
}

func (s *ConfigSuite) Test_Load_invalid_schedule_bridge_returns_error() {
	dir := s.T().TempDir()
	path := s.writeConfig(dir, `[{"type": "schedule", "schedules": "not-an-array"}]`)
	s.T().Setenv("CELERITY_LOCAL_EVENTS_CONFIG_FILE", path)

	_, err := Load()
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "parsing schedule bridge 0")
}

func (s *ConfigSuite) Test_Load_invalid_topic_bridge_returns_error() {
	dir := s.T().TempDir()
	path := s.writeConfig(dir, `[{"type": "topic_bridge", "source": "not-an-object"}]`)
	s.T().Setenv("CELERITY_LOCAL_EVENTS_CONFIG_FILE", path)

	_, err := Load()
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "parsing topic bridge 0")
}

func (s *ConfigSuite) Test_Load_invalid_dynamodb_stream_returns_error() {
	dir := s.T().TempDir()
	path := s.writeConfig(dir, `[{"type": "dynamodb_stream", "source": "not-an-object"}]`)
	s.T().Setenv("CELERITY_LOCAL_EVENTS_CONFIG_FILE", path)

	_, err := Load()
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "parsing dynamodb stream bridge 0")
}

func (s *ConfigSuite) Test_Load_invalid_minio_notification_returns_error() {
	dir := s.T().TempDir()
	path := s.writeConfig(dir, `[{"type": "minio_notification", "source": "not-an-object"}]`)
	s.T().Setenv("CELERITY_LOCAL_EVENTS_CONFIG_FILE", path)

	_, err := Load()
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "parsing minio notification bridge 0")
}

// --- RedisURL tests ---

func (s *ConfigSuite) Test_RedisURL_default_when_empty() {
	s.T().Setenv("CELERITY_LOCAL_REDIS_URL", "")
	s.Assert().Equal("redis://127.0.0.1:6379", RedisURL())
}

func (s *ConfigSuite) Test_RedisURL_redis_scheme_passthrough() {
	s.T().Setenv("CELERITY_LOCAL_REDIS_URL", "redis://myhost:6380")
	s.Assert().Equal("redis://myhost:6380", RedisURL())
}

func (s *ConfigSuite) Test_RedisURL_custom_url_passthrough() {
	s.T().Setenv("CELERITY_LOCAL_REDIS_URL", "rediss://secure:6380")
	s.Assert().Equal("rediss://secure:6380", RedisURL())
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}
