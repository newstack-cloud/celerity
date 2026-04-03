package seed

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/newstack-cloud/celerity/apps/cli/internal/testutils"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type ValkeyIntegrationSuite struct {
	suite.Suite
	endpoint string
	logger   *zap.Logger
}

func TestValkeyIntegrationSuite(t *testing.T) {
	suite.Run(t, new(ValkeyIntegrationSuite))
}

func (s *ValkeyIntegrationSuite) SetupTest() {
	s.endpoint = testutils.RequireEnv(s.T(), "CELERITY_TEST_VALKEY_ENDPOINT")
	logger, _ := zap.NewDevelopment()
	s.logger = logger
}

func (s *ValkeyIntegrationSuite) redisURL() string {
	return "redis://" + s.endpoint
}

func (s *ValkeyIntegrationSuite) cleanKey(key string) {
	opts, _ := redis.ParseURL(s.redisURL())
	client := redis.NewClient(opts)
	defer client.Close()
	client.Del(context.Background(), key)
}

func (s *ValkeyIntegrationSuite) readKey(key string) map[string]string {
	opts, _ := redis.ParseURL(s.redisURL())
	client := redis.NewClient(opts)
	defer client.Close()

	val, err := client.Get(context.Background(), key).Result()
	s.Require().NoError(err)

	var result map[string]string
	s.Require().NoError(json.Unmarshal([]byte(val), &result))
	return result
}

func (s *ValkeyIntegrationSuite) Test_seed_config_writes_values() {
	storeID := "integration_test_basic"
	s.cleanKey(storeID)

	seeder, err := NewValkeyConfigSeeder(s.redisURL(), s.logger)
	s.Require().NoError(err)
	defer seeder.Close()

	err = seeder.SeedConfig(context.Background(), storeID, map[string]string{
		"api_url": "http://localhost:8080",
		"timeout": "30",
	})
	s.Require().NoError(err)

	values := s.readKey(storeID)
	s.Assert().Equal("http://localhost:8080", values["api_url"])
	s.Assert().Equal("30", values["timeout"])
}

func (s *ValkeyIntegrationSuite) Test_seed_config_merges_with_existing() {
	storeID := "integration_test_merge"
	s.cleanKey(storeID)

	seeder, err := NewValkeyConfigSeeder(s.redisURL(), s.logger)
	s.Require().NoError(err)
	defer seeder.Close()

	// First seed.
	s.Require().NoError(seeder.SeedConfig(context.Background(), storeID, map[string]string{
		"key_a": "value_a",
		"key_b": "original_b",
	}))

	// Second seed overwrites key_b and adds key_c.
	s.Require().NoError(seeder.SeedConfig(context.Background(), storeID, map[string]string{
		"key_b": "updated_b",
		"key_c": "value_c",
	}))

	values := s.readKey(storeID)
	s.Assert().Equal("value_a", values["key_a"])
	s.Assert().Equal("updated_b", values["key_b"])
	s.Assert().Equal("value_c", values["key_c"])
}

func (s *ValkeyIntegrationSuite) Test_seed_config_empty_map_creates_empty_json() {
	storeID := "integration_test_empty"
	s.cleanKey(storeID)

	seeder, err := NewValkeyConfigSeeder(s.redisURL(), s.logger)
	s.Require().NoError(err)
	defer seeder.Close()

	s.Require().NoError(seeder.SeedConfig(context.Background(), storeID, map[string]string{}))

	values := s.readKey(storeID)
	s.Assert().Empty(values)
}

func (s *ValkeyIntegrationSuite) Test_close_releases_connection() {
	seeder, err := NewValkeyConfigSeeder(s.redisURL(), s.logger)
	s.Require().NoError(err)
	s.Require().NoError(seeder.Close())
}
