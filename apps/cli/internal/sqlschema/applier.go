package sqlschema

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ariga.io/atlas/sql/migrate"
	atlasschema "ariga.io/atlas/sql/schema"
	"go.uber.org/zap"
)

const sumFileName = "atlas.sum"

// Applier connects to a SQL database and applies schemas and migrations to it,
// backed by the embedded Atlas engine. Migration history is tracked in the
// atlas_schema_revisions table (see pgRevisions). Engine-specific connection
// concerns are delegated to a dialect implementation.
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

	normConnStr, err := d.NormalizeConnString(connStr)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(d.DriverName(), normConnStr)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", engine, err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging %s: %w", engine, err)
	}

	return &Applier{db: db, dialect: d, connStr: normConnStr, logger: logger}, nil
}

// Close releases the database connection.
func (a *Applier) Close() error {
	return a.db.Close()
}

// EnsureDatabase creates a database if it does not already exist.
// Must be called on an Applier connected to the admin database.
func (a *Applier) EnsureDatabase(ctx context.Context, dbName string) error {
	var count int
	err := a.db.QueryRowContext(ctx, a.dialect.DatabaseExistsQuery(), dbName).Scan(&count)
	if err != nil {
		return fmt.Errorf("checking if database %q exists: %w", dbName, err)
	}
	if count > 0 {
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

// DropDatabase removes a database if it exists. Must be called on an Applier
// connected to the admin database, with nothing else connected to the target.
func (a *Applier) DropDatabase(ctx context.Context, dbName string) error {
	_, err := a.db.ExecContext(ctx, a.dialect.DropDatabaseSQL(a.dialect.QuoteIdentifier(dbName)))
	if err != nil {
		return fmt.Errorf("dropping database %q: %w", dbName, err)
	}
	return nil
}

// HasUncommittedSchemaChanges reports whether the schema YAML describes changes
// that no committed migration covers. It answers the same question as
// `schema diff --check`, but against a scratch database on the connection this
// Applier already holds rather than a throwaway container.
//
// Must be called on an Applier connected to the admin database. The scratch
// database is created empty, replayed into, and dropped again.
func (a *Applier) HasUncommittedSchemaChanges(
	ctx context.Context,
	schemaPath, migrationsPath, engine, scratchDBName string,
) (bool, error) {
	if schemaPath == "" {
		return false, nil
	}
	hasFiles, err := hasMigrationFiles(migrationsPath)
	if err != nil || !hasFiles {
		return false, err
	}

	// Drop first: a scratch database left behind by an interrupted run would
	// otherwise fail the replay, which needs an empty database.
	if err := a.DropDatabase(ctx, scratchDBName); err != nil {
		return false, err
	}
	if err := a.EnsureDatabase(ctx, scratchDBName); err != nil {
		return false, err
	}
	defer func() {
		if err := a.DropDatabase(ctx, scratchDBName); err != nil {
			a.logger.Debug("could not drop scratch database",
				zap.String("database", scratchDBName), zap.Error(err))
		}
	}()

	scratchConnStr, err := a.dialect.ReplaceDatabase(a.connStr, scratchDBName)
	if err != nil {
		return false, err
	}
	return CheckPendingMigration(ctx, scratchConnStr, schemaPath, migrationsPath, engine)
}

// ForDatabase returns a new Applier connected to a specific database,
// derived from the original connection string by replacing the database path.
func (a *Applier) ForDatabase(dbName string) (*Applier, error) {
	newConnStr, err := a.dialect.ReplaceDatabase(a.connStr, dbName)
	if err != nil {
		return nil, err
	}
	newConnStr, err = a.dialect.NormalizeConnString(newConnStr)
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

func (a *Applier) atlasDriver() (migrate.Driver, error) {
	return a.dialect.OpenAtlas(a.db)
}

// Apply brings the connected database up to date for local development.
//
// Once a migration directory exists it is the only source of schema changes, so
// a local database comes up from exactly the migration history every other
// environment runs. Uncommitted schema YAML edits are reported separately (see
// HasUncommittedSchemaChanges) rather than being silently applied on top.
// Reconciling the YAML declaratively over a migrated database would drop
// anything the YAML cannot model, including hand-written indexes, triggers and
// the migration-history table itself.
//
// The schema YAML is applied declaratively only before the first migration
// exists, so a new project gets a working local database without having to run
// `celerity schema diff` first.
//
// The risk gate is fully open here (local dev applies everything); the gate is
// for deployed/live databases (see `schema apply`).
func (a *Applier) Apply(ctx context.Context, schemaPath, migrationsPath, engine string) error {
	hasFiles, err := hasMigrationFiles(migrationsPath)
	if err != nil {
		return err
	}
	if hasFiles {
		return a.ApplyMigrationDir(ctx, migrationsPath, RiskBreaking)
	}
	return a.ApplyDeclarative(ctx, schemaPath, engine)
}

// ApplyDeclarative brings the connected database into line with the desired
// state described by a schema YAML file, applying the diff directly without
// writing migration files. This is the local-development path: it keeps the
// schema YAML as the source of truth for ephemeral databases. Committed
// migrations (via `celerity schema diff`) are applied by ApplyMigrationDir.
func (a *Applier) ApplyDeclarative(ctx context.Context, schemaPath, engine string) error {
	if schemaPath == "" {
		return nil
	}

	parsed, err := ParseSchemaFile(schemaPath)
	if err != nil {
		return fmt.Errorf("parsing schema: %w", err)
	}
	desired, err := desiredSchema(parsed, engine)
	if err != nil {
		return fmt.Errorf("building desired schema: %w", err)
	}

	schemaName, err := a.dialect.SchemaName(ctx, a.db)
	if err != nil {
		return err
	}
	desired.Name = schemaName

	// Extensions are created directly so they exist before any column that
	// depends on them (e.g. a default calling an extension function).
	if a.dialect.SupportsExtensions() {
		for _, ext := range parsed.Extensions {
			if _, err := a.db.ExecContext(ctx, fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS %q", ext)); err != nil {
				return fmt.Errorf("creating extension %q: %w", ext, err)
			}
		}
	}

	drv, err := a.atlasDriver()
	if err != nil {
		return err
	}

	// Excluded objects are filtered out of the inspected state, so objects the
	// YAML deliberately does not model (hand-written expression indexes,
	// triggers, the migration-history table) are neither dropped nor altered.
	current, err := drv.InspectSchema(ctx, schemaName, &atlasschema.InspectOptions{
		Exclude: excludePatterns(parsed),
	})
	if err != nil {
		return fmt.Errorf("inspecting current schema: %w", err)
	}

	changes, err := drv.SchemaDiff(current, desired)
	if err != nil {
		return fmt.Errorf("diffing schema: %w", err)
	}
	if len(changes) == 0 {
		a.logger.Debug("schema already up to date", zap.String("schemaPath", schemaPath))
		return nil
	}

	if err := drv.ApplyChanges(ctx, changes); err != nil {
		return fmt.Errorf("applying schema changes: %w", err)
	}
	a.logger.Info("schema applied",
		zap.String("schemaPath", schemaPath),
		zap.Int("changes", len(changes)),
	)
	return nil
}

// ApplyMigrationDir applies pending migration files from an Atlas migration
// directory in timestamp order, tracking each in the atlas_schema_revisions
// table so already-applied migrations are skipped. Returns nil if the directory
// is empty or absent.
//
// maxRisk is the highest risk level (see the migration risk header) that may be
// auto-applied. Pending migrations are applied in order until one exceeds
// maxRisk, at which point the apply stops with an error naming that migration,
// so a live system is never broken by an auto-applied contract change.
// Migrations already applied before the blocked one remain committed.
func (a *Applier) ApplyMigrationDir(ctx context.Context, migrationsPath string, maxRisk RiskLevel) error {
	hasFiles, err := hasMigrationFiles(migrationsPath)
	if err != nil {
		return err
	}
	if !hasFiles {
		return nil
	}
	if err := ValidateMigrationFileNames(migrationsPath); err != nil {
		return err
	}

	dir, err := migrate.NewLocalDir(migrationsPath)
	if err != nil {
		return fmt.Errorf("opening migration directory: %w", err)
	}
	if err := ensureSumFile(migrationsPath, dir); err != nil {
		return fmt.Errorf("ensuring %s: %w", sumFileName, err)
	}

	drv, err := a.atlasDriver()
	if err != nil {
		return err
	}
	rrw, err := a.dialect.NewRevisions(ctx, a.db)
	if err != nil {
		return err
	}

	// A pre-existing database (with the default "public" schema) is not "clean"
	// by Atlas's definition; Celerity manages existing databases, so allow it.
	ex, err := migrate.NewExecutor(drv, dir, rrw, migrate.WithAllowDirty(true))
	if err != nil {
		return fmt.Errorf("creating migration executor: %w", err)
	}

	pending, err := ex.Pending(ctx)
	if errors.Is(err, migrate.ErrNoPendingFiles) {
		a.logger.Debug("no pending migrations", zap.String("migrationsPath", migrationsPath))
		return nil
	}
	if err != nil {
		return fmt.Errorf("computing pending migrations: %w", err)
	}

	applied := 0
	for _, file := range pending {
		stmts, _ := file.Stmts()
		risk, detail := effectiveRisk(file.Bytes(), stmts)
		if riskRank(risk) > riskRank(maxRisk) {
			a.logger.Info("stopping before a migration that exceeds the risk threshold",
				zap.String("migration", file.Name()),
				zap.String("risk", string(risk)),
				zap.Int("applied", applied),
			)
			return fmt.Errorf(
				"migration %q is classified %q%s and was not applied; run any backfill/data steps then re-run with %s, or split it into expand/contract",
				file.Name(), risk, detail, allowFlagHint(risk))
		}
		if err := ex.Execute(ctx, file); err != nil {
			return fmt.Errorf("applying migration %s: %w", file.Name(), err)
		}
		applied++
	}

	a.logger.Info("migrations applied",
		zap.String("migrationsPath", migrationsPath),
		zap.Int("applied", applied),
	)
	return nil
}

// PendingMigration describes a committed migration that has not yet been applied
// to the connected database.
type PendingMigration struct {
	Name string
	Risk RiskLevel
}

// PendingMigrations returns the migrations that are committed but not yet applied
// to the connected database, in apply order, each with its effective risk.
func (a *Applier) PendingMigrations(ctx context.Context, migrationsPath string) ([]PendingMigration, error) {
	hasFiles, err := hasMigrationFiles(migrationsPath)
	if err != nil || !hasFiles {
		return nil, err
	}

	dir, err := migrate.NewLocalDir(migrationsPath)
	if err != nil {
		return nil, fmt.Errorf("opening migration directory: %w", err)
	}
	if err := ensureSumFile(migrationsPath, dir); err != nil {
		return nil, fmt.Errorf("ensuring %s: %w", sumFileName, err)
	}
	drv, err := a.atlasDriver()
	if err != nil {
		return nil, err
	}
	rrw, err := a.dialect.NewRevisions(ctx, a.db)
	if err != nil {
		return nil, err
	}
	ex, err := migrate.NewExecutor(drv, dir, rrw, migrate.WithAllowDirty(true))
	if err != nil {
		return nil, fmt.Errorf("creating migration executor: %w", err)
	}
	pending, err := ex.Pending(ctx)
	if errors.Is(err, migrate.ErrNoPendingFiles) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("computing pending migrations: %w", err)
	}

	result := make([]PendingMigration, 0, len(pending))
	for _, file := range pending {
		stmts, _ := file.Stmts()
		risk, _ := effectiveRisk(file.Bytes(), stmts)
		result = append(result, PendingMigration{Name: file.Name(), Risk: risk})
	}
	return result, nil
}

func allowFlagHint(risk RiskLevel) string {
	if risk == RiskBreaking {
		return "--allow-breaking"
	}
	return "--allow-risky"
}

// ExecSQL executes a raw SQL string against the database.
func (a *Applier) ExecSQL(ctx context.Context, sqlContent string) error {
	_, err := a.db.ExecContext(ctx, sqlContent)
	return err
}

func hasMigrationFiles(migrationsPath string) (bool, error) {
	if migrationsPath == "" {
		return false, nil
	}
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading migration directory %s: %w", migrationsPath, err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			return true, nil
		}
	}
	return false, nil
}

// Written only when absent, so a hand-placed migration directory applies
// without a manual step. An existing atlas.sum is left untouched and is
// validated by the executor instead.
func ensureSumFile(migrationsPath string, dir *migrate.LocalDir) error {
	if _, err := os.Stat(filepath.Join(migrationsPath, sumFileName)); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	sum, err := dir.Checksum()
	if err != nil {
		return err
	}
	return migrate.WriteSumFile(dir, sum)
}
