package sqlschema

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/newstack-cloud/celerity/apps/cli/internal/testutils"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// MySQLIntegrationSuite exercises the MySQL dialect end-to-end against a real
// MySQL server: schema diff, apply, history, inspect and drift.
type MySQLIntegrationSuite struct {
	suite.Suite
	host, port, user, password, database string
	devDatabases                         []string
}

var mysqlDevCounter int64

func TestMySQLIntegrationSuite(t *testing.T) {
	suite.Run(t, new(MySQLIntegrationSuite))
}

func (s *MySQLIntegrationSuite) SetupTest() {
	// MySQL is gated off for v0; enable it for the duration of these tests.
	enableMySQLForTest(s.T())
	s.host = testutils.RequireEnv(s.T(), "CELERITY_TEST_MYSQL_HOST")
	s.port = testutils.RequireEnv(s.T(), "CELERITY_TEST_MYSQL_PORT")
	s.user = testutils.RequireEnv(s.T(), "CELERITY_TEST_MYSQL_USER")
	s.password = testutils.RequireEnv(s.T(), "CELERITY_TEST_MYSQL_PASSWORD")
	s.database = testutils.RequireEnv(s.T(), "CELERITY_TEST_MYSQL_DATABASE")
	s.resetTables(s.dsn(s.database))
}

func (s *MySQLIntegrationSuite) TearDownTest() {
	admin, err := NewApplier("mysql", s.dsn(s.database), zap.NewNop())
	if err != nil {
		return
	}
	defer admin.Close()
	for _, name := range s.devDatabases {
		_ = admin.ExecSQL(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", name))
	}
	s.devDatabases = nil
}

func (s *MySQLIntegrationSuite) dsn(db string) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", s.user, s.password, s.host, s.port, db)
}

// resetTables drops every table in the connected database, leaving it empty.
func (s *MySQLIntegrationSuite) resetTables(dsn string) {
	appl, err := NewApplier("mysql", dsn, zap.NewNop())
	s.Require().NoError(err)
	defer appl.Close()
	ctx := context.Background()

	rows, err := appl.db.QueryContext(ctx,
		"SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE()")
	s.Require().NoError(err)
	var tables []string
	for rows.Next() {
		var name string
		s.Require().NoError(rows.Scan(&name))
		tables = append(tables, name)
	}
	s.Require().NoError(rows.Err())
	rows.Close()

	if len(tables) == 0 {
		return
	}
	s.Require().NoError(appl.ExecSQL(ctx, "SET FOREIGN_KEY_CHECKS=0"))
	for _, t := range tables {
		s.Require().NoError(appl.ExecSQL(ctx, fmt.Sprintf("DROP TABLE IF EXISTS `%s`", t)))
	}
	s.Require().NoError(appl.ExecSQL(ctx, "SET FOREIGN_KEY_CHECKS=1"))
}

func (s *MySQLIntegrationSuite) freshDevDB() string {
	name := fmt.Sprintf("celerity_mysql_dev_%d", atomic.AddInt64(&mysqlDevCounter, 1))
	admin, err := NewApplier("mysql", s.dsn(s.database), zap.NewNop())
	s.Require().NoError(err)
	defer admin.Close()
	ctx := context.Background()
	s.Require().NoError(admin.ExecSQL(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", name)))
	s.Require().NoError(admin.ExecSQL(ctx, fmt.Sprintf("CREATE DATABASE `%s`", name)))
	s.devDatabases = append(s.devDatabases, name)
	return s.dsn(name)
}

func (s *MySQLIntegrationSuite) writeSchema(content string) (string, string) {
	dir := s.T().TempDir()
	path := filepath.Join(dir, "schema.yaml")
	s.Require().NoError(os.WriteFile(path, []byte(content), 0o644))
	return path, s.T().TempDir()
}

const mysqlSchema = `
tables:
  customers:
    columns:
      id:
        type: bigint
        primaryKey: true
      email:
        type: varchar(255)
  orders:
    columns:
      id:
        type: bigint
        primaryKey: true
      total_cents:
        type: int
        nullable: false
`

func (s *MySQLIntegrationSuite) Test_full_flow_diff_apply_history_show() {
	ctx := context.Background()
	schemaPath, migrationsDir := s.writeSchema(mysqlSchema)

	// Generate a migration on a throwaway dev database.
	path, err := GenerateMigration(ctx, s.freshDevDB(), schemaPath, migrationsDir, "mysql", "init")
	s.Require().NoError(err)
	s.Require().NotEmpty(path)
	content, err := os.ReadFile(path)
	s.Require().NoError(err)
	s.Assert().Contains(string(content), "CREATE TABLE")

	// Apply it to the live database.
	live, err := NewApplier("mysql", s.dsn(s.database), zap.NewNop())
	s.Require().NoError(err)
	defer live.Close()
	s.Require().NoError(live.ApplyMigrationDir(ctx, migrationsDir, RiskBreaking))

	// History records the applied migration.
	records, err := ReadMigrationHistory(ctx, live)
	s.Require().NoError(err)
	s.Require().Len(records, 1)
	s.Assert().Equal("init", records[0].Description)
	s.Assert().False(records[0].Failed)

	// Show reflects the applied tables.
	shown, err := InspectCurrentSchema(ctx, live)
	s.Require().NoError(err)
	s.Assert().Contains(shown, "customers")
	s.Assert().Contains(shown, "orders")

	// Re-applying is a no-op (idempotent).
	s.Require().NoError(live.ApplyMigrationDir(ctx, migrationsDir, RiskBreaking))
	records, err = ReadMigrationHistory(ctx, live)
	s.Require().NoError(err)
	s.Assert().Len(records, 1)
}

func (s *MySQLIntegrationSuite) Test_structural_drift() {
	ctx := context.Background()
	schemaPath, migrationsDir := s.writeSchema(mysqlSchema)

	_, err := GenerateMigration(ctx, s.freshDevDB(), schemaPath, migrationsDir, "mysql", "init")
	s.Require().NoError(err)

	live, err := NewApplier("mysql", s.dsn(s.database), zap.NewNop())
	s.Require().NoError(err)
	defer live.Close()
	s.Require().NoError(live.ApplyMigrationDir(ctx, migrationsDir, RiskBreaking))

	// No drift when the live database matches the migrations.
	report, err := DetectDrift(ctx, live, s.freshDevDB(), migrationsDir, "mysql")
	s.Require().NoError(err)
	s.Assert().False(report.HasDrift(), "unexpected drift: %+v", report)

	// An out-of-band column is structural drift.
	s.Require().NoError(live.ExecSQL(ctx, "ALTER TABLE orders ADD COLUMN discount_cents int"))
	report, err = DetectDrift(ctx, live, s.freshDevDB(), migrationsDir, "mysql")
	s.Require().NoError(err)
	s.Require().True(report.HasDrift())
	s.Assert().Contains(strings.Join(report.Structural, "\n"), "discount_cents")
}
