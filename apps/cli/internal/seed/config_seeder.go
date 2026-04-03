package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// ConfigSeeder loads config and secrets YAML files into a Valkey store.
type ConfigSeeder interface {
	SeedConfig(ctx context.Context, storeID string, values map[string]string) error
	Close() error
}

// ConfigSeedResult tracks what was seeded for TUI reporting.
type ConfigSeedResult struct {
	StoreIDs []string
	Keys     int
}

// ConfigResourceInfo describes a config resource extracted from the blueprint.
type ConfigResourceInfo struct {
	ResourceName string
	StoreName    string
}

// ValkeyConfigSeeder implements ConfigSeeder using a Valkey (Redis-compatible) backend.
type ValkeyConfigSeeder struct {
	client *redis.Client
	logger *zap.Logger
}

// NewValkeyConfigSeeder creates a new Valkey config seeder.
// The endpoint must be a redis:// URL (e.g. "redis://localhost:6379")
// as produced by the compose generator's runtime env vars.
func NewValkeyConfigSeeder(endpoint string, logger *zap.Logger) (*ValkeyConfigSeeder, error) {
	opts, err := redis.ParseURL(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parsing redis endpoint %q: %w", endpoint, err)
	}

	return &ValkeyConfigSeeder{
		client: redis.NewClient(opts),
		logger: logger,
	}, nil
}

// SeedConfig merges a config map into the JSON-encoded value in Valkey.
// If the key already exists, new values are merged into the existing data
// (new keys are added, existing keys are overwritten). If the key does not
// exist, it is created with the provided values.
// The storeID is used as the Valkey key, matching the convention expected
// by the SDK config local backends.
func (s *ValkeyConfigSeeder) SeedConfig(ctx context.Context, storeID string, values map[string]string) error {
	merged := make(map[string]string)

	// Read existing data if present.
	existing, err := s.client.Get(ctx, storeID).Result()
	if err == nil && existing != "" {
		if jsonErr := json.Unmarshal([]byte(existing), &merged); jsonErr != nil {
			s.logger.Debug("existing config is not valid JSON, overwriting",
				zap.String("storeID", storeID),
			)
		}
	}

	maps.Copy(merged, values)

	data, err := json.Marshal(merged)
	if err != nil {
		return fmt.Errorf("marshalling config for store %s: %w", storeID, err)
	}

	if err := s.client.Set(ctx, storeID, string(data), 0).Err(); err != nil {
		return fmt.Errorf("setting config in Valkey for store %s: %w", storeID, err)
	}

	s.logger.Debug("seeded config store",
		zap.String("storeID", storeID),
		zap.Int("keys", len(merged)),
	)

	return nil
}

// Close closes the Valkey client connection.
func (s *ValkeyConfigSeeder) Close() error {
	return s.client.Close()
}

// CollectConfigResources extracts celerity/config resources from a blueprint
// and returns their resource names and spec.name values.
func CollectConfigResources(bp *schema.Blueprint) []ConfigResourceInfo {
	if bp.Resources == nil {
		return nil
	}

	var configs []ConfigResourceInfo
	for name, resource := range bp.Resources.Values {
		if resource.Type == nil || resource.Type.Value != "celerity/config" {
			continue
		}

		storeName := extractSpecStringField(resource, "name")
		if storeName == "" {
			storeName = name
		}

		configs = append(configs, ConfigResourceInfo{
			ResourceName: name,
			StoreName:    storeName,
		})
	}

	return configs
}

// LoadConfigYAML reads a YAML file containing flat key-value pairs and returns them as a map.
// Returns an empty map if the file does not exist.
func LoadConfigYAML(filePath string) (map[string]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("reading config file %s: %w", filePath, err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", filePath, err)
	}

	result := make(map[string]string, len(raw))
	for k, v := range raw {
		result[k] = fmt.Sprintf("%v", v)
	}

	return result, nil
}

// LoadAndMergeConfig reads config and secrets YAML files for a given store name,
// merges them (secrets override config), and returns the combined map.
func LoadAndMergeConfig(configDir string, secretsDir string, storeName string) (map[string]string, error) {
	merged := map[string]string{}

	if configDir != "" {
		configPath := filepath.Join(configDir, fmt.Sprintf("%s.yaml", storeName))
		configVals, err := LoadConfigYAML(configPath)
		if err != nil {
			return nil, err
		}
		maps.Copy(merged, configVals)
	}

	if secretsDir != "" {
		secretsPath := filepath.Join(secretsDir, fmt.Sprintf("%s.yaml", storeName))
		secretVals, err := LoadConfigYAML(secretsPath)
		if err != nil {
			return nil, err
		}
		maps.Copy(merged, secretVals)
	}

	return merged, nil
}

// ConfigStoreIDEnvVars generates the environment variables that tell the runtime
// where to find each config store in Valkey.
func ConfigStoreIDEnvVars(bp *schema.Blueprint) map[string]string {
	configs := CollectConfigResources(bp)
	if len(configs) == 0 {
		return nil
	}

	envVars := make(map[string]string, len(configs)+1)

	for _, cfg := range configs {
		upper := strings.ToUpper(cfg.StoreName)
		storeIDKey := fmt.Sprintf("CELERITY_CONFIG_%s_STORE_ID", upper)
		envVars[storeIDKey] = cfg.StoreName
		// Set the NAMESPACE so the SDK's config layer registers the namespace
		// under the original camelCase name (e.g. "appConfig") rather than the
		// lowercased env-var-derived name (e.g. "appconfig").
		// STORE_PREFIX is intentionally not set for local dev — each config
		// namespace has its own dedicated Valkey key so there is no shared
		// key space requiring prefix-based filtering. STORE_PREFIX is used
		// in deployed environments (e.g. AWS SSM Parameter Store) where keys
		// share a path prefix.
		configNamespaceKey := fmt.Sprintf("CELERITY_CONFIG_%s_NAMESPACE", upper)
		envVars[configNamespaceKey] = cfg.StoreName
	}

	return envVars
}

// ResourcesConfigStoreID is the Valkey key used for the "resources" config namespace.
// The SDK's resource layers (datastore, bucket, queue, topic, cache, sql-database)
// read connection credentials from this namespace.
const ResourcesConfigStoreID = "resources"

// ResourcesConfigStoreEnvVars returns the environment variables needed to register
// the "resources" config namespace in the SDK's ConfigService.
func ResourcesConfigStoreEnvVars() map[string]string {
	return map[string]string{
		"CELERITY_CONFIG_RESOURCES_STORE_ID": ResourcesConfigStoreID,
	}
}

// ResourceConfigValues generates all resource credential config keys for the
// "resources" config namespace. This includes cache, datastore, bucket, queue,
// topic, and SQL database resources.
func ResourceConfigValues(bp *schema.Blueprint) map[string]string {
	if bp.Resources == nil {
		return nil
	}

	values := map[string]string{}
	maps.Copy(values, cacheConfigValues(bp))
	maps.Copy(values, simpleResourceConfigValues(bp, "celerity/datastore"))
	maps.Copy(values, simpleResourceConfigValues(bp, "celerity/bucket"))
	maps.Copy(values, simpleResourceConfigValues(bp, "celerity/queue"))
	maps.Copy(values, simpleResourceConfigValues(bp, "celerity/topic"))
	maps.Copy(values, sqlDatabaseConfigValues(bp))

	if len(values) == 0 {
		return nil
	}
	return values
}

// cacheConfigValues generates credential config keys for celerity/cache resources.
// The SDK's CacheLayer reads {configKey}_host, _port, _authMode, _tls from the
// resources namespace.
func cacheConfigValues(bp *schema.Blueprint) map[string]string {
	values := map[string]string{}
	for _, resource := range bp.Resources.Values {
		if resource.Type == nil || resource.Type.Value != "celerity/cache" {
			continue
		}
		name := extractSpecStringField(resource, "name")
		if name == "" {
			continue
		}
		values[fmt.Sprintf("%s_host", name)] = "valkey"
		values[fmt.Sprintf("%s_port", name)] = "6379"
		values[fmt.Sprintf("%s_authMode", name)] = "password"
		values[fmt.Sprintf("%s_tls", name)] = "false"
		values[fmt.Sprintf("%s_clusterMode", name)] = "false"
	}
	return values
}

// simpleResourceConfigValues generates config keys for resources of the given type
// where the config key and value are both the resource's spec.name (or the resource
// name as fallback). Used for datastore, bucket, queue, and topic resources.
func simpleResourceConfigValues(bp *schema.Blueprint, resourceType string) map[string]string {
	values := map[string]string{}
	for resourceName, resource := range bp.Resources.Values {
		if resource.Type == nil || resource.Type.Value != resourceType {
			continue
		}
		name := extractSpecStringField(resource, "name")
		if name == "" {
			name = resourceName
		}
		values[name] = name
	}
	return values
}

// sqlDatabaseConfigValues generates the credential config keys for all
// celerity/sqlDatabase resources in the blueprint.
//
// For each database resource with spec.name "audit", generates:
//
//	audit_host, audit_port, audit_database, audit_user,
//	audit_password, audit_engine, audit_authMode, audit_ssl
func sqlDatabaseConfigValues(bp *schema.Blueprint) map[string]string {
	if bp.Resources == nil {
		return nil
	}

	values := map[string]string{}
	for _, resource := range bp.Resources.Values {
		if resource.Type == nil || resource.Type.Value != "celerity/sqlDatabase" {
			continue
		}

		dbName := extractSpecStringField(resource, "name")
		if dbName == "" {
			continue
		}
		engine := extractSpecStringField(resource, "engine")
		if engine == "" {
			engine = "postgres"
		}

		port := "5432"
		if engine == "mysql" {
			port = "3306"
		}

		values[fmt.Sprintf("%s_host", dbName)] = "sql-database"
		values[fmt.Sprintf("%s_port", dbName)] = port
		values[fmt.Sprintf("%s_database", dbName)] = dbName
		values[fmt.Sprintf("%s_user", dbName)] = "celerity"
		values[fmt.Sprintf("%s_password", dbName)] = "celerity"
		values[fmt.Sprintf("%s_engine", dbName)] = engine
		values[fmt.Sprintf("%s_authMode", dbName)] = "password"
		values[fmt.Sprintf("%s_ssl", dbName)] = "false"
	}

	if len(values) == 0 {
		return nil
	}
	return values
}
