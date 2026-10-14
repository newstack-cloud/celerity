package sqlschema

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/newstack-cloud/celerity/apps/cli/internal/testutils"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type ApplierIntegrationSuite struct {
	suite.Suite
	connStr string
	logger  *zap.Logger
}

func TestApplierIntegrationSuite(t *testing.T) {
	suite.Run(t, new(ApplierIntegrationSuite))
}

func (s *ApplierIntegrationSuite) SetupTest() {
	host := testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_HOST")
	port := testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_PORT")
	user := testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_USER")
	password := testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_PASSWORD")
	database := testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_DATABASE")

	s.connStr = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, database)
	logger, _ := zap.NewDevelopment()
	s.logger = logger

	// Clean up test tables from previous runs.
	s.cleanupTestTables()
}

func (s *ApplierIntegrationSuite) cleanupTestTables() {
	db, err := sql.Open("pgx", s.connStr)
	if err != nil {
		return
	}
	defer db.Close()
	_, _ = db.Exec("DROP TABLE IF EXISTS integration_test_users CASCADE")
	_, _ = db.Exec("DROP TABLE IF EXISTS integration_test_orders CASCADE")
	// The history lives in its own schema; the table in the managed schema is
	// the pre-move location, which the adoption path recreates.
	_, _ = db.Exec(fmt.Sprintf("DROP SCHEMA IF EXISTS %q CASCADE", pgRevisionsSchema))
	_, _ = db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", pgLegacyRevisionsQualified))
}

func (s *ApplierIntegrationSuite) tableExists(name string) bool {
	db, err := sql.Open("pgx", s.connStr)
	s.Require().NoError(err)
	defer db.Close()
	var exists bool
	err = db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = $1)`, name,
	).Scan(&exists)
	s.Require().NoError(err)
	return exists
}

func (s *ApplierIntegrationSuite) columnExists(table, column string) bool {
	db, err := sql.Open("pgx", s.connStr)
	s.Require().NoError(err)
	defer db.Close()
	var exists bool
	err = db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = $1 AND column_name = $2)`,
		table, column,
	).Scan(&exists)
	s.Require().NoError(err)
	return exists
}

// --- NewApplier ---

func (s *ApplierIntegrationSuite) Test_new_applier_connects_successfully() {
	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()
}

func (s *ApplierIntegrationSuite) Test_new_applier_invalid_connection_returns_error() {
	_, err := NewApplier("postgres", "postgres://bad:bad@localhost:1/nodb?sslmode=disable", s.logger)
	s.Assert().Error(err)
}

// --- EnsureDatabase ---

func (s *ApplierIntegrationSuite) Test_ensure_database_existing_is_idempotent() {
	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	// The test database itself already exists (created by docker-compose).
	err = applier.EnsureDatabase(context.Background(), "celerity_test")
	s.Assert().NoError(err)
}

// --- ExecSQL ---

func (s *ApplierIntegrationSuite) Test_exec_sql() {
	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	err = applier.ExecSQL(context.Background(), "SELECT 1")
	s.Assert().NoError(err)
}

func (s *ApplierIntegrationSuite) Test_exec_sql_invalid_returns_error() {
	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	err = applier.ExecSQL(context.Background(), "INVALID SQL STATEMENT")
	s.Assert().Error(err)
}

// --- ApplyDeclarative ---

func (s *ApplierIntegrationSuite) Test_apply_declarative_creates_tables() {
	dir := s.T().TempDir()
	schemaPath := filepath.Join(dir, "schema.yaml")
	s.Require().NoError(os.WriteFile(schemaPath, []byte(`
tables:
  integration_test_users:
    columns:
      id:
        type: bigint
        primaryKey: true
      name:
        type: varchar(100)
      email:
        type: varchar(255)
        unique: true
`), 0o644))

	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	s.Require().NoError(applier.ApplyDeclarative(context.Background(), schemaPath, "postgres"))
	s.Assert().True(s.tableExists("integration_test_users"))
}

func (s *ApplierIntegrationSuite) Test_apply_declarative_empty_path_is_noop() {
	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	s.Assert().NoError(applier.ApplyDeclarative(context.Background(), "", "postgres"))
}

func (s *ApplierIntegrationSuite) Test_apply_declarative_evolves_existing_schema() {
	dir := s.T().TempDir()
	v1 := filepath.Join(dir, "v1.yaml")
	v2 := filepath.Join(dir, "v2.yaml")
	s.Require().NoError(os.WriteFile(v1, []byte(`
tables:
  integration_test_users:
    columns:
      id:
        type: bigint
        primaryKey: true
      name:
        type: varchar(100)
`), 0o644))
	s.Require().NoError(os.WriteFile(v2, []byte(`
tables:
  integration_test_users:
    columns:
      id:
        type: bigint
        primaryKey: true
      name:
        type: varchar(100)
      tier:
        type: varchar(20)
        nullable: false
        default: "'free'"
`), 0o644))

	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	s.Require().NoError(applier.ApplyDeclarative(context.Background(), v1, "postgres"))
	s.Require().False(s.columnExists("integration_test_users", "tier"))

	// Re-applying the same desired state is a no-op (idempotent diff).
	s.Require().NoError(applier.ApplyDeclarative(context.Background(), v1, "postgres"))

	// Evolving the schema generates and applies the ALTER.
	s.Require().NoError(applier.ApplyDeclarative(context.Background(), v2, "postgres"))
	s.Assert().True(s.columnExists("integration_test_users", "tier"))
}

// --- ApplyMigrationDir ---

func (s *ApplierIntegrationSuite) Test_apply_migration_dir_applies_pending() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(
		filepath.Join(dir, "20260101000000_create_orders.sql"),
		[]byte("CREATE TABLE integration_test_orders (id bigint PRIMARY KEY, total numeric);"),
		0o644,
	))

	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	s.Require().NoError(applier.ApplyMigrationDir(context.Background(), dir, RiskBreaking))
	s.Assert().True(s.tableExists("integration_test_orders"))

	// atlas.sum is generated for a hand-placed directory.
	_, err = os.Stat(filepath.Join(dir, sumFileName))
	s.Assert().NoError(err)
}

func (s *ApplierIntegrationSuite) Test_apply_migration_dir_skips_already_applied() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(
		filepath.Join(dir, "20260101000000_init.sql"),
		[]byte("CREATE TABLE integration_test_orders (id bigint PRIMARY KEY);"),
		0o644,
	))

	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	// Applying twice must not error (second run finds no pending migrations).
	s.Require().NoError(applier.ApplyMigrationDir(context.Background(), dir, RiskBreaking))
	s.Require().NoError(applier.ApplyMigrationDir(context.Background(), dir, RiskBreaking))
	s.Assert().True(s.tableExists("integration_test_orders"))
}

func (s *ApplierIntegrationSuite) Test_apply_migration_dir_empty_is_noop() {
	dir := s.T().TempDir()

	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	s.Assert().NoError(applier.ApplyMigrationDir(context.Background(), dir, RiskBreaking))
}

// --- Non-transactional migrations ---

// Test_apply_non_transactional_migration proves the `-- atlas:txmode none`
// directive is honoured: CREATE INDEX CONCURRENTLY cannot run inside a
// transaction block, so it only succeeds when the migration runs
// non-transactionally. Statement-level resume after a partial failure is backed
// by the Applied/Total/PartialHashes fields the revision store persists.
func (s *ApplierIntegrationSuite) Test_apply_non_transactional_migration() {
	migrationsPath := s.T().TempDir()
	s.Require().NoError(os.WriteFile(
		filepath.Join(migrationsPath, "20260101000000_concurrent_index.sql"),
		[]byte("-- celerity:risk safe\n-- atlas:txmode none\n"+
			"CREATE TABLE integration_test_nt (id bigint PRIMARY KEY, sku text);\n"+
			"CREATE INDEX CONCURRENTLY integration_test_nt_sku ON integration_test_nt (sku);\n"),
		0o644,
	))

	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	s.Require().NoError(applier.ApplyMigrationDir(context.Background(), migrationsPath, RiskBreaking))
	s.Assert().True(s.tableExists("integration_test_nt"))
}

// --- Apply (migrations + declarative dev sync) ---

func (s *ApplierIntegrationSuite) Test_apply_uses_migrations_alone_once_any_exist() {
	dir := s.T().TempDir()
	schemaPath := filepath.Join(dir, "schema.yaml")
	migrationsPath := filepath.Join(dir, "migrations")
	s.Require().NoError(os.MkdirAll(migrationsPath, 0o755))

	// The migration creates orders and the schema YAML additionally describes
	// users, a change no migration has captured yet. Once a migration directory
	// exists it is the only source of schema changes locally, so the local
	// database comes up from exactly the history every other environment runs
	// and the uncaptured edit is not applied. A dev session warns about it and
	// points at `celerity schema diff` rather than quietly applying it, because
	// a local database that has had schema YAML applied on top of the history
	// no longer matches what a deploy would produce.
	s.Require().NoError(os.WriteFile(schemaPath, []byte(`
tables:
  integration_test_orders:
    columns:
      id:
        type: bigint
        primaryKey: true
  integration_test_users:
    columns:
      id:
        type: bigint
        primaryKey: true
`), 0o644))
	s.Require().NoError(os.WriteFile(
		filepath.Join(migrationsPath, "20260101000000_create_orders.sql"),
		[]byte("CREATE TABLE integration_test_orders (id bigint PRIMARY KEY);"),
		0o644,
	))

	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	s.Require().NoError(applier.Apply(context.Background(), schemaPath, migrationsPath, "postgres"))
	s.Assert().True(s.tableExists("integration_test_orders"), "created by the migration")
	s.Assert().False(
		s.tableExists("integration_test_users"),
		"schema YAML not yet captured in a migration must not be applied once migrations exist",
	)
}

func (s *ApplierIntegrationSuite) Test_apply_falls_back_to_declarative_without_migrations() {
	dir := s.T().TempDir()
	schemaPath := filepath.Join(dir, "schema.yaml")
	s.Require().NoError(os.WriteFile(schemaPath, []byte(`
tables:
  integration_test_users:
    columns:
      id:
        type: bigint
        primaryKey: true
`), 0o644))

	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	// No migrations directory → declarative apply from the schema YAML.
	s.Require().NoError(applier.Apply(context.Background(), schemaPath, filepath.Join(dir, "migrations"), "postgres"))
	s.Assert().True(s.tableExists("integration_test_users"))
}

// --- ForDatabase ---

func (s *ApplierIntegrationSuite) Test_for_database_connects_to_different_db() {
	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	// ForDatabase to the same test database should work.
	dbApplier, err := applier.ForDatabase("celerity_test")
	s.Require().NoError(err)
	defer dbApplier.Close()

	err = dbApplier.ExecSQL(context.Background(), "SELECT 1")
	s.Assert().NoError(err)
}

// --- risk gate ---

func (s *ApplierIntegrationSuite) Test_apply_migration_dir_gates_breaking() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(
		filepath.Join(dir, "20260101000000_create.sql"),
		[]byte("-- celerity:risk safe\nCREATE TABLE integration_test_orders (id bigint PRIMARY KEY, legacy varchar(10));"),
		0o644,
	))
	s.Require().NoError(os.WriteFile(
		filepath.Join(dir, "20260101000001_drop.sql"),
		[]byte("-- celerity:risk breaking\nALTER TABLE integration_test_orders DROP COLUMN legacy;"),
		0o644,
	))

	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	// Default (safe) applies the safe migration but stops at the breaking one.
	err = applier.ApplyMigrationDir(context.Background(), dir, RiskSafe)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "breaking")
	s.Assert().True(s.tableExists("integration_test_orders"), "safe migration applied")
	s.Assert().True(s.columnExists("integration_test_orders", "legacy"), "breaking migration NOT applied")

	// --allow-breaking applies it.
	s.Require().NoError(applier.ApplyMigrationDir(context.Background(), dir, RiskBreaking))
	s.Assert().False(s.columnExists("integration_test_orders", "legacy"))
}

func (s *ApplierIntegrationSuite) Test_apply_migration_dir_backstops_spoofed_header() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(
		filepath.Join(dir, "20260101000000_create.sql"),
		[]byte("-- celerity:risk safe\nCREATE TABLE integration_test_orders (id bigint PRIMARY KEY, legacy varchar(10));"),
		0o644,
	))
	// The header lies (safe) but the SQL drops a column, so the statement
	// backstop must still block it under the default gate.
	s.Require().NoError(os.WriteFile(
		filepath.Join(dir, "20260101000001_drop.sql"),
		[]byte("-- celerity:risk safe\nALTER TABLE integration_test_orders DROP COLUMN legacy;"),
		0o644,
	))

	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	err = applier.ApplyMigrationDir(context.Background(), dir, RiskSafe)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "breaking")
	s.Assert().True(s.columnExists("integration_test_orders", "legacy"), "spoofed-safe drop must NOT be applied")

	// Even --allow-risky is not enough, since a data-destructive statement needs
	// --allow-breaking.
	err = applier.ApplyMigrationDir(context.Background(), dir, RiskRisky)
	s.Require().Error(err)
	s.Assert().True(s.columnExists("integration_test_orders", "legacy"))

	s.Require().NoError(applier.ApplyMigrationDir(context.Background(), dir, RiskBreaking))
	s.Assert().False(s.columnExists("integration_test_orders", "legacy"))
}
