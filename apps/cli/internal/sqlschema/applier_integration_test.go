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
	_, _ = db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", migrationsTable))
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

// --- ApplySchema ---

func (s *ApplierIntegrationSuite) Test_apply_schema_creates_tables() {
	dir := s.T().TempDir()
	schemaPath := filepath.Join(dir, "schema.yaml")
	schemaContent := `
tables:
  integration_test_users:
    columns:
      id:
        type: serial
        primaryKey: true
      name:
        type: varchar(100)
      email:
        type: varchar(255)
        unique: true
`
	s.Require().NoError(os.WriteFile(schemaPath, []byte(schemaContent), 0o644))

	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	err = applier.ApplySchema(context.Background(), schemaPath, "postgres")
	s.Require().NoError(err)

	// Verify table was created.
	db, err := sql.Open("pgx", s.connStr)
	s.Require().NoError(err)
	defer db.Close()

	var exists bool
	err = db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM information_schema.tables
			WHERE table_name = 'integration_test_users'
		)
	`).Scan(&exists)
	s.Require().NoError(err)
	s.Assert().True(exists)
}

func (s *ApplierIntegrationSuite) Test_apply_schema_empty_path_is_noop() {
	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	err = applier.ApplySchema(context.Background(), "", "postgres")
	s.Assert().NoError(err)
}

// --- ApplyMigrations ---

func (s *ApplierIntegrationSuite) Test_apply_migrations_runs_up_scripts() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(
		filepath.Join(dir, "V001__create_orders.up.sql"),
		[]byte("CREATE TABLE integration_test_orders (id serial PRIMARY KEY, total numeric);"),
		0o644,
	))
	s.Require().NoError(os.WriteFile(
		filepath.Join(dir, "V001__create_orders.down.sql"),
		[]byte("DROP TABLE IF EXISTS integration_test_orders;"),
		0o644,
	))

	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	err = applier.ApplyMigrations(context.Background(), dir)
	s.Require().NoError(err)

	// Verify table was created.
	db, err := sql.Open("pgx", s.connStr)
	s.Require().NoError(err)
	defer db.Close()

	var exists bool
	err = db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM information_schema.tables
			WHERE table_name = 'integration_test_orders'
		)
	`).Scan(&exists)
	s.Require().NoError(err)
	s.Assert().True(exists)
}

func (s *ApplierIntegrationSuite) Test_apply_migrations_skips_already_applied() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(
		filepath.Join(dir, "V001__initial.up.sql"),
		[]byte("CREATE TABLE IF NOT EXISTS integration_test_orders (id serial PRIMARY KEY);"),
		0o644,
	))

	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	// Apply twice.
	s.Require().NoError(applier.ApplyMigrations(context.Background(), dir))
	s.Require().NoError(applier.ApplyMigrations(context.Background(), dir))
}

func (s *ApplierIntegrationSuite) Test_apply_migrations_empty_dir_is_noop() {
	dir := s.T().TempDir()

	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	err = applier.ApplyMigrations(context.Background(), dir)
	s.Assert().NoError(err)
}

// --- RollbackMigrations ---

func (s *ApplierIntegrationSuite) Test_rollback_migrations_runs_down_scripts() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(
		filepath.Join(dir, "V001__create_orders.up.sql"),
		[]byte("CREATE TABLE integration_test_orders (id serial PRIMARY KEY);"),
		0o644,
	))
	s.Require().NoError(os.WriteFile(
		filepath.Join(dir, "V001__create_orders.down.sql"),
		[]byte("DROP TABLE IF EXISTS integration_test_orders;"),
		0o644,
	))

	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	// Apply first.
	s.Require().NoError(applier.ApplyMigrations(context.Background(), dir))

	// Then rollback.
	err = applier.RollbackMigrations(context.Background(), dir)
	s.Require().NoError(err)

	// Verify table was dropped.
	db, err := sql.Open("pgx", s.connStr)
	s.Require().NoError(err)
	defer db.Close()

	var exists bool
	err = db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM information_schema.tables
			WHERE table_name = 'integration_test_orders'
		)
	`).Scan(&exists)
	s.Require().NoError(err)
	s.Assert().False(exists)
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

// --- ApplyAll / RollbackAll ---

func (s *ApplierIntegrationSuite) Test_apply_all_and_rollback_all() {
	dir := s.T().TempDir()
	schemaDir := filepath.Join(dir, "schema")
	migrationsDir := filepath.Join(dir, "migrations")
	s.Require().NoError(os.MkdirAll(schemaDir, 0o755))
	s.Require().NoError(os.MkdirAll(migrationsDir, 0o755))

	schemaPath := filepath.Join(schemaDir, "schema.yaml")
	s.Require().NoError(os.WriteFile(schemaPath, []byte(`
tables:
  integration_test_users:
    columns:
      id:
        type: serial
        primaryKey: true
      name:
        type: varchar(100)
`), 0o644))

	s.Require().NoError(os.WriteFile(
		filepath.Join(migrationsDir, "V001__add_email.up.sql"),
		[]byte("ALTER TABLE integration_test_users ADD COLUMN email varchar(255);"),
		0o644,
	))
	s.Require().NoError(os.WriteFile(
		filepath.Join(migrationsDir, "V001__add_email.down.sql"),
		[]byte("ALTER TABLE integration_test_users DROP COLUMN IF EXISTS email;"),
		0o644,
	))

	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	// Apply schema + migrations.
	err = applier.ApplyAll(context.Background(), schemaPath, migrationsDir, "postgres")
	s.Require().NoError(err)

	// Verify email column exists.
	db, err := sql.Open("pgx", s.connStr)
	s.Require().NoError(err)
	defer db.Close()

	var exists bool
	err = db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'integration_test_users' AND column_name = 'email'
		)
	`).Scan(&exists)
	s.Require().NoError(err)
	s.Assert().True(exists)

	// Rollback.
	err = applier.RollbackAll(context.Background(), schemaPath, migrationsDir, "postgres")
	s.Require().NoError(err)
}
