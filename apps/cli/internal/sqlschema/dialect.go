package sqlschema

import (
	"context"
	"database/sql"
	"fmt"

	"ariga.io/atlas/sql/migrate"
	atlasschema "ariga.io/atlas/sql/schema"
)

// dialect encapsulates engine-specific SQL syntax, driver details and the Atlas
// integration for a database engine. Each supported engine provides its own
// implementation.
type dialect interface {
	// DriverName returns the database/sql driver name (e.g. "pgx", "mysql").
	DriverName() string

	// DatabaseExistsQuery returns a query that accepts a database name parameter
	// and returns a single boolean column indicating whether the database exists.
	DatabaseExistsQuery() string

	// CreateDatabaseSQL returns the DDL to create a database with the given name.
	// The name is already quoted with QuoteIdentifier.
	CreateDatabaseSQL(quotedName string) string

	// DropDatabaseSQL returns the DDL to drop a database if it exists. The name
	// is already quoted with QuoteIdentifier.
	DropDatabaseSQL(quotedName string) string

	// QuoteIdentifier quotes a database identifier (table name, database name)
	// using the engine's quoting convention.
	QuoteIdentifier(name string) string

	// ReplaceDatabase returns a new connection string with the database name replaced.
	ReplaceDatabase(connStr string, dbName string) (string, error)

	// NormalizeConnString adjusts a connection string with parameters the engine
	// needs (e.g. MySQL's parseTime), returning it unchanged when none are needed.
	NormalizeConnString(connStr string) (string, error)

	// OpenAtlas wraps a live connection in the engine's Atlas driver, which
	// provides schema inspection, diffing and change application.
	OpenAtlas(db *sql.DB) (migrate.Driver, error)

	// ParseType parses an engine-native column type (e.g. "varchar(255)") into an
	// Atlas type.
	ParseType(columnType string) (atlasschema.Type, error)

	// SchemaName returns the name of the schema Celerity manages within the
	// connected database. Postgres uses a fixed "public"; MySQL uses the connected
	// database name.
	SchemaName(ctx context.Context, db *sql.DB) (string, error)

	// IndexTypeAttr maps an index method from the schema YAML (e.g. "gin") to the
	// engine's Atlas attribute. It returns a nil attribute for the empty string
	// and for the engine default (btree), so a schema that omits the method
	// diffs identically to one that spells out the default. Unsupported methods
	// are an error naming what the engine accepts.
	IndexTypeAttr(indexType string) (atlasschema.Attr, error)

	// NewRevisions returns the engine-specific migration-history store backing the
	// atlas_schema_revisions table.
	NewRevisions(ctx context.Context, db *sql.DB) (migrate.RevisionReadWriter, error)

	// ProvisionAppUser creates or repairs the application database role. Every
	// identifier in the request has already been validated, which is what lets
	// the implementations interpolate them into DDL that cannot take
	// placeholders. It must be idempotent: it runs after every deploy.
	ProvisionAppUser(ctx context.Context, db *sql.DB, req provisionRequest) (*ProvisionReport, error)

	// SupportsExtensions reports whether the engine has CREATE EXTENSION (Postgres).
	SupportsExtensions() bool

	// FingerprintAdvancedObjects fingerprints the objects the structural diff does
	// not cover (views, triggers, functions, policies) for drift detection. It
	// returns an empty map on engines where this is not yet implemented.
	FingerprintAdvancedObjects(ctx context.Context, db *sql.DB, schemaName string) (map[string]string, error)
}

// mysqlEnabled gates the MySQL dialect. MySQL support is fully implemented but is
// not part of the v0 release, so it is disabled by default; the MySQL tests flip
// it on. Flip this default (or remove the gate) to ship MySQL support.
var mysqlEnabled = false

func dialectForEngine(engine string) (dialect, error) {
	switch engine {
	case "postgres", "":
		return postgresDialect{}, nil
	case "mysql":
		if !mysqlEnabled {
			return nil, fmt.Errorf("SQL engine %q is not yet supported (postgres is supported in v0; MySQL is planned for a future release)", engine)
		}
		return mysqlDialect{}, nil
	default:
		return nil, fmt.Errorf("unsupported SQL engine %q (supported: postgres)", engine)
	}
}
