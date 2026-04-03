package seed

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type LoadSeedConfigTestSuite struct {
	suite.Suite
}

func (s *LoadSeedConfigTestSuite) mkdirAll(path string) {
	s.Require().NoError(os.MkdirAll(path, 0o755))
}

func (s *LoadSeedConfigTestSuite) writeFile(path string, content string) {
	s.Require().NoError(os.WriteFile(path, []byte(content), 0o644))
}

func (s *LoadSeedConfigTestSuite) Test_nonexistent_dir_returns_nil() {
	cfg, err := LoadSeedConfig("/nonexistent/path")
	s.Require().NoError(err)
	s.Assert().Nil(cfg)
}

func (s *LoadSeedConfigTestSuite) Test_empty_dir_returns_empty_config() {
	dir := s.T().TempDir()
	cfg, err := LoadSeedConfig(dir)
	s.Require().NoError(err)
	s.Require().NotNil(cfg)
	s.Assert().Empty(cfg.NoSQL)
	s.Assert().Empty(cfg.SQL)
	s.Assert().Empty(cfg.Storage)
	s.Assert().Empty(cfg.Hooks)
}

func (s *LoadSeedConfigTestSuite) Test_nosql_tables_discovered_from_subdirectories() {
	dir := s.T().TempDir()
	usersDir := filepath.Join(dir, "nosql", "users")
	ordersDir := filepath.Join(dir, "nosql", "orders")
	s.mkdirAll(usersDir)
	s.mkdirAll(ordersDir)

	s.writeFile(filepath.Join(usersDir, "user-001.json"), `{"id":"1","name":"Alice"}`)
	s.writeFile(filepath.Join(usersDir, "user-002.json"), `{"id":"2","name":"Bob"}`)
	s.writeFile(filepath.Join(ordersDir, "order-001.json"), `{"id":"o1"}`)

	cfg, err := LoadSeedConfig(dir)
	s.Require().NoError(err)
	s.Require().Len(cfg.NoSQL, 2)

	tableMap := map[string]NoSQLTableSeed{}
	for _, table := range cfg.NoSQL {
		tableMap[table.TableName] = table
	}

	users, ok := tableMap["users"]
	s.Require().True(ok, "expected 'users' table")
	s.Assert().Len(users.Files, 2)

	orders, ok := tableMap["orders"]
	s.Require().True(ok, "expected 'orders' table")
	s.Assert().Len(orders.Files, 1)
}

func (s *LoadSeedConfigTestSuite) Test_nosql_ignores_non_json_files() {
	dir := s.T().TempDir()
	tableDir := filepath.Join(dir, "nosql", "users")
	s.mkdirAll(tableDir)

	s.writeFile(filepath.Join(tableDir, "user-001.json"), `{"id":"1"}`)
	s.writeFile(filepath.Join(tableDir, "readme.txt"), "not a record")

	cfg, err := LoadSeedConfig(dir)
	s.Require().NoError(err)
	s.Require().Len(cfg.NoSQL, 1)
	s.Assert().Len(cfg.NoSQL[0].Files, 1)
}

func (s *LoadSeedConfigTestSuite) Test_sql_databases_discovered_from_subdirectories() {
	dir := s.T().TempDir()
	auditDir := filepath.Join(dir, "sql", "audit")
	s.mkdirAll(auditDir)

	s.writeFile(filepath.Join(auditDir, "002_seed.sql"), "INSERT INTO ...")
	s.writeFile(filepath.Join(auditDir, "001_schema.sql"), "CREATE TABLE ...")

	cfg, err := LoadSeedConfig(dir)
	s.Require().NoError(err)
	s.Require().Len(cfg.SQL, 1)
	s.Assert().Equal("audit", cfg.SQL[0].DatabaseName)
	s.Require().Len(cfg.SQL[0].Files, 2)
	s.Assert().Equal("001_schema.sql", filepath.Base(cfg.SQL[0].Files[0]))
	s.Assert().Equal("002_seed.sql", filepath.Base(cfg.SQL[0].Files[1]))
}

func (s *LoadSeedConfigTestSuite) Test_sql_multiple_databases() {
	dir := s.T().TempDir()
	auditDir := filepath.Join(dir, "sql", "audit")
	analyticsDir := filepath.Join(dir, "sql", "analytics")
	s.mkdirAll(auditDir)
	s.mkdirAll(analyticsDir)

	s.writeFile(filepath.Join(auditDir, "001_seed.sql"), "INSERT INTO audit_log ...")
	s.writeFile(filepath.Join(analyticsDir, "001_seed.sql"), "INSERT INTO events ...")

	cfg, err := LoadSeedConfig(dir)
	s.Require().NoError(err)
	s.Require().Len(cfg.SQL, 2)

	dbMap := map[string]SQLDatabaseSeed{}
	for _, db := range cfg.SQL {
		dbMap[db.DatabaseName] = db
	}
	s.Assert().Contains(dbMap, "audit")
	s.Assert().Contains(dbMap, "analytics")
}

func (s *LoadSeedConfigTestSuite) Test_storage_buckets_discovered_from_subdirectories() {
	dir := s.T().TempDir()
	bucketDir := filepath.Join(dir, "buckets", "my-bucket")
	s.mkdirAll(bucketDir)

	s.writeFile(filepath.Join(bucketDir, "logo.png"), "fake png data")
	s.writeFile(filepath.Join(bucketDir, "config.json"), `{"key":"val"}`)

	cfg, err := LoadSeedConfig(dir)
	s.Require().NoError(err)
	s.Require().Len(cfg.Storage, 1)
	s.Assert().Equal("my-bucket", cfg.Storage[0].BucketName)
	s.Assert().Len(cfg.Storage[0].Files, 2)
}

func (s *LoadSeedConfigTestSuite) Test_hooks_discovered_from_hooks_directory() {
	dir := s.T().TempDir()
	hooksDir := filepath.Join(dir, "hooks")
	s.mkdirAll(hooksDir)

	s.writeFile(filepath.Join(hooksDir, "post-setup.sh"), "#!/bin/bash\necho done")

	cfg, err := LoadSeedConfig(dir)
	s.Require().NoError(err)
	s.Require().Len(cfg.Hooks, 1)
	s.Assert().Equal("post-setup.sh", filepath.Base(cfg.Hooks[0]))
}

func (s *LoadSeedConfigTestSuite) Test_full_convention_structure() {
	dir := s.T().TempDir()

	s.mkdirAll(filepath.Join(dir, "nosql", "users"))
	s.writeFile(filepath.Join(dir, "nosql", "users", "u1.json"), `{"id":"1"}`)

	s.mkdirAll(filepath.Join(dir, "sql", "mydb"))
	s.writeFile(filepath.Join(dir, "sql", "mydb", "001.sql"), "INSERT INTO t VALUES (1)")

	s.mkdirAll(filepath.Join(dir, "buckets", "assets"))
	s.writeFile(filepath.Join(dir, "buckets", "assets", "img.png"), "data")

	s.mkdirAll(filepath.Join(dir, "hooks"))
	s.writeFile(filepath.Join(dir, "hooks", "setup.sh"), "#!/bin/bash")

	cfg, err := LoadSeedConfig(dir)
	s.Require().NoError(err)
	s.Assert().Len(cfg.NoSQL, 1)
	s.Assert().Len(cfg.SQL, 1)
	s.Assert().Len(cfg.Storage, 1)
	s.Assert().Len(cfg.Hooks, 1)
}

func TestLoadSeedConfigTestSuite(t *testing.T) {
	suite.Run(t, new(LoadSeedConfigTestSuite))
}

type ResolveSeedDirTestSuite struct {
	suite.Suite
}

func (s *ResolveSeedDirTestSuite) Test_run_mode_uses_seed_directory() {
	appDir := s.T().TempDir()
	result := ResolveSeedDir(appDir, "run")
	s.Assert().Equal(filepath.Join(appDir, "seed", "local"), result)
}

func (s *ResolveSeedDirTestSuite) Test_test_mode_prefers_seed_test_directory() {
	appDir := s.T().TempDir()
	seedTestDir := filepath.Join(appDir, "seed", "test")
	s.Require().NoError(os.MkdirAll(seedTestDir, 0o755))

	result := ResolveSeedDir(appDir, "test")
	s.Assert().Equal(seedTestDir, result)
}

func (s *ResolveSeedDirTestSuite) Test_test_mode_falls_back_to_seed_when_seed_test_absent() {
	appDir := s.T().TempDir()
	result := ResolveSeedDir(appDir, "test")
	s.Assert().Equal(filepath.Join(appDir, "seed", "local"), result)
}

func TestResolveSeedDirTestSuite(t *testing.T) {
	suite.Run(t, new(ResolveSeedDirTestSuite))
}

type ResolveConfigDirTestSuite struct {
	suite.Suite
}

func (s *ResolveConfigDirTestSuite) Test_run_mode_uses_config_local_directory() {
	appDir := s.T().TempDir()
	result := ResolveConfigDir(appDir, "run")
	s.Assert().Equal(filepath.Join(appDir, "config", "local"), result)
}

func (s *ResolveConfigDirTestSuite) Test_test_mode_prefers_config_test_directory() {
	appDir := s.T().TempDir()
	configTestDir := filepath.Join(appDir, "config", "test")
	s.Require().NoError(os.MkdirAll(configTestDir, 0o755))

	result := ResolveConfigDir(appDir, "test")
	s.Assert().Equal(configTestDir, result)
}

func (s *ResolveConfigDirTestSuite) Test_test_mode_falls_back_to_config_local_when_test_absent() {
	appDir := s.T().TempDir()
	result := ResolveConfigDir(appDir, "test")
	s.Assert().Equal(filepath.Join(appDir, "config", "local"), result)
}

func TestResolveConfigDirTestSuite(t *testing.T) {
	suite.Run(t, new(ResolveConfigDirTestSuite))
}

type ResolveSecretsDirTestSuite struct {
	suite.Suite
}

func (s *ResolveSecretsDirTestSuite) Test_run_mode_uses_secrets_local_directory() {
	appDir := s.T().TempDir()
	result := ResolveSecretsDir(appDir, "run")
	s.Assert().Equal(filepath.Join(appDir, "secrets", "local"), result)
}

func (s *ResolveSecretsDirTestSuite) Test_test_mode_prefers_secrets_test_directory() {
	appDir := s.T().TempDir()
	secretsTestDir := filepath.Join(appDir, "secrets", "test")
	s.Require().NoError(os.MkdirAll(secretsTestDir, 0o755))

	result := ResolveSecretsDir(appDir, "test")
	s.Assert().Equal(secretsTestDir, result)
}

func (s *ResolveSecretsDirTestSuite) Test_test_mode_falls_back_to_secrets_local_when_test_absent() {
	appDir := s.T().TempDir()
	result := ResolveSecretsDir(appDir, "test")
	s.Assert().Equal(filepath.Join(appDir, "secrets", "local"), result)
}

func TestResolveSecretsDirTestSuite(t *testing.T) {
	suite.Run(t, new(ResolveSecretsDirTestSuite))
}
