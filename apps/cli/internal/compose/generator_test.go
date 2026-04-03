package compose

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type GenerateComposeConfigTestSuite struct {
	suite.Suite
	logger *zap.Logger
}

func (s *GenerateComposeConfigTestSuite) SetupTest() {
	logger, _ := zap.NewDevelopment()
	s.logger = logger
}

func (s *GenerateComposeConfigTestSuite) loadBlueprint(yamlContent string) *schema.Blueprint {
	bp, err := schema.LoadString(yamlContent, schema.YAMLSpecFormat)
	s.Require().NoError(err, "failed to load test blueprint")
	return bp
}

func (s *GenerateComposeConfigTestSuite) Test_empty_blueprint_produces_no_services() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources: {}
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)
	s.Assert().Empty(cfg.Services)
	s.Assert().Empty(cfg.RuntimeEnvVars)
}

func (s *GenerateComposeConfigTestSuite) Test_datastore_aws_creates_dynamodb_local_service() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)

	ds, ok := cfg.Services[ServiceNameDatastore]
	s.Require().True(ok, "expected datastore service")
	s.Assert().Equal(dynamoDBLocalImage, ds.Image)
	s.Assert().NotNil(ds.HealthCheck)

	endpoint, ok := cfg.RuntimeEnvVars[EnvDatastoreEndpoint]
	s.Require().True(ok, "expected CELERITY_LOCAL_DATASTORE_ENDPOINT")
	s.Assert().Equal("http://datastore:8000", endpoint)
}

func (s *GenerateComposeConfigTestSuite) Test_datastore_gcloud_returns_unsupported_error() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
`)
	appDir := s.T().TempDir()
	_, err := GenerateComposeConfig(bp, DeployTargetGCloud, "test-project", appDir, 0, true, s.logger)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "not yet supported")
}

func (s *GenerateComposeConfigTestSuite) Test_datastore_azure_returns_unsupported_error() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
`)
	appDir := s.T().TempDir()
	_, err := GenerateComposeConfig(bp, DeployTargetAzure, "test-project", appDir, 0, true, s.logger)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "not yet supported")
}

func (s *GenerateComposeConfigTestSuite) Test_datastore_unknown_target_returns_error() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
`)
	appDir := s.T().TempDir()
	_, err := GenerateComposeConfig(bp, "custom-platform", "test-project", appDir, 0, true, s.logger)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "unsupported deploy target")
}

func (s *GenerateComposeConfigTestSuite) Test_host_env_vars_rewrite_all_service_hostnames() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
  myBucket:
    type: "celerity/bucket"
    spec:
      name: files
  auditDb:
    type: "celerity/sqlDatabase"
    spec:
      name: audit
  myQueue:
    type: "celerity/queue"
    spec:
      name: events
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)

	// URL-type host env vars should use localhost, not Docker service names.
	for k, v := range cfg.HostEnvVars {
		if strings.Contains(v, "://") || strings.Contains(v, "@") {
			s.Assert().Contains(v, "localhost", "expected localhost in HostEnvVars[%s]=%s", k, v)
			s.Assert().NotContains(v, "://datastore:", "HostEnvVars should not contain Docker service names")
			s.Assert().NotContains(v, "://storage:", "HostEnvVars should not contain Docker service names")
		}
	}
}

func (s *GenerateComposeConfigTestSuite) Test_aws_serverless_target_uses_dynamodb() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWSServerless, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)

	_, ok := cfg.Services[ServiceNameDatastore]
	s.Assert().True(ok, "aws-serverless should still use DynamoDB Local")
}

func (s *GenerateComposeConfigTestSuite) Test_bucket_creates_minio_service() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myBucket:
    type: "celerity/bucket"
    spec:
      name: my-bucket
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)

	storage, ok := cfg.Services[ServiceNameStorage]
	s.Require().True(ok, "expected storage service")
	s.Assert().Equal(minioImage, storage.Image)

	s.Assert().Contains(cfg.RuntimeEnvVars, EnvBucketEndpoint)
	s.Assert().Contains(cfg.RuntimeEnvVars, EnvBucketAccessKey)
	s.Assert().Contains(cfg.RuntimeEnvVars, EnvBucketSecretKey)
}

func (s *GenerateComposeConfigTestSuite) Test_valkey_shared_across_queue_and_cache() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myQueue:
    type: "celerity/queue"
    spec:
      name: order-events
  myCache:
    type: "celerity/cache"
    spec:
      name: sessions
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)

	// Single Valkey service even though two resource types need it.
	s.Assert().Len(cfg.Services, 1)

	valkey, ok := cfg.Services[ServiceNameValkey]
	s.Require().True(ok, "expected valkey service")
	s.Assert().Equal(valkeyImage, valkey.Image)

	s.Assert().Contains(cfg.RuntimeEnvVars, EnvQueueEndpoint)
	s.Assert().Contains(cfg.RuntimeEnvVars, EnvCacheEndpoint)
}

func (s *GenerateComposeConfigTestSuite) Test_sql_database_creates_postgres_service() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  auditDb:
    type: "celerity/sqlDatabase"
    spec:
      engine: postgres
      name: audit
      schemaPath: "./schemas/audit-db.yaml"
      migrationsPath: "./sql/audit-db"
      authMode: password
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)

	pg, ok := cfg.Services[ServiceNameSqlDatabase]
	s.Require().True(ok, "expected sql-database service")
	s.Assert().Equal(postgresImage, pg.Image)
	s.Assert().Equal([]string{postgresPort + ":" + postgresPort}, pg.Ports)
	s.Assert().Equal(defaultPostgresCreds.User, pg.Environment["POSTGRES_USER"])
	s.Assert().Equal(defaultPostgresCreds.Password, pg.Environment["POSTGRES_PASSWORD"])
	s.Assert().Equal(defaultPostgresCreds.Database, pg.Environment["POSTGRES_DB"])
	s.Assert().NotNil(pg.HealthCheck)
	s.Assert().Equal([]string{"CMD-SHELL", "pg_isready -U " + defaultPostgresCreds.User}, pg.HealthCheck.Test)

	endpoint, ok := cfg.RuntimeEnvVars[EnvSqlDatabaseEndpoint]
	s.Require().True(ok, "expected CELERITY_LOCAL_SQL_DATABASE_ENDPOINT")
	s.Assert().Equal(
		"postgres://celerity:celerity@sql-database:5432/celerity?sslmode=disable",
		endpoint,
	)
}

func (s *GenerateComposeConfigTestSuite) Test_all_resource_types_produce_four_services() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
  myBucket:
    type: "celerity/bucket"
    spec:
      name: my-bucket
  myQueue:
    type: "celerity/queue"
    spec:
      name: events
  myTopic:
    type: "celerity/topic"
    spec:
      name: notifications
  myConfig:
    type: "celerity/config"
    spec:
      name: app-config
  myCache:
    type: "celerity/cache"
    spec:
      name: sessions
  myVpc:
    type: "celerity/vpc"
    spec:
      name: main-vpc
  auditDb:
    type: "celerity/sqlDatabase"
    spec:
      engine: postgres
      name: audit
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)

	// 4 services: datastore, storage, valkey (shared), sql-database.
	s.Assert().Len(cfg.Services, 4)
	s.Assert().Contains(cfg.Services, ServiceNameDatastore)
	s.Assert().Contains(cfg.Services, ServiceNameStorage)
	s.Assert().Contains(cfg.Services, ServiceNameValkey)
	s.Assert().Contains(cfg.Services, ServiceNameSqlDatabase)
}

func (s *GenerateComposeConfigTestSuite) Test_writes_valid_compose_yaml_file() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myQueue:
    type: "celerity/queue"
    spec:
      name: events
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)

	expectedPath := filepath.Join(appDir, ".celerity", "compose.generated.yaml")
	s.Assert().Equal(expectedPath, cfg.FilePath)

	data, err := os.ReadFile(expectedPath)
	s.Require().NoError(err, "compose file not written")

	var parsed composeFile
	s.Require().NoError(yaml.Unmarshal(data, &parsed))
	s.Assert().Contains(parsed.Services, ServiceNameValkey)
}

func (s *GenerateComposeConfigTestSuite) Test_vpc_only_blueprint_produces_no_services() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myVpc:
    type: "celerity/vpc"
    spec:
      name: main-vpc
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)
	s.Assert().Empty(cfg.Services)
}

func (s *GenerateComposeConfigTestSuite) parseBridgeConfigs(appDir string) []bridgeConfig {
	path := filepath.Join(appDir, ".celerity", "local-events-config.json")
	data, err := os.ReadFile(path)
	s.Require().NoError(err, "local-events-config.json not written")
	var bridges []bridgeConfig
	s.Require().NoError(json.Unmarshal(data, &bridges))
	return bridges
}

func findBridgeByType(bridges []bridgeConfig, t string) *bridgeConfig {
	for i := range bridges {
		if bridges[i].Type == t {
			return &bridges[i]
		}
	}
	return nil
}

func (s *GenerateComposeConfigTestSuite) Test_schedule_bridge_creates_local_events_service() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myTopic:
    type: "celerity/topic"
    spec:
      name: notifications
  dailySync:
    type: "celerity/schedule"
    linkSelector:
      byLabel:
        handler: sync
    spec:
      schedule: "rate(1d)"
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)

	le, ok := cfg.Services[ServiceNameLocalEvents]
	s.Require().True(ok, "expected local-events service")
	s.Assert().Equal(localEventsImage, le.Image)
	s.Assert().Contains(le.DependsOn, ServiceNameValkey)
	s.Assert().NotContains(le.DependsOn, ServiceNameDatastore)
	s.Assert().NotContains(le.DependsOn, ServiceNameStorage)
	s.Assert().Equal("redis://"+ServiceNameValkey+":"+valkeyPort, le.Environment["CELERITY_LOCAL_REDIS_URL"])

	bridges := s.parseBridgeConfigs(appDir)
	s.Require().Len(bridges, 1)
	s.Assert().Equal("schedule", bridges[0].Type)
	s.Require().Len(bridges[0].Schedules, 1)
	s.Assert().Equal("dailySync", bridges[0].Schedules[0].ID)
	s.Assert().Equal("rate(1d)", bridges[0].Schedules[0].Schedule)
	s.Assert().Equal("celerity:schedules:dailySync", bridges[0].Schedules[0].Stream)
}

func (s *GenerateComposeConfigTestSuite) Test_topic_bridge_for_topic_consumers() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myTopic:
    type: "celerity/topic"
    spec:
      name: notifications
  notificationConsumer:
    type: "celerity/consumer"
    linkSelector:
      byLabel:
        handler: notify
    spec:
      sourceId: "celerity::topic::notifications"
  emailConsumer:
    type: "celerity/consumer"
    linkSelector:
      byLabel:
        handler: email
    spec:
      sourceId: "celerity::topic::notifications"
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)
	s.Assert().Contains(cfg.Services, ServiceNameLocalEvents)

	bridges := s.parseBridgeConfigs(appDir)
	tb := findBridgeByType(bridges, "topic_bridge")
	s.Require().NotNil(tb, "expected topic_bridge")

	sourceMap, ok := tb.Source.(map[string]any)
	s.Require().True(ok, "expected source to be a map")
	s.Assert().Equal("celerity:topic:channel:notifications", sourceMap["channel"])
	s.Require().Len(tb.Targets, 2)

	targetStreams := []string{tb.Targets[0].Stream, tb.Targets[1].Stream}
	s.Assert().Contains(targetStreams, "celerity:topic:notifications:notificationConsumer")
	s.Assert().Contains(targetStreams, "celerity:topic:notifications:emailConsumer")
}

func (s *GenerateComposeConfigTestSuite) Test_dynamodb_stream_bridge_for_linked_consumer() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  ordersTable:
    type: "celerity/datastore"
    linkSelector:
      byLabel:
        app: orders
    spec:
      name: orders
      keys:
        partitionKey: orderId
      schema:
        fields:
          orderId: string
  ordersConsumer:
    type: "celerity/consumer"
    metadata:
      labels:
        app: orders
    linkSelector:
      byLabel:
        handler: processOrder
    spec:
      batchSize: 25
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)

	s.Assert().Contains(cfg.Services, ServiceNameLocalEvents)
	s.Assert().Contains(cfg.Services, ServiceNameDatastore)

	le := cfg.Services[ServiceNameLocalEvents]
	s.Assert().Contains(le.DependsOn, ServiceNameDatastore)
	s.Assert().Contains(le.DependsOn, ServiceNameValkey)

	bridges := s.parseBridgeConfigs(appDir)
	db := findBridgeByType(bridges, "dynamodb_stream")
	s.Require().NotNil(db, "expected dynamodb_stream bridge")

	sourceMap, ok := db.Source.(map[string]any)
	s.Require().True(ok)
	s.Assert().Equal("http://"+ServiceNameDatastore+":"+dynamoDBLocalPort, sourceMap["endpoint"])
	s.Assert().Equal("local", sourceMap["region"])
	s.Assert().Equal("orders", sourceMap["tableName"])

	targetMap, ok := db.Target.(map[string]any)
	s.Require().True(ok)
	s.Assert().Equal("celerity:datastore:ordersTable", targetMap["stream"])

	s.Require().Contains(cfg.StreamEnabledTables, "ordersTable")
	s.Assert().True(cfg.StreamEnabledTables["ordersTable"])
}

func (s *GenerateComposeConfigTestSuite) Test_minio_notification_bridge_for_linked_consumer() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  uploadsBucket:
    type: "celerity/bucket"
    linkSelector:
      byLabel:
        app: uploads
    spec:
      name: uploads
  uploadsConsumer:
    type: "celerity/consumer"
    metadata:
      labels:
        app: uploads
      annotations:
        celerity.consumer.bucket.events: "created"
    linkSelector:
      byLabel:
        handler: processUpload
    spec:
      batchSize: 10
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)

	s.Assert().Contains(cfg.Services, ServiceNameLocalEvents)
	s.Assert().Contains(cfg.Services, ServiceNameStorage)

	le := cfg.Services[ServiceNameLocalEvents]
	s.Assert().Contains(le.DependsOn, ServiceNameStorage)

	bridges := s.parseBridgeConfigs(appDir)
	mn := findBridgeByType(bridges, "minio_notification")
	s.Require().NotNil(mn, "expected minio_notification bridge")

	sourceMap, ok := mn.Source.(map[string]any)
	s.Require().True(ok)
	s.Assert().Equal("http://"+ServiceNameStorage+":"+minioPort, sourceMap["endpoint"])
	s.Assert().Equal(defaultMinioCreds.AccessKey, sourceMap["accessKey"])
	s.Assert().Equal(defaultMinioCreds.SecretKey, sourceMap["secretKey"])
	s.Assert().Equal("uploads", sourceMap["bucketName"])

	events, ok := sourceMap["events"].([]any)
	s.Require().True(ok)
	s.Assert().Contains(events, "s3:ObjectCreated:*")

	targetMap, ok := mn.Target.(map[string]any)
	s.Require().True(ok)
	s.Assert().Equal("celerity:bucket:uploadsBucket", targetMap["stream"])
}

func (s *GenerateComposeConfigTestSuite) Test_minio_bridge_defaults_events_when_no_annotation() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  assetsBucket:
    type: "celerity/bucket"
    linkSelector:
      byLabel:
        app: assets
    spec:
      name: assets
  assetsConsumer:
    type: "celerity/consumer"
    metadata:
      labels:
        app: assets
    linkSelector:
      byLabel:
        handler: processAsset
    spec:
      batchSize: 5
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)
	s.Assert().Contains(cfg.Services, ServiceNameLocalEvents)

	bridges := s.parseBridgeConfigs(appDir)
	mn := findBridgeByType(bridges, "minio_notification")
	s.Require().NotNil(mn)

	sourceMap, ok := mn.Source.(map[string]any)
	s.Require().True(ok)
	events, ok := sourceMap["events"].([]any)
	s.Require().True(ok)
	s.Assert().Len(events, 2)
	s.Assert().Contains(events, "s3:ObjectCreated:*")
	s.Assert().Contains(events, "s3:ObjectRemoved:*")
}

func (s *GenerateComposeConfigTestSuite) Test_combined_blueprint_all_bridge_types() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myTopic:
    type: "celerity/topic"
    spec:
      name: notifications
  ordersTable:
    type: "celerity/datastore"
    linkSelector:
      byLabel:
        app: orders
    spec:
      name: orders
      keys:
        partitionKey: orderId
      schema:
        fields:
          orderId: string
  uploadsBucket:
    type: "celerity/bucket"
    linkSelector:
      byLabel:
        app: uploads
    spec:
      name: uploads
  dailySync:
    type: "celerity/schedule"
    linkSelector:
      byLabel:
        handler: sync
    spec:
      schedule: "cron(0 0 * * ? *)"
  topicConsumer:
    type: "celerity/consumer"
    linkSelector:
      byLabel:
        handler: notify
    spec:
      sourceId: "celerity::topic::notifications"
  ordersConsumer:
    type: "celerity/consumer"
    metadata:
      labels:
        app: orders
    linkSelector:
      byLabel:
        handler: processOrder
    spec:
      batchSize: 25
  uploadsConsumer:
    type: "celerity/consumer"
    metadata:
      labels:
        app: uploads
      annotations:
        celerity.consumer.bucket.events: "created,deleted"
    linkSelector:
      byLabel:
        handler: processUpload
    spec:
      batchSize: 10
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)

	s.Assert().Contains(cfg.Services, ServiceNameDatastore)
	s.Assert().Contains(cfg.Services, ServiceNameStorage)
	s.Assert().Contains(cfg.Services, ServiceNameValkey)
	s.Assert().Contains(cfg.Services, ServiceNameLocalEvents)

	le := cfg.Services[ServiceNameLocalEvents]
	s.Assert().Len(le.DependsOn, 3)
	s.Assert().Contains(le.DependsOn, ServiceNameValkey)
	s.Assert().Contains(le.DependsOn, ServiceNameDatastore)
	s.Assert().Contains(le.DependsOn, ServiceNameStorage)

	bridges := s.parseBridgeConfigs(appDir)
	s.Assert().NotNil(findBridgeByType(bridges, "schedule"))
	s.Assert().NotNil(findBridgeByType(bridges, "topic_bridge"))
	s.Assert().NotNil(findBridgeByType(bridges, "dynamodb_stream"))
	s.Assert().NotNil(findBridgeByType(bridges, "minio_notification"))

	s.Assert().Contains(cfg.StreamEnabledTables, "ordersTable")
}

func (s *GenerateComposeConfigTestSuite) Test_no_local_events_when_no_bridges() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
  myBucket:
    type: "celerity/bucket"
    spec:
      name: my-bucket
  myQueue:
    type: "celerity/queue"
    spec:
      name: events
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)

	s.Assert().NotContains(cfg.Services, ServiceNameLocalEvents)
	s.Assert().Contains(cfg.Services, ServiceNameDatastore)
	s.Assert().Contains(cfg.Services, ServiceNameStorage)
	s.Assert().Contains(cfg.Services, ServiceNameValkey)
	s.Assert().Empty(cfg.StreamEnabledTables)

	configPath := filepath.Join(appDir, ".celerity", "local-events-config.json")
	_, err = os.Stat(configPath)
	s.Assert().True(os.IsNotExist(err), "config file should not exist when no bridges")
}

func (s *GenerateComposeConfigTestSuite) Test_stream_enabled_tables_only_for_linked_datastores() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  ordersTable:
    type: "celerity/datastore"
    linkSelector:
      byLabel:
        app: orders
    spec:
      name: orders
      keys:
        partitionKey: orderId
      schema:
        fields:
          orderId: string
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
  ordersConsumer:
    type: "celerity/consumer"
    metadata:
      labels:
        app: orders
    linkSelector:
      byLabel:
        handler: processOrder
    spec:
      batchSize: 25
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)

	s.Assert().Len(cfg.StreamEnabledTables, 1)
	s.Assert().True(cfg.StreamEnabledTables["ordersTable"])
	s.Assert().False(cfg.StreamEnabledTables["usersTable"])
}

func (s *GenerateComposeConfigTestSuite) Test_dynamodb_stream_bridge_works_for_aws_serverless() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  ordersTable:
    type: "celerity/datastore"
    linkSelector:
      byLabel:
        app: orders
    spec:
      name: orders
      keys:
        partitionKey: orderId
      schema:
        fields:
          orderId: string
  ordersConsumer:
    type: "celerity/consumer"
    metadata:
      labels:
        app: orders
    linkSelector:
      byLabel:
        handler: processOrder
    spec:
      batchSize: 25
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWSServerless, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)

	s.Assert().Contains(cfg.Services, ServiceNameLocalEvents)
	bridges := s.parseBridgeConfigs(appDir)
	db := findBridgeByType(bridges, "dynamodb_stream")
	s.Require().NotNil(db, "expected dynamodb_stream bridge for aws-serverless")
	s.Assert().Contains(cfg.StreamEnabledTables, "ordersTable")
}

func (s *GenerateComposeConfigTestSuite) Test_consumer_with_no_matching_labels_no_bridges() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  ordersTable:
    type: "celerity/datastore"
    linkSelector:
      byLabel:
        app: orders
    spec:
      name: orders
      keys:
        partitionKey: orderId
      schema:
        fields:
          orderId: string
  strayConsumer:
    type: "celerity/consumer"
    metadata:
      labels:
        app: payments
    linkSelector:
      byLabel:
        handler: processPayment
    spec:
      batchSize: 10
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)

	s.Assert().NotContains(cfg.Services, ServiceNameLocalEvents)
	s.Assert().Empty(cfg.StreamEnabledTables)
}

func (s *GenerateComposeConfigTestSuite) Test_minio_bridge_maps_abstract_event_names_to_s3() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  docsBucket:
    type: "celerity/bucket"
    linkSelector:
      byLabel:
        app: docs
    spec:
      name: docs
  docsConsumer:
    type: "celerity/consumer"
    metadata:
      labels:
        app: docs
      annotations:
        celerity.consumer.bucket.events: "created,deleted,metadataUpdated"
    linkSelector:
      byLabel:
        handler: processDocs
    spec:
      batchSize: 5
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)
	s.Assert().Contains(cfg.Services, ServiceNameLocalEvents)

	bridges := s.parseBridgeConfigs(appDir)
	mn := findBridgeByType(bridges, "minio_notification")
	s.Require().NotNil(mn)

	sourceMap, ok := mn.Source.(map[string]any)
	s.Require().True(ok)
	events, ok := sourceMap["events"].([]any)
	s.Require().True(ok)
	s.Assert().Len(events, 3)
	s.Assert().Contains(events, "s3:ObjectCreated:*")
	s.Assert().Contains(events, "s3:ObjectRemoved:*")
	s.Assert().Contains(events, "s3:ObjectTagging:*")
}

func (s *GenerateComposeConfigTestSuite) Test_minio_bridge_passes_through_s3_format_events() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  legacyBucket:
    type: "celerity/bucket"
    linkSelector:
      byLabel:
        app: legacy
    spec:
      name: legacy
  legacyConsumer:
    type: "celerity/consumer"
    metadata:
      labels:
        app: legacy
      annotations:
        celerity.consumer.bucket.events: "s3:ObjectCreated:Put,s3:ObjectRemoved:Delete"
    linkSelector:
      byLabel:
        handler: processLegacy
    spec:
      batchSize: 5
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)
	s.Assert().Contains(cfg.Services, ServiceNameLocalEvents)

	bridges := s.parseBridgeConfigs(appDir)
	mn := findBridgeByType(bridges, "minio_notification")
	s.Require().NotNil(mn)

	sourceMap, ok := mn.Source.(map[string]any)
	s.Require().True(ok)
	events, ok := sourceMap["events"].([]any)
	s.Require().True(ok)
	s.Assert().Len(events, 2)
	s.Assert().Contains(events, "s3:ObjectCreated:Put")
	s.Assert().Contains(events, "s3:ObjectRemoved:Delete")
}

func Test_mapBucketEventToS3(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"created", "s3:ObjectCreated:*"},
		{"deleted", "s3:ObjectRemoved:*"},
		{"metadataUpdated", "s3:ObjectTagging:*"},
		{"s3:ObjectCreated:Put", "s3:ObjectCreated:Put"},
		{"s3:ObjectRemoved:*", "s3:ObjectRemoved:*"},
		{"unknown", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapBucketEventToS3(tt.input)
			if result != tt.expected {
				t.Errorf("mapBucketEventToS3(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func (s *GenerateComposeConfigTestSuite) Test_jwt_auth_creates_dev_auth_service() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myApi:
    type: "celerity/api"
    spec:
      protocols:
        - http
      auth:
        guards:
          myJwtGuard:
            type: jwt
            issuer: "https://auth.example.com"
            audience: "my-app"
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)

	devAuth, ok := cfg.Services[ServiceNameDevAuth]
	s.Require().True(ok, "expected dev-auth service")
	s.Assert().Equal(devAuthImage, devAuth.Image)
	s.Assert().Equal("my-app", devAuth.Environment["DEV_AUTH_AUDIENCE"])
	s.Assert().Contains(devAuth.Environment["DEV_AUTH_ISSUER"], "host.docker.internal")
	s.Assert().Contains(cfg.RuntimeEnvVars, EnvDevAuthBaseURL)
}

func (s *GenerateComposeConfigTestSuite) Test_jwt_auth_default_audience_when_none_set() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myApi:
    type: "celerity/api"
    spec:
      protocols:
        - http
      auth:
        guards:
          myJwtGuard:
            type: jwt
            issuer: "https://auth.example.com"
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)

	devAuth := cfg.Services[ServiceNameDevAuth]
	s.Assert().Equal("celerity-test-app", devAuth.Environment["DEV_AUTH_AUDIENCE"])
}

func (s *GenerateComposeConfigTestSuite) Test_no_jwt_auth_skips_dev_auth_service() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myApi:
    type: "celerity/api"
    spec:
      protocols:
        - http
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)
	s.Assert().NotContains(cfg.Services, ServiceNameDevAuth)
}

func (s *GenerateComposeConfigTestSuite) Test_port_offset_applies_to_services() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 100, true, s.logger)
	s.Require().NoError(err)

	ds := cfg.Services[ServiceNameDatastore]
	// With offset 100, host port should be 8100 (8000 + 100)
	s.Assert().Equal("8100:8000", ds.Ports[0])

	// Host env vars should use localhost with offset ports.
	s.Assert().Contains(cfg.HostEnvVars[EnvDatastoreEndpoint], "localhost:8100")
}

func (s *GenerateComposeConfigTestSuite) Test_stubs_service_added_when_stubs_dir_exists() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myQueue:
    type: "celerity/queue"
    spec:
      name: events
`)
	appDir := s.T().TempDir()

	// Create stubs directory with a service.
	svcDir := filepath.Join(appDir, "stubs", "payments")
	s.Require().NoError(os.MkdirAll(svcDir, 0o755))
	s.Require().NoError(os.WriteFile(
		filepath.Join(svcDir, "service.yaml"),
		[]byte("port: 9001\nconfigKey: payments_url\n"),
		0o644,
	))
	s.Require().NoError(os.WriteFile(
		filepath.Join(svcDir, "get.yaml"),
		[]byte("endpoint:\n  method: GET\n  path: /\nstubs:\n  - responses:\n      - is:\n          statusCode: 200\n"),
		0o644,
	))

	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)

	stubs, ok := cfg.Services[ServiceNameStubs]
	s.Require().True(ok, "expected stubs service")
	s.Assert().Equal(mountebankImage, stubs.Image)
	s.Assert().Contains(cfg.RuntimeEnvVars, EnvStubsAPIURL)
	s.Assert().Contains(cfg.RuntimeEnvVars, "CELERITY_STUB_PAYMENTS_URL")
	s.Require().Len(cfg.StubServices, 1)
	s.Assert().Equal("payments", cfg.StubServices[0].Name)
	s.Assert().Equal(9001, cfg.StubServices[0].Port)
	s.Assert().Equal("payments_url", cfg.StubServices[0].ConfigKey)
}

func (s *GenerateComposeConfigTestSuite) Test_no_stubs_when_stubs_dir_absent() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myQueue:
    type: "celerity/queue"
    spec:
      name: events
`)
	appDir := s.T().TempDir()
	cfg, err := GenerateComposeConfig(bp, DeployTargetAWS, "test-project", appDir, 0, true, s.logger)
	s.Require().NoError(err)
	s.Assert().NotContains(cfg.Services, ServiceNameStubs)
}

func TestGenerateComposeConfigTestSuite(t *testing.T) {
	suite.Run(t, new(GenerateComposeConfigTestSuite))
}
