package sqlschema

import "fmt"

// dialect encapsulates engine-specific SQL syntax and driver details.
// Each supported database engine provides its own implementation.
type dialect interface {
	// DriverName returns the database/sql driver name (e.g. "pgx", "mysql").
	DriverName() string

	// DatabaseExistsQuery returns a query that accepts a database name parameter
	// and returns a single boolean column indicating whether the database exists.
	DatabaseExistsQuery() string

	// CreateDatabaseSQL returns the DDL to create a database with the given name.
	// The name is already quoted with QuoteIdentifier.
	CreateDatabaseSQL(quotedName string) string

	// CreateMigrationsTableSQL returns the DDL to create the migrations tracking table.
	CreateMigrationsTableSQL(tableName string) string

	// InsertVersionSQL returns the SQL to insert a migration version record,
	// ignoring duplicates. Uses engine-appropriate parameter placeholders.
	InsertVersionSQL(tableName string) string

	// DeleteVersionSQL returns the SQL to delete a migration version record.
	DeleteVersionSQL(tableName string) string

	// SelectVersionsSQL returns the SQL to select all applied migration versions.
	SelectVersionsSQL(tableName string) string

	// QuoteIdentifier quotes a database identifier (table name, database name)
	// using the engine's quoting convention.
	QuoteIdentifier(name string) string

	// ReplaceDatabase returns a new connection string with the database name replaced.
	ReplaceDatabase(connStr string, dbName string) (string, error)
}

func dialectForEngine(engine string) (dialect, error) {
	switch engine {
	case "postgres", "":
		return postgresDialect{}, nil
	case "mysql":
		return nil, fmt.Errorf("SQL engine %q is not yet supported (planned for v1)", engine)
	default:
		return nil, fmt.Errorf("unsupported SQL engine %q", engine)
	}
}
