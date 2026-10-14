package sqlschema

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/newstack-cloud/celerity/apps/cli/internal/testutils"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// MigrationDirIntegrationSuite covers the schema diff/new engine. It uses the
// dev database purely as Atlas's replay target, so each test drops any objects
// it may have created and starts from an empty database.
type MigrationDirIntegrationSuite struct {
	suite.Suite
	devConnStr string
}

func TestMigrationDirIntegrationSuite(t *testing.T) {
	suite.Run(t, new(MigrationDirIntegrationSuite))
}

func (s *MigrationDirIntegrationSuite) SetupTest() {
	host := testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_HOST")
	port := testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_PORT")
	user := testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_USER")
	password := testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_PASSWORD")
	database := testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_DATABASE")
	s.devConnStr = FormatConnectionString(host, port, user, password, database)

	// Atlas requires a clean dev database to replay into.
	s.dropAll()
}

func (s *MigrationDirIntegrationSuite) dropAll() {
	// Reset the public schema so the dev database is completely clean, which is
	// what the Atlas planner requires to replay a migration directory into. A
	// real `schema diff` uses a fresh ephemeral container for the same reason.
	appl, err := NewApplier("postgres", s.devConnStr, zap.NewNop())
	s.Require().NoError(err)
	defer appl.Close()
	for _, stmt := range []string{
		"DROP SCHEMA public CASCADE",
		"CREATE SCHEMA public",
		// Atlas counts every schema in the database when deciding whether it is
		// clean, so the migration-history schema has to go too.
		fmt.Sprintf("DROP SCHEMA IF EXISTS %q CASCADE", pgRevisionsSchema),
	} {
		s.Require().NoError(appl.ExecSQL(context.Background(), stmt))
	}
}

func (s *MigrationDirIntegrationSuite) writeSchema(dir, name, content string) string {
	path := filepath.Join(dir, name)
	s.Require().NoError(os.WriteFile(path, []byte(content), 0o644))
	return path
}

func (s *MigrationDirIntegrationSuite) Test_scaffold_migration_creates_file_and_sum() {
	dir := s.T().TempDir()
	path, err := ScaffoldMigration(dir, "add audit trigger")
	s.Require().NoError(err)

	s.Assert().FileExists(path)
	s.Assert().Contains(filepath.Base(path), "_add_audit_trigger.sql")
	s.Assert().FileExists(filepath.Join(dir, sumFileName))
}

func (s *MigrationDirIntegrationSuite) Test_generate_migration_initial_creates_file() {
	migrationsDir := s.T().TempDir()
	schemaPath := s.writeSchema(s.T().TempDir(), "schema.yaml", `
tables:
  customers:
    columns:
      id:
        type: bigint
        primaryKey: true
      email:
        type: varchar(255)
`)

	path, err := GenerateMigration(context.Background(), s.devConnStr, schemaPath, migrationsDir, "postgres", "init")
	s.Require().NoError(err)
	s.Require().NotEmpty(path)
	s.Assert().FileExists(path)
	s.Assert().Contains(filepath.Base(path), "_init.sql")

	content, err := os.ReadFile(path)
	s.Require().NoError(err)
	s.Assert().Contains(string(content), "CREATE TABLE")
	s.Assert().FileExists(filepath.Join(migrationsDir, sumFileName))
}

func (s *MigrationDirIntegrationSuite) Test_generate_migration_no_change_returns_empty() {
	migrationsDir := s.T().TempDir()
	schemaDir := s.T().TempDir()
	schemaPath := s.writeSchema(schemaDir, "schema.yaml", `
tables:
  customers:
    columns:
      id:
        type: bigint
        primaryKey: true
`)

	first, err := GenerateMigration(context.Background(), s.devConnStr, schemaPath, migrationsDir, "postgres", "init")
	s.Require().NoError(err)
	s.Require().NotEmpty(first)

	// Re-running against the same schema produces no new migration.
	s.dropAll()
	second, err := GenerateMigration(context.Background(), s.devConnStr, schemaPath, migrationsDir, "postgres", "noop")
	s.Require().NoError(err)
	s.Assert().Empty(second, "no schema change should yield no migration file")
}

func (s *MigrationDirIntegrationSuite) Test_generate_migration_evolution_creates_alter() {
	migrationsDir := s.T().TempDir()
	schemaDir := s.T().TempDir()
	v1 := s.writeSchema(schemaDir, "v1.yaml", `
tables:
  customers:
    columns:
      id:
        type: bigint
        primaryKey: true
`)
	v2 := s.writeSchema(schemaDir, "v2.yaml", `
tables:
  customers:
    columns:
      id:
        type: bigint
        primaryKey: true
      tier:
        type: varchar(20)
        nullable: false
        default: "'free'"
`)

	_, err := GenerateMigration(context.Background(), s.devConnStr, v1, migrationsDir, "postgres", "init")
	s.Require().NoError(err)

	s.dropAll()
	altPath, err := GenerateMigration(context.Background(), s.devConnStr, v2, migrationsDir, "postgres", "add_tier")
	s.Require().NoError(err)
	s.Require().NotEmpty(altPath)

	content, err := os.ReadFile(altPath)
	s.Require().NoError(err)
	s.Assert().Contains(string(content), "ADD COLUMN")
	s.Assert().Contains(string(content), "tier")
}

func (s *MigrationDirIntegrationSuite) Test_generated_migrations_apply_cleanly() {
	migrationsDir := s.T().TempDir()
	schemaDir := s.T().TempDir()
	schemaPath := s.writeSchema(schemaDir, "schema.yaml", `
tables:
  customers:
    columns:
      id:
        type: bigint
        primaryKey: true
      email:
        type: varchar(255)
        unique: true
`)

	_, err := GenerateMigration(context.Background(), s.devConnStr, schemaPath, migrationsDir, "postgres", "init")
	s.Require().NoError(err)

	// A generated migration must apply against a clean database.
	s.dropAll()
	applier, err := NewApplier("postgres", s.devConnStr, zap.NewNop())
	s.Require().NoError(err)
	defer applier.Close()
	s.Require().NoError(applier.ApplyMigrationDir(context.Background(), migrationsDir, RiskBreaking))
}

func (s *MigrationDirIntegrationSuite) Test_check_pending_migration() {
	migrationsDir := s.T().TempDir()
	schemaDir := s.T().TempDir()
	v1 := s.writeSchema(schemaDir, "v1.yaml", `
tables:
  customers:
    columns:
      id:
        type: bigint
        primaryKey: true
      email:
        type: varchar(255)
`)

	// No migrations yet, but the schema defines tables → pending.
	s.dropAll()
	pending, err := CheckPendingMigration(context.Background(), s.devConnStr, v1, migrationsDir, "postgres")
	s.Require().NoError(err)
	s.Assert().True(pending, "an initial migration is needed")

	// Generate the migration → now up to date.
	s.dropAll()
	_, err = GenerateMigration(context.Background(), s.devConnStr, v1, migrationsDir, "postgres", "init")
	s.Require().NoError(err)
	s.dropAll()
	pending, err = CheckPendingMigration(context.Background(), s.devConnStr, v1, migrationsDir, "postgres")
	s.Require().NoError(err)
	s.Assert().False(pending, "committed migration covers the schema")

	// Change the schema without generating a migration → pending again.
	v2 := s.writeSchema(schemaDir, "v2.yaml", `
tables:
  customers:
    columns:
      id:
        type: bigint
        primaryKey: true
      email:
        type: varchar(255)
      tier:
        type: varchar(20)
`)
	s.dropAll()
	pending, err = CheckPendingMigration(context.Background(), s.devConnStr, v2, migrationsDir, "postgres")
	s.Require().NoError(err)
	s.Assert().True(pending, "the schema change is not yet captured by a migration")
}

func (s *MigrationDirIntegrationSuite) Test_generated_migration_stamps_risk_header() {
	migrationsDir := s.T().TempDir()
	schemaDir := s.T().TempDir()
	v1 := s.writeSchema(schemaDir, "v1.yaml", `
tables:
  customers:
    columns:
      id:
        type: bigint
        primaryKey: true
      legacy:
        type: varchar(10)
`)
	v2 := s.writeSchema(schemaDir, "v2.yaml", `
tables:
  customers:
    columns:
      id:
        type: bigint
        primaryKey: true
`)

	// Creating a table is additive → safe.
	p1, err := GenerateMigration(context.Background(), s.devConnStr, v1, migrationsDir, "postgres", "init")
	s.Require().NoError(err)
	risk1, err := ReadMigrationRisk(p1)
	s.Require().NoError(err)
	s.Assert().Equal(RiskSafe, risk1)

	// Dropping a column is destructive → breaking.
	s.dropAll()
	p2, err := GenerateMigration(context.Background(), s.devConnStr, v2, migrationsDir, "postgres", "drop_legacy")
	s.Require().NoError(err)
	risk2, err := ReadMigrationRisk(p2)
	s.Require().NoError(err)
	s.Assert().Equal(RiskBreaking, risk2)
}
