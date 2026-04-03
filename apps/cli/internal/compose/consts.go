package compose

// Resource type constants matching the blueprint spec.
const (
	ResourceTypeDatastore   = "celerity/datastore"
	ResourceTypeBucket      = "celerity/bucket"
	ResourceTypeQueue       = "celerity/queue"
	ResourceTypeTopic       = "celerity/topic"
	ResourceTypeConfig      = "celerity/config"
	ResourceTypeCache       = "celerity/cache"
	ResourceTypeSqlDatabase = "celerity/sqlDatabase"
	ResourceTypeVPC         = "celerity/vpc"
	ResourceTypeSchedule    = "celerity/schedule"
	ResourceTypeConsumer    = "celerity/consumer"
	ResourceTypeAPI         = "celerity/api"
)

// Deploy target constants.
const (
	DeployTargetAWS              = "aws"
	DeployTargetAWSServerless    = "aws-serverless"
	DeployTargetGCloud           = "gcloud"
	DeployTargetGCloudServerless = "gcloud-serverless"
	DeployTargetAzure            = "azure"
	DeployTargetAzureServerless  = "azure-serverless"
)

// Default images and ports for local emulators.
const (
	valkeyImage  = "valkey/valkey:8-alpine"
	valkeyPort   = "6379"
	minioImage   = "minio/minio:RELEASE.2025-09-07T16-13-09Z"
	minioPort    = "9000"
	minioAPIPort = "9001"

	postgresImage = "postgres:17-alpine"
	postgresPort  = "5432"

	dynamoDBLocalImage = "amazon/dynamodb-local:3.3.0"
	dynamoDBLocalPort  = "8000"

	localEventsImageRepo    = "ghcr.io/newstack-cloud/celerity-local-events"
	localEventsImageVersion = "0.4.1"
	localEventsImage        = localEventsImageRepo + ":" + localEventsImageVersion

	devAuthImageRepo    = "ghcr.io/newstack-cloud/celerity-dev-auth"
	devAuthImageVersion = "0.2.0"
	devAuthImage        = devAuthImageRepo + ":" + devAuthImageVersion
	devAuthPort         = "9099"

	mountebankImage   = "bbyars/mountebank:2.9.3"
	mountebankAPIPort = "2525"
)

// ServiceName constants for compose services.
// Used by both the compose generator and the seed package for host endpoint mapping.
const (
	ServiceNameDatastore   = "datastore"
	ServiceNameSqlDatabase = "sql-database"
	ServiceNameStorage     = "storage"
	ServiceNameValkey      = "valkey"
	ServiceNameLocalEvents = "local-events"
	ServiceNameDevAuth     = "dev-auth"
	ServiceNameStubs       = "stubs"
)

// Runtime environment variable names set by compose services.
const (
	EnvDatastoreEndpoint   = "CELERITY_LOCAL_DATASTORE_ENDPOINT"
	EnvSqlDatabaseEndpoint = "CELERITY_LOCAL_SQL_DATABASE_ENDPOINT"
	EnvBucketEndpoint      = "CELERITY_LOCAL_BUCKET_ENDPOINT"
	EnvBucketAccessKey     = "CELERITY_LOCAL_BUCKET_ACCESS_KEY"
	EnvBucketSecretKey     = "CELERITY_LOCAL_BUCKET_SECRET_KEY"
	EnvQueueEndpoint       = "CELERITY_LOCAL_QUEUE_ENDPOINT"
	EnvTopicEndpoint       = "CELERITY_LOCAL_TOPIC_ENDPOINT"
	EnvConfigEndpoint      = "CELERITY_LOCAL_CONFIG_ENDPOINT"
	EnvCacheEndpoint       = "CELERITY_LOCAL_CACHE_ENDPOINT"
	EnvDevAuthBaseURL      = "CELERITY_DEV_AUTH_BASE_URL"
	EnvStubsAPIURL         = "CELERITY_STUBS_API_URL"
)

var defaultPostgresCreds = struct {
	User     string
	Password string
	Database string
}{
	User:     "celerity",
	Password: "celerity",
	Database: "celerity",
}

var defaultMinioCreds = struct {
	AccessKey string
	SecretKey string
}{
	AccessKey: "minioadmin",
	SecretKey: "minioadmin",
}

// valkeyResourceTypes are the resource types that share a single Valkey instance.
var valkeyResourceTypes = map[string]string{
	ResourceTypeQueue:  EnvQueueEndpoint,
	ResourceTypeTopic:  EnvTopicEndpoint,
	ResourceTypeConfig: EnvConfigEndpoint,
	ResourceTypeCache:  EnvCacheEndpoint,
}
