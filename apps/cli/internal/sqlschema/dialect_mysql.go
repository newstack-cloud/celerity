package sqlschema

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"ariga.io/atlas/sql/migrate"
	"ariga.io/atlas/sql/mysql"
	atlasschema "ariga.io/atlas/sql/schema"
	gomysql "github.com/go-sql-driver/mysql"
)

type mysqlDialect struct{}

func (mysqlDialect) DriverName() string { return "mysql" }

func (mysqlDialect) DatabaseExistsQuery() string {
	return "SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = ?"
}

func (mysqlDialect) CreateDatabaseSQL(quotedName string) string {
	return fmt.Sprintf("CREATE DATABASE %s", quotedName)
}

func (mysqlDialect) DropDatabaseSQL(quotedName string) string {
	return fmt.Sprintf("DROP DATABASE IF EXISTS %s", quotedName)
}

func (mysqlDialect) QuoteIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func (mysqlDialect) ReplaceDatabase(connStr string, dbName string) (string, error) {
	cfg, err := gomysql.ParseDSN(connStr)
	if err != nil {
		return "", fmt.Errorf("parsing MySQL DSN: %w", err)
	}
	cfg.DBName = dbName
	return cfg.FormatDSN(), nil
}

// NormalizeConnString ensures the DSN scans DATETIME columns into time.Time,
// which the migration-history reader relies on.
func (mysqlDialect) NormalizeConnString(connStr string) (string, error) {
	cfg, err := gomysql.ParseDSN(connStr)
	if err != nil {
		return "", fmt.Errorf("parsing MySQL DSN: %w", err)
	}
	cfg.ParseTime = true
	return cfg.FormatDSN(), nil
}

func (mysqlDialect) OpenAtlas(db *sql.DB) (migrate.Driver, error) {
	drv, err := mysql.Open(db)
	if err != nil {
		return nil, fmt.Errorf("opening atlas mysql driver: %w", err)
	}
	return drv, nil
}

func (mysqlDialect) ParseType(columnType string) (atlasschema.Type, error) {
	return mysql.ParseType(columnType)
}

// SchemaName returns the connected database, which is the schema Atlas manages
// for MySQL (MySQL has no separate schema namespace within a database).
func (mysqlDialect) SchemaName(ctx context.Context, db *sql.DB) (string, error) {
	var name sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&name); err != nil {
		return "", fmt.Errorf("reading current database name: %w", err)
	}
	if !name.Valid || name.String == "" {
		return "", fmt.Errorf("no database selected in the MySQL connection")
	}
	return name.String, nil
}

// The index methods that can be declared in the schema YAML.
var mysqlIndexTypes = map[string]string{
	"btree":    mysql.IndexTypeBTree,
	"hash":     mysql.IndexTypeHash,
	"fulltext": mysql.IndexTypeFullText,
	"spatial":  mysql.IndexTypeSpatial,
}

func (mysqlDialect) IndexTypeAttr(indexType string) (atlasschema.Attr, error) {
	if indexType == "" {
		return nil, nil
	}
	t, ok := mysqlIndexTypes[strings.ToLower(indexType)]
	if !ok {
		return nil, fmt.Errorf(
			"unsupported index type %q for mysql (supported: %s)",
			indexType, strings.Join(sortedKeys(mysqlIndexTypes), ", "),
		)
	}
	if t == mysql.IndexTypeBTree {
		return nil, nil
	}
	return &mysql.IndexType{T: t}, nil
}

func (mysqlDialect) NewRevisions(ctx context.Context, db *sql.DB) (migrate.RevisionReadWriter, error) {
	return newMySQLRevisions(ctx, db)
}

func (mysqlDialect) SupportsExtensions() bool { return false }

// FingerprintAdvancedObjects returns no fingerprints. Advanced-object drift
// (triggers, functions, views) is not yet implemented for MySQL, so those
// objects are not tracked by drift detection there.
func (mysqlDialect) FingerprintAdvancedObjects(context.Context, *sql.DB, string) (map[string]string, error) {
	return map[string]string{}, nil
}
