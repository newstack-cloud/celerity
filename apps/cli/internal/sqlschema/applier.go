package sqlschema

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
)

const migrationsTable = "celerity_schema_migrations"

// Applier applies SQL schemas and migration scripts to a database.
// It tracks applied migrations in a celerity_schema_migrations table.
// Engine-specific SQL is delegated to a dialect implementation.
type Applier struct {
	db      *sql.DB
	dialect dialect
	connStr string
	logger  *zap.Logger
}

// NewApplier creates an Applier for the given engine and connection string.
func NewApplier(engine string, connStr string, logger *zap.Logger) (*Applier, error) {
	d, err := dialectForEngine(engine)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(d.DriverName(), connStr)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", engine, err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging %s: %w", engine, err)
	}

	return &Applier{db: db, dialect: d, connStr: connStr, logger: logger}, nil
}

// Close releases the database connection.
func (a *Applier) Close() error {
	return a.db.Close()
}

// EnsureDatabase creates a database if it does not already exist.
// Must be called on an Applier connected to the admin database.
func (a *Applier) EnsureDatabase(ctx context.Context, dbName string) error {
	var exists bool
	err := a.db.QueryRowContext(ctx, a.dialect.DatabaseExistsQuery(), dbName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("checking if database %q exists: %w", dbName, err)
	}
	if exists {
		a.logger.Debug("database already exists", zap.String("database", dbName))
		return nil
	}

	// CREATE DATABASE cannot be parameterised; quote the identifier.
	_, err = a.db.ExecContext(ctx, a.dialect.CreateDatabaseSQL(a.dialect.QuoteIdentifier(dbName)))
	if err != nil {
		return fmt.Errorf("creating database %q: %w", dbName, err)
	}
	a.logger.Info("database created", zap.String("database", dbName))
	return nil
}

// ForDatabase returns a new Applier connected to a specific database,
// derived from the original connection string by replacing the database path.
func (a *Applier) ForDatabase(dbName string) (*Applier, error) {
	newConnStr, err := a.dialect.ReplaceDatabase(a.connStr, dbName)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(a.dialect.DriverName(), newConnStr)
	if err != nil {
		return nil, fmt.Errorf("connecting to database %s: %w", dbName, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database %s: %w", dbName, err)
	}

	return &Applier{db: db, dialect: a.dialect, connStr: newConnStr, logger: a.logger}, nil
}

func (a *Applier) ensureMigrationsTable(ctx context.Context) error {
	_, err := a.db.ExecContext(ctx, a.dialect.CreateMigrationsTableSQL(migrationsTable))
	return err
}

func (a *Applier) appliedVersions(ctx context.Context) (map[int]bool, error) {
	rows, err := a.db.QueryContext(ctx, a.dialect.SelectVersionsSQL(migrationsTable))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	versions := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		versions[v] = true
	}
	return versions, rows.Err()
}

func (a *Applier) recordVersion(ctx context.Context, version int, description string) error {
	_, err := a.db.ExecContext(ctx, a.dialect.InsertVersionSQL(migrationsTable),
		version, description, time.Now(),
	)
	return err
}

func (a *Applier) removeVersion(ctx context.Context, version int) error {
	_, err := a.db.ExecContext(ctx, a.dialect.DeleteVersionSQL(migrationsTable), version)
	return err
}

// ApplySchema parses a schema YAML file and executes the up DDL statements.
func (a *Applier) ApplySchema(ctx context.Context, schemaPath string, engine string) error {
	if schemaPath == "" {
		return nil
	}

	s, err := ParseSchemaFile(schemaPath)
	if err != nil {
		return fmt.Errorf("parsing schema: %w", err)
	}

	ddl, err := GenerateDDL(s, engine)
	if err != nil {
		return fmt.Errorf("generating DDL: %w", err)
	}

	for _, stmt := range ddl.Up {
		a.logger.Debug("executing DDL", zap.String("statement", truncate(stmt, 200)))
		if _, err := a.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("executing DDL: %w\nstatement: %s", err, stmt)
		}
	}

	a.logger.Info("schema applied",
		zap.String("schemaPath", schemaPath),
		zap.Int("statements", len(ddl.Up)),
	)
	return nil
}

// RollbackSchema parses a schema YAML file and executes the down DDL statements.
func (a *Applier) RollbackSchema(ctx context.Context, schemaPath string, engine string) error {
	if schemaPath == "" {
		return nil
	}

	s, err := ParseSchemaFile(schemaPath)
	if err != nil {
		return fmt.Errorf("parsing schema: %w", err)
	}

	ddl, err := GenerateDDL(s, engine)
	if err != nil {
		return fmt.Errorf("generating DDL: %w", err)
	}

	for _, stmt := range ddl.Down {
		a.logger.Debug("executing rollback DDL", zap.String("statement", truncate(stmt, 200)))
		if _, err := a.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("executing rollback DDL: %w\nstatement: %s", err, stmt)
		}
	}

	a.logger.Info("schema rolled back",
		zap.String("schemaPath", schemaPath),
		zap.Int("statements", len(ddl.Down)),
	)
	return nil
}

// ApplyMigrations discovers and executes pending up migration scripts in version order.
func (a *Applier) ApplyMigrations(ctx context.Context, migrationsPath string) error {
	scripts, err := DiscoverMigrationScripts(migrationsPath)
	if err != nil {
		return err
	}
	if len(scripts) == 0 {
		return nil
	}

	if err := a.ensureMigrationsTable(ctx); err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	applied, err := a.appliedVersions(ctx)
	if err != nil {
		return fmt.Errorf("reading applied versions: %w", err)
	}

	appliedCount := 0
	for _, script := range scripts {
		if applied[script.Version] {
			a.logger.Debug("migration already applied, skipping",
				zap.Int("version", script.Version),
			)
			continue
		}
		if script.UpPath == "" {
			continue
		}

		content, err := os.ReadFile(script.UpPath)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", script.UpPath, err)
		}

		a.logger.Debug("applying migration",
			zap.Int("version", script.Version),
			zap.String("description", script.Description),
		)
		if _, err := a.db.ExecContext(ctx, string(content)); err != nil {
			return fmt.Errorf("applying migration V%d__%s: %w", script.Version, script.Description, err)
		}

		if err := a.recordVersion(ctx, script.Version, script.Description); err != nil {
			return fmt.Errorf("recording migration V%d: %w", script.Version, err)
		}
		appliedCount++
	}

	if appliedCount > 0 {
		a.logger.Info("migrations applied",
			zap.String("migrationsPath", migrationsPath),
			zap.Int("applied", appliedCount),
			zap.Int("skipped", len(scripts)-appliedCount),
		)
	}
	return nil
}

// RollbackMigrations discovers and executes down migration scripts in reverse version order.
func (a *Applier) RollbackMigrations(ctx context.Context, migrationsPath string) error {
	scripts, err := DiscoverMigrationScripts(migrationsPath)
	if err != nil {
		return err
	}
	if len(scripts) == 0 {
		return nil
	}

	if err := a.ensureMigrationsTable(ctx); err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	applied, err := a.appliedVersions(ctx)
	if err != nil {
		return fmt.Errorf("reading applied versions: %w", err)
	}

	for i := len(scripts) - 1; i >= 0; i-- {
		script := scripts[i]
		if !applied[script.Version] {
			continue
		}
		if script.DownPath == "" {
			a.logger.Warn("no down migration found, skipping",
				zap.Int("version", script.Version),
				zap.String("description", script.Description),
			)
			continue
		}

		content, err := os.ReadFile(script.DownPath)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", script.DownPath, err)
		}

		a.logger.Debug("rolling back migration",
			zap.Int("version", script.Version),
			zap.String("description", script.Description),
		)
		if _, err := a.db.ExecContext(ctx, string(content)); err != nil {
			return fmt.Errorf("rolling back migration V%d__%s: %w", script.Version, script.Description, err)
		}

		if err := a.removeVersion(ctx, script.Version); err != nil {
			return fmt.Errorf("removing migration record V%d: %w", script.Version, err)
		}
	}

	a.logger.Info("migrations rolled back",
		zap.String("migrationsPath", migrationsPath),
		zap.Int("scripts", len(scripts)),
	)
	return nil
}

// ApplyAll applies schema DDL (up) followed by pending migration up scripts.
func (a *Applier) ApplyAll(ctx context.Context, schemaPath, migrationsPath, engine string) error {
	if err := a.ApplySchema(ctx, schemaPath, engine); err != nil {
		return err
	}
	return a.ApplyMigrations(ctx, migrationsPath)
}

// RollbackAll rolls back applied migrations (down, reverse order) then schema DDL (down).
func (a *Applier) RollbackAll(ctx context.Context, schemaPath, migrationsPath, engine string) error {
	if err := a.RollbackMigrations(ctx, migrationsPath); err != nil {
		return err
	}
	return a.RollbackSchema(ctx, schemaPath, engine)
}

// ExecSQL executes a raw SQL string against the database.
func (a *Applier) ExecSQL(ctx context.Context, sqlContent string) error {
	_, err := a.db.ExecContext(ctx, sqlContent)
	return err
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
