package sqlschema

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"ariga.io/atlas/sql/migrate"
	"ariga.io/atlas/sql/postgres"
	atlasschema "ariga.io/atlas/sql/schema"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// The schema Celerity manages within a Postgres database. Postgres databases
// always have a "public" schema, and scoping diffs to it keeps other schemas
// untouched.
const postgresSchemaName = "public"

type postgresDialect struct{}

func (postgresDialect) DriverName() string { return "pgx" }

func (postgresDialect) DatabaseExistsQuery() string {
	return "SELECT COUNT(*) FROM pg_database WHERE datname = $1"
}

func (postgresDialect) CreateDatabaseSQL(quotedName string) string {
	return fmt.Sprintf("CREATE DATABASE %s", quotedName)
}

func (postgresDialect) DropDatabaseSQL(quotedName string) string {
	return fmt.Sprintf("DROP DATABASE IF EXISTS %s", quotedName)
}

func (postgresDialect) QuoteIdentifier(name string) string {
	return fmt.Sprintf("%q", name)
}

func (postgresDialect) ReplaceDatabase(connStr string, dbName string) (string, error) {
	u, err := url.Parse(connStr)
	if err != nil {
		return "", fmt.Errorf("parsing connection string: %w", err)
	}
	u.Path = "/" + dbName
	return u.String(), nil
}

func (postgresDialect) OpenAtlas(db *sql.DB) (migrate.Driver, error) {
	drv, err := postgres.Open(db)
	if err != nil {
		return nil, fmt.Errorf("opening atlas postgres driver: %w", err)
	}
	return drv, nil
}

func (postgresDialect) ParseType(columnType string) (atlasschema.Type, error) {
	return postgres.ParseType(columnType)
}

func (postgresDialect) SchemaName(context.Context, *sql.DB) (string, error) {
	return postgresSchemaName, nil
}

// The index methods that can be declared in the schema YAML. Methods beyond
// these, and indexes needing operator classes, expressions or predicates, belong
// in a hand-written migration.
var postgresIndexTypes = map[string]string{
	"btree":  postgres.IndexTypeBTree,
	"gin":    postgres.IndexTypeGIN,
	"gist":   postgres.IndexTypeGiST,
	"spgist": postgres.IndexTypeSPGiST,
	"brin":   postgres.IndexTypeBRIN,
	"hash":   postgres.IndexTypeHash,
}

func (postgresDialect) IndexTypeAttr(indexType string) (atlasschema.Attr, error) {
	if indexType == "" {
		return nil, nil
	}
	t, ok := postgresIndexTypes[strings.ToLower(indexType)]
	if !ok {
		return nil, fmt.Errorf(
			"unsupported index type %q for postgres (supported: %s)",
			indexType, strings.Join(sortedKeys(postgresIndexTypes), ", "),
		)
	}
	// Atlas defaults an absent attribute to btree; leaving it unset keeps a
	// schema that spells out the default diff-identical to one that omits it.
	if t == postgres.IndexTypeBTree {
		return nil, nil
	}
	return &postgres.IndexType{T: t}, nil
}

func (postgresDialect) NewRevisions(ctx context.Context, db *sql.DB) (migrate.RevisionReadWriter, error) {
	return newPGRevisions(ctx, db)
}

func (postgresDialect) SupportsExtensions() bool { return true }

func (postgresDialect) NormalizeConnString(connStr string) (string, error) {
	return connStr, nil
}

func (postgresDialect) FingerprintAdvancedObjects(ctx context.Context, db *sql.DB, _ string) (map[string]string, error) {
	return fingerprintPostgresAdvancedObjects(ctx, db)
}
