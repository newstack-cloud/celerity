package sqlschema

import (
	"fmt"
	"net/url"
)

type postgresDialect struct{}

func (postgresDialect) DriverName() string { return "pgx" }

func (postgresDialect) DatabaseExistsQuery() string {
	return "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)"
}

func (postgresDialect) CreateDatabaseSQL(quotedName string) string {
	return fmt.Sprintf("CREATE DATABASE %s", quotedName)
}

func (postgresDialect) CreateMigrationsTableSQL(tableName string) string {
	return fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version integer NOT NULL,
			description text NOT NULL,
			applied_at timestamptz NOT NULL DEFAULT NOW(),
			PRIMARY KEY (version)
		)
	`, tableName)
}

func (postgresDialect) InsertVersionSQL(tableName string) string {
	return fmt.Sprintf(
		"INSERT INTO %s (version, description, applied_at) VALUES ($1, $2, $3) ON CONFLICT (version) DO NOTHING",
		tableName,
	)
}

func (postgresDialect) DeleteVersionSQL(tableName string) string {
	return fmt.Sprintf("DELETE FROM %s WHERE version = $1", tableName)
}

func (postgresDialect) SelectVersionsSQL(tableName string) string {
	return fmt.Sprintf("SELECT version FROM %s ORDER BY version", tableName)
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
