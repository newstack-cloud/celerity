package seed

import (
	"os"
	"path/filepath"
	"sort"
)

// SeedConfig describes all seed data discovered in a convention directory.
type SeedConfig struct {
	NoSQL   []NoSQLTableSeed
	SQL     []SQLDatabaseSeed
	Storage []StorageBucketSeed
	Hooks   []string
}

// NoSQLTableSeed represents seed data for a single NoSQL table.
type NoSQLTableSeed struct {
	TableName string
	Files     []string
}

// SQLDatabaseSeed represents seed data for a single SQL database.
// Each subdirectory in sql/ maps to a database name (matching celerity/sqlDatabase spec.name).
type SQLDatabaseSeed struct {
	DatabaseName string
	Files        []string // .sql files, sorted alphabetically
}

// StorageBucketSeed represents seed data for a single storage bucket.
type StorageBucketSeed struct {
	BucketName string
	Files      []string
}

// LoadSeedConfig discovers seed data in the given directory following
// the Celerity convention structure: nosql/, sql/, buckets/, hooks/.
// Returns nil if the directory does not exist.
func LoadSeedConfig(seedDir string) (*SeedConfig, error) {
	if _, err := os.Stat(seedDir); os.IsNotExist(err) {
		return nil, nil
	}

	config := &SeedConfig{}

	nosqlDir := filepath.Join(seedDir, "nosql")
	if tables, err := loadNoSQLSeed(nosqlDir); err != nil {
		return nil, err
	} else {
		config.NoSQL = tables
	}

	sqlDir := filepath.Join(seedDir, "sql")
	if scripts, err := loadSQLSeed(sqlDir); err != nil {
		return nil, err
	} else {
		config.SQL = scripts
	}

	storageDir := filepath.Join(seedDir, "buckets")
	if buckets, err := loadStorageSeed(storageDir); err != nil {
		return nil, err
	} else {
		config.Storage = buckets
	}

	hooksDir := filepath.Join(seedDir, "hooks")
	if hooks, err := loadHooks(hooksDir); err != nil {
		return nil, err
	} else {
		config.Hooks = hooks
	}

	return config, nil
}

// ResolveSeedDir determines the seed directory based on the command mode.
// For "test" mode, it prefers seed/test/ and falls back to seed/local/.
// For "run" mode (or any other), it uses seed/local/.
func ResolveSeedDir(appDir string, mode string) string {
	if mode == "test" {
		testDir := filepath.Join(appDir, "seed", "test")
		if _, err := os.Stat(testDir); err == nil {
			return testDir
		}
	}
	return filepath.Join(appDir, "seed", "local")
}

// ResolveConfigDir determines the config directory based on the command mode.
// For "test" mode, it prefers config/test/ and falls back to config/local/.
// For "run" mode (or any other), it uses config/local/.
func ResolveConfigDir(appDir string, mode string) string {
	if mode == "test" {
		testDir := filepath.Join(appDir, "config", "test")
		if _, err := os.Stat(testDir); err == nil {
			return testDir
		}
	}
	return filepath.Join(appDir, "config", "local")
}

// ResolveSecretsDir determines the secrets directory based on the command mode.
// For "test" mode, it prefers secrets/test/ and falls back to secrets/local/.
// For "run" mode (or any other), it uses secrets/local/.
func ResolveSecretsDir(appDir string, mode string) string {
	if mode == "test" {
		testDir := filepath.Join(appDir, "secrets", "test")
		if _, err := os.Stat(testDir); err == nil {
			return testDir
		}
	}
	return filepath.Join(appDir, "secrets", "local")
}

// loadNoSQLSeed reads subdirectories of nosql/, each representing a table.
// Each .json file in a subdirectory is a single record.
func loadNoSQLSeed(nosqlDir string) ([]NoSQLTableSeed, error) {
	if _, err := os.Stat(nosqlDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(nosqlDir)
	if err != nil {
		return nil, err
	}

	var tables []NoSQLTableSeed
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		tableName := entry.Name()
		tableDir := filepath.Join(nosqlDir, tableName)
		files, err := collectFiles(tableDir, ".json")
		if err != nil {
			return nil, err
		}
		if len(files) == 0 {
			continue
		}

		tables = append(tables, NoSQLTableSeed{
			TableName: tableName,
			Files:     files,
		})
	}

	return tables, nil
}

// loadSQLSeed reads subdirectories of sql/, each representing a database.
// Each .sql file in a subdirectory is a seed script executed in filename-sorted order.
func loadSQLSeed(sqlDir string) ([]SQLDatabaseSeed, error) {
	if _, err := os.Stat(sqlDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(sqlDir)
	if err != nil {
		return nil, err
	}

	var databases []SQLDatabaseSeed
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dbName := entry.Name()
		dbDir := filepath.Join(sqlDir, dbName)
		files, err := collectFiles(dbDir, ".sql")
		if err != nil {
			return nil, err
		}
		if len(files) == 0 {
			continue
		}

		databases = append(databases, SQLDatabaseSeed{
			DatabaseName: dbName,
			Files:        files,
		})
	}

	return databases, nil
}

func loadStorageSeed(storageDir string) ([]StorageBucketSeed, error) {
	if _, err := os.Stat(storageDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(storageDir)
	if err != nil {
		return nil, err
	}

	var buckets []StorageBucketSeed
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		bucketName := entry.Name()
		bucketDir := filepath.Join(storageDir, bucketName)
		files, err := collectAllFiles(bucketDir)
		if err != nil {
			return nil, err
		}
		if len(files) == 0 {
			continue
		}

		buckets = append(buckets, StorageBucketSeed{
			BucketName: bucketName,
			Files:      files,
		})
	}

	return buckets, nil
}

func loadHooks(hooksDir string) ([]string, error) {
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return nil, nil
	}
	return collectFiles(hooksDir, ".sh")
}

// collectFiles returns sorted absolute paths of files with the given extension.
func collectFiles(dir string, ext string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ext {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}

	sort.Strings(files)
	return files, nil
}

// collectAllFiles returns sorted absolute paths of all files in a directory (non-recursive).
func collectAllFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}

	sort.Strings(files)
	return files, nil
}
