package seed

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type ExecutorTestSuite struct {
	suite.Suite
	logger *zap.Logger
}

func TestExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(ExecutorTestSuite))
}

func (s *ExecutorTestSuite) SetupTest() {
	logger, _ := zap.NewDevelopment()
	s.logger = logger
}

type mockNoSQLSeeder struct {
	calls []struct {
		table string
		data  []byte
	}
	err error
}

func (m *mockNoSQLSeeder) PutItem(_ context.Context, tableName string, itemJSON []byte) error {
	m.calls = append(m.calls, struct {
		table string
		data  []byte
	}{tableName, itemJSON})
	return m.err
}

type mockStorageUploader struct {
	calls []struct {
		bucket, key string
		data        []byte
	}
	err error
}

func (m *mockStorageUploader) Upload(_ context.Context, bucketName string, objectKey string, data []byte) error {
	m.calls = append(m.calls, struct {
		bucket, key string
		data        []byte
	}{bucketName, objectKey, data})
	return m.err
}

type mockSQLSeeder struct {
	calls []struct{ db, sql string }
	err   error
}

func (m *mockSQLSeeder) ExecSQL(_ context.Context, databaseName string, sql string) error {
	m.calls = append(m.calls, struct{ db, sql string }{databaseName, sql})
	return m.err
}

func (s *ExecutorTestSuite) Test_nil_config_returns_empty_result() {
	result, err := ExecuteSeed(context.Background(), nil, &mockNoSQLSeeder{}, &mockStorageUploader{}, &mockSQLSeeder{}, s.logger)
	s.Require().NoError(err)
	s.Assert().Equal(0, result.Records)
	s.Assert().Equal(0, result.Files)
	s.Assert().Equal(0, result.SQLScripts)
}

func (s *ExecutorTestSuite) Test_seeds_nosql_tables() {
	dir := s.T().TempDir()
	f1 := filepath.Join(dir, "item1.json")
	f2 := filepath.Join(dir, "item2.json")
	s.Require().NoError(os.WriteFile(f1, []byte(`{"id":"1"}`), 0o644))
	s.Require().NoError(os.WriteFile(f2, []byte(`{"id":"2"}`), 0o644))

	seeder := &mockNoSQLSeeder{}
	cfg := &SeedConfig{
		NoSQL: []NoSQLTableSeed{
			{TableName: "users", Files: []string{f1, f2}},
		},
	}

	result, err := ExecuteSeed(context.Background(), cfg, seeder, &mockStorageUploader{}, nil, s.logger)
	s.Require().NoError(err)
	s.Assert().Equal(2, result.Records)
	s.Assert().Len(seeder.calls, 2)
	s.Assert().Equal("users", seeder.calls[0].table)
}

func (s *ExecutorTestSuite) Test_seeds_storage_buckets() {
	dir := s.T().TempDir()
	f1 := filepath.Join(dir, "image.png")
	s.Require().NoError(os.WriteFile(f1, []byte("png-data"), 0o644))

	uploader := &mockStorageUploader{}
	cfg := &SeedConfig{
		Storage: []StorageBucketSeed{
			{BucketName: "assets", Files: []string{f1}},
		},
	}

	result, err := ExecuteSeed(context.Background(), cfg, &mockNoSQLSeeder{}, uploader, nil, s.logger)
	s.Require().NoError(err)
	s.Assert().Equal(1, result.Files)
	s.Assert().Len(uploader.calls, 1)
	s.Assert().Equal("assets", uploader.calls[0].bucket)
	s.Assert().Equal("image.png", uploader.calls[0].key)
}

func (s *ExecutorTestSuite) Test_seeds_sql_databases() {
	dir := s.T().TempDir()
	f1 := filepath.Join(dir, "setup.sql")
	s.Require().NoError(os.WriteFile(f1, []byte("CREATE TABLE users (id INT);"), 0o644))

	seeder := &mockSQLSeeder{}
	cfg := &SeedConfig{
		SQL: []SQLDatabaseSeed{
			{DatabaseName: "mydb", Files: []string{f1}},
		},
	}

	result, err := ExecuteSeed(context.Background(), cfg, &mockNoSQLSeeder{}, &mockStorageUploader{}, seeder, s.logger)
	s.Require().NoError(err)
	s.Assert().Equal(1, result.SQLScripts)
	s.Assert().Len(seeder.calls, 1)
	s.Assert().Equal("mydb", seeder.calls[0].db)
}

func (s *ExecutorTestSuite) Test_nil_sql_seeder_skips_sql() {
	dir := s.T().TempDir()
	f1 := filepath.Join(dir, "setup.sql")
	s.Require().NoError(os.WriteFile(f1, []byte("SELECT 1;"), 0o644))

	cfg := &SeedConfig{
		SQL: []SQLDatabaseSeed{
			{DatabaseName: "mydb", Files: []string{f1}},
		},
	}

	result, err := ExecuteSeed(context.Background(), cfg, &mockNoSQLSeeder{}, &mockStorageUploader{}, nil, s.logger)
	s.Require().NoError(err)
	s.Assert().Equal(0, result.SQLScripts)
}

func (s *ExecutorTestSuite) Test_nosql_seeder_error_propagates() {
	dir := s.T().TempDir()
	f := filepath.Join(dir, "item.json")
	s.Require().NoError(os.WriteFile(f, []byte(`{}`), 0o644))

	seeder := &mockNoSQLSeeder{err: context.DeadlineExceeded}
	cfg := &SeedConfig{
		NoSQL: []NoSQLTableSeed{{TableName: "t", Files: []string{f}}},
	}

	_, err := ExecuteSeed(context.Background(), cfg, seeder, &mockStorageUploader{}, nil, s.logger)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "seeding table")
}

func (s *ExecutorTestSuite) Test_storage_uploader_error_propagates() {
	dir := s.T().TempDir()
	f := filepath.Join(dir, "file.txt")
	s.Require().NoError(os.WriteFile(f, []byte("data"), 0o644))

	uploader := &mockStorageUploader{err: context.DeadlineExceeded}
	cfg := &SeedConfig{
		Storage: []StorageBucketSeed{{BucketName: "b", Files: []string{f}}},
	}

	_, err := ExecuteSeed(context.Background(), cfg, &mockNoSQLSeeder{}, uploader, nil, s.logger)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "uploading")
}

func (s *ExecutorTestSuite) Test_missing_seed_file_returns_error() {
	cfg := &SeedConfig{
		NoSQL: []NoSQLTableSeed{{TableName: "t", Files: []string{"/nonexistent/item.json"}}},
	}

	_, err := ExecuteSeed(context.Background(), cfg, &mockNoSQLSeeder{}, &mockStorageUploader{}, nil, s.logger)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "reading seed file")
}
