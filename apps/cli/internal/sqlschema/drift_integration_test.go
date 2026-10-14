package sqlschema

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/newstack-cloud/celerity/apps/cli/internal/testutils"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// DriftIntegrationSuite exercises DetectDrift end-to-end against a real Postgres
// server. The primary test database plays the "live" database; each test also
// provisions a throwaway dev database on the same server to reconstruct the
// expected state from the applied migrations.
type DriftIntegrationSuite struct {
	suite.Suite
	host, port, user, password, database string
	primaryConn                          string
	devDatabases                         []string
}

var driftDevCounter int64

func TestDriftIntegrationSuite(t *testing.T) {
	suite.Run(t, new(DriftIntegrationSuite))
}

func (s *DriftIntegrationSuite) SetupTest() {
	s.host = testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_HOST")
	s.port = testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_PORT")
	s.user = testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_USER")
	s.password = testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_PASSWORD")
	s.database = testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_DATABASE")
	s.primaryConn = FormatConnectionString(s.host, s.port, s.user, s.password, s.database)
	s.reset(s.primaryConn)
}

func (s *DriftIntegrationSuite) TearDownTest() {
	admin, err := NewApplier("postgres", s.primaryConn, zap.NewNop())
	if err != nil {
		return
	}
	defer admin.Close()
	for _, name := range s.devDatabases {
		_ = admin.ExecSQL(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name))
	}
	s.devDatabases = nil
}

// reset returns the public schema of a database to empty.
func (s *DriftIntegrationSuite) reset(connStr string) {
	appl, err := NewApplier("postgres", connStr, zap.NewNop())
	s.Require().NoError(err)
	defer appl.Close()
	s.Require().NoError(appl.ExecSQL(context.Background(), "DROP SCHEMA public CASCADE"))
	s.Require().NoError(appl.ExecSQL(context.Background(), "CREATE SCHEMA public"))
	// The migration history lives in its own schema, outside the one the reset
	// above clears.
	s.Require().NoError(appl.ExecSQL(context.Background(),
		fmt.Sprintf("DROP SCHEMA IF EXISTS %q CASCADE", pgRevisionsSchema)))
}

// freshDevDB creates a clean, uniquely-named database on the same server and
// returns its connection string.
func (s *DriftIntegrationSuite) freshDevDB() string {
	name := fmt.Sprintf("celerity_drift_dev_%d", atomic.AddInt64(&driftDevCounter, 1))
	admin, err := NewApplier("postgres", s.primaryConn, zap.NewNop())
	s.Require().NoError(err)
	defer admin.Close()
	_ = admin.ExecSQL(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name))
	s.Require().NoError(admin.EnsureDatabase(context.Background(), name))
	s.devDatabases = append(s.devDatabases, name)
	return FormatConnectionString(s.host, s.port, s.user, s.password, name)
}

func (s *DriftIntegrationSuite) writeSchemaFile(content string) string {
	path := filepath.Join(s.T().TempDir(), "schema.yaml")
	s.Require().NoError(os.WriteFile(path, []byte(content), 0o644))
	return path
}

const driftBaseSchema = `
tables:
  orders:
    columns:
      id:
        type: bigint
        primaryKey: true
      total_cents:
        type: integer
        nullable: false
`

// migrateLive generates a migration for the given schema and applies it to the
// primary (live) database, returning the migrations directory.
func (s *DriftIntegrationSuite) migrateLive(schemaContent string) string {
	ctx := context.Background()
	migrationsDir := s.T().TempDir()
	schemaPath := s.writeSchemaFile(schemaContent)

	// Generate the migration on its own clean dev database.
	genDev := s.freshDevDB()
	_, err := GenerateMigration(ctx, genDev, schemaPath, migrationsDir, "postgres", "init")
	s.Require().NoError(err)

	// Apply it to the live database.
	live, err := NewApplier("postgres", s.primaryConn, zap.NewNop())
	s.Require().NoError(err)
	defer live.Close()
	s.Require().NoError(live.ApplyMigrationDir(ctx, migrationsDir, RiskBreaking))
	return migrationsDir
}

func (s *DriftIntegrationSuite) detect(migrationsDir, devConn string) *DriftReport {
	live, err := NewApplier("postgres", s.primaryConn, zap.NewNop())
	s.Require().NoError(err)
	defer live.Close()
	report, err := DetectDrift(context.Background(), live, devConn, migrationsDir, "postgres")
	s.Require().NoError(err)
	return report
}

func (s *DriftIntegrationSuite) execLive(sqlStmt string) {
	live, err := NewApplier("postgres", s.primaryConn, zap.NewNop())
	s.Require().NoError(err)
	defer live.Close()
	s.Require().NoError(live.ExecSQL(context.Background(), sqlStmt))
}

func (s *DriftIntegrationSuite) Test_no_drift_when_live_matches_migrations() {
	migrationsDir := s.migrateLive(driftBaseSchema)

	report := s.detect(migrationsDir, s.freshDevDB())
	s.Assert().False(report.HasDrift(), "expected no drift, got: %+v", report)
}

func (s *DriftIntegrationSuite) Test_structural_drift_out_of_band_column() {
	migrationsDir := s.migrateLive(driftBaseSchema)

	// An out-of-band hotfix adds a column no migration defines.
	s.execLive("ALTER TABLE orders ADD COLUMN discount_cents integer")

	report := s.detect(migrationsDir, s.freshDevDB())
	s.Require().True(report.HasDrift())
	s.Require().NotEmpty(report.Structural)
	joined := ""
	for _, l := range report.Structural {
		joined += l + "\n"
	}
	s.Assert().Contains(joined, "discount_cents")
	s.Assert().Contains(joined, "exists in the database but not in the migrations")
}

func (s *DriftIntegrationSuite) Test_advanced_drift_out_of_band_view() {
	migrationsDir := s.migrateLive(driftBaseSchema)

	// An out-of-band view is an advanced object the structural diff does not cover.
	s.execLive("CREATE VIEW big_orders AS SELECT id FROM orders WHERE total_cents > 10000")

	report := s.detect(migrationsDir, s.freshDevDB())
	s.Require().True(report.HasDrift())
	s.Require().NotEmpty(report.Advanced)
	joined := ""
	for _, l := range report.Advanced {
		joined += l + "\n"
	}
	s.Assert().Contains(joined, "big_orders")
	s.Assert().Contains(joined, "created out-of-band")
}

func (s *DriftIntegrationSuite) Test_history_drift_when_applied_file_removed() {
	migrationsDir := s.migrateLive(driftBaseSchema)

	// Remove the applied migration file from the directory to simulate a history
	// that no longer matches the recorded revisions.
	entries, err := os.ReadDir(migrationsDir)
	s.Require().NoError(err)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".sql" {
			s.Require().NoError(os.Remove(filepath.Join(migrationsDir, e.Name())))
		}
	}

	// History drift needs no dev database.
	report := s.detect(migrationsDir, "")
	s.Require().True(report.HasDrift())
	s.Require().NotEmpty(report.History)
	s.Assert().Contains(report.History[0], "missing from the migration directory")
}
