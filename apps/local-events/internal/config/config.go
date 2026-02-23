package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Bridge is the decoded, type-safe representation of a bridge entry.
// Exactly one of the typed fields is populated based on Type.
type Bridge struct {
	Type              string
	Schedule          *ScheduleBridgeConfig
	TopicBridge       *TopicBridgeConfig
	DynamoDBStream    *DynamoDBStreamConfig
	MinIONotification *MinIONotificationConfig
}

// ScheduleBridgeConfig holds schedule trigger definitions.
type ScheduleBridgeConfig struct {
	Schedules []ScheduleEntry `json:"schedules"`
}

// ScheduleEntry describes a single schedule within a schedule bridge.
type ScheduleEntry struct {
	ID       string `json:"id"`
	Schedule string `json:"schedule"`
	Stream   string `json:"stream"`
	Input    any    `json:"input"`
}

// TopicBridgeConfig holds topic pub/sub fan-out configuration.
type TopicBridgeConfig struct {
	Source  TopicBridgeSource   `json:"source"`
	Targets []TopicBridgeTarget `json:"targets"`
}

// TopicBridgeSource identifies the Valkey pub/sub channel to subscribe to.
type TopicBridgeSource struct {
	Channel string `json:"channel"`
}

// TopicBridgeTarget identifies a Valkey stream to write each published message to.
type TopicBridgeTarget struct {
	Stream string `json:"stream"`
}

// DynamoDBStreamConfig holds configuration for polling DynamoDB Streams.
type DynamoDBStreamConfig struct {
	Source DynamoDBStreamSource `json:"source"`
	Target StreamTarget         `json:"target"`
}

// DynamoDBStreamSource identifies the DynamoDB Local table to poll for stream records.
type DynamoDBStreamSource struct {
	Endpoint  string `json:"endpoint"`
	Region    string `json:"region"`
	TableName string `json:"tableName"`
}

// MinIONotificationConfig holds configuration for listening to MinIO bucket notifications.
type MinIONotificationConfig struct {
	Source MinIONotificationSource `json:"source"`
	Target StreamTarget            `json:"target"`
}

// MinIONotificationSource identifies the MinIO bucket to listen for events on.
type MinIONotificationSource struct {
	Endpoint  string   `json:"endpoint"`
	AccessKey string   `json:"accessKey"`
	SecretKey string   `json:"secretKey"`
	Bucket    string   `json:"bucketName"`
	Events    []string `json:"events"`
}

// StreamTarget identifies a Valkey stream to write events to.
type StreamTarget struct {
	Stream string `json:"stream"`
}

const defaultConfigPath = "/etc/celerity/local-events-config.json"

// Load reads and decodes bridge configurations from a JSON file.
// Each entry is decoded in two phases: first the type field, then the
// full entry into the appropriate typed config struct.
func Load() ([]Bridge, error) {
	path := os.Getenv("CELERITY_LOCAL_EVENTS_CONFIG_FILE")
	if path == "" {
		path = defaultConfigPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	var rawEntries []json.RawMessage
	if err := json.Unmarshal(data, &rawEntries); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}

	bridges := make([]Bridge, 0, len(rawEntries))
	for i, raw := range rawEntries {
		var header struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &header); err != nil {
			return nil, fmt.Errorf("parsing bridge %d type: %w", i, err)
		}

		b := Bridge{Type: header.Type}
		switch header.Type {
		case "schedule":
			var cfg ScheduleBridgeConfig
			if err := json.Unmarshal(raw, &cfg); err != nil {
				return nil, fmt.Errorf("parsing schedule bridge %d: %w", i, err)
			}
			b.Schedule = &cfg

		case "topic_bridge":
			var cfg TopicBridgeConfig
			if err := json.Unmarshal(raw, &cfg); err != nil {
				return nil, fmt.Errorf("parsing topic bridge %d: %w", i, err)
			}
			b.TopicBridge = &cfg

		case "dynamodb_stream":
			var cfg DynamoDBStreamConfig
			if err := json.Unmarshal(raw, &cfg); err != nil {
				return nil, fmt.Errorf("parsing dynamodb stream bridge %d: %w", i, err)
			}
			b.DynamoDBStream = &cfg

		case "minio_notification":
			var cfg MinIONotificationConfig
			if err := json.Unmarshal(raw, &cfg); err != nil {
				return nil, fmt.Errorf("parsing minio notification bridge %d: %w", i, err)
			}
			b.MinIONotification = &cfg

		default:
			// Unknown types are preserved so main.go can log a warning.
		}

		bridges = append(bridges, b)
	}

	return bridges, nil
}

// RedisURL returns the Valkey/Redis connection URL from the environment.
func RedisURL() string {
	url := os.Getenv("CELERITY_LOCAL_REDIS_URL")
	if url == "" {
		return "redis://127.0.0.1:6379"
	}
	return url
}
