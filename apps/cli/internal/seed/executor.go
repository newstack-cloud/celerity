package seed

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

// NoSQLSeeder inserts records into a NoSQL datastore.
type NoSQLSeeder interface {
	PutItem(ctx context.Context, tableName string, itemJSON []byte) error
}

// StorageUploader uploads files to an object store bucket.
type StorageUploader interface {
	Upload(ctx context.Context, bucketName string, objectKey string, data []byte) error
}

// SQLSeeder executes SQL statements against a named database.
type SQLSeeder interface {
	ExecSQL(ctx context.Context, databaseName string, sql string) error
}

// SeedResult tracks what was seeded for TUI reporting.
type SeedResult struct {
	Records    int
	Files      int
	SQLScripts int
}

// ExecuteSeed applies seed data from a loaded SeedConfig to the running services.
func ExecuteSeed(
	ctx context.Context,
	cfg *SeedConfig,
	nosqlSeeder NoSQLSeeder,
	storageUploader StorageUploader,
	sqlSeeder SQLSeeder,
	logger *zap.Logger,
) (*SeedResult, error) {
	if cfg == nil {
		return &SeedResult{}, nil
	}

	result := &SeedResult{}

	if err := seedNoSQL(ctx, cfg.NoSQL, nosqlSeeder, result, logger); err != nil {
		return nil, err
	}

	if err := seedStorage(ctx, cfg.Storage, storageUploader, result, logger); err != nil {
		return nil, err
	}

	if err := seedSQL(ctx, cfg.SQL, sqlSeeder, result, logger); err != nil {
		return nil, err
	}

	return result, nil
}

func seedNoSQL(
	ctx context.Context,
	tables []NoSQLTableSeed,
	seeder NoSQLSeeder,
	result *SeedResult,
	logger *zap.Logger,
) error {
	for _, table := range tables {
		logger.Debug("seeding table", zap.String("table", table.TableName), zap.Int("files", len(table.Files)))
		for _, filePath := range table.Files {
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("reading seed file %s: %w", filePath, err)
			}

			if err := seeder.PutItem(ctx, table.TableName, data); err != nil {
				return fmt.Errorf("seeding table %s from %s: %w", table.TableName, filepath.Base(filePath), err)
			}
			result.Records++
		}
	}
	return nil
}

func seedStorage(
	ctx context.Context,
	buckets []StorageBucketSeed,
	uploader StorageUploader,
	result *SeedResult,
	logger *zap.Logger,
) error {
	for _, bucket := range buckets {
		logger.Debug("seeding bucket", zap.String("bucket", bucket.BucketName), zap.Int("files", len(bucket.Files)))
		for _, filePath := range bucket.Files {
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("reading storage file %s: %w", filePath, err)
			}

			objectKey := filepath.Base(filePath)
			if err := uploader.Upload(ctx, bucket.BucketName, objectKey, data); err != nil {
				return fmt.Errorf("uploading %s to bucket %s: %w", objectKey, bucket.BucketName, err)
			}
			result.Files++
		}
	}
	return nil
}

func seedSQL(
	ctx context.Context,
	databases []SQLDatabaseSeed,
	seeder SQLSeeder,
	result *SeedResult,
	logger *zap.Logger,
) error {
	if seeder == nil {
		return nil
	}

	for _, db := range databases {
		logger.Debug("seeding sql database", zap.String("database", db.DatabaseName), zap.Int("scripts", len(db.Files)))
		for _, filePath := range db.Files {
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("reading sql seed file %s: %w", filePath, err)
			}

			if err := seeder.ExecSQL(ctx, db.DatabaseName, string(data)); err != nil {
				return fmt.Errorf("executing sql seed %s against %s: %w", filepath.Base(filePath), db.DatabaseName, err)
			}
			result.SQLScripts++
		}
	}
	return nil
}
