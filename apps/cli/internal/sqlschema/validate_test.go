package sqlschema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ValidateSuite struct {
	suite.Suite
}

func TestValidateSuite(t *testing.T) {
	suite.Run(t, new(ValidateSuite))
}

func (s *ValidateSuite) writeFile(dir, name, content string) string {
	path := filepath.Join(dir, name)
	s.Require().NoError(os.WriteFile(path, []byte(content), 0o644))
	return path
}

func (s *ValidateSuite) Test_validate_schema_valid() {
	path := s.writeFile(s.T().TempDir(), "schema.yaml", `
tables:
  customers:
    columns:
      id:
        type: bigint
        primaryKey: true
  orders:
    columns:
      id:
        type: bigint
        primaryKey: true
      customer_id:
        type: bigint
        references:
          table: customers
          column: id
`)
	s.Assert().NoError(ValidateSchema(path, "postgres"))
}

func (s *ValidateSuite) Test_validate_schema_bad_foreign_key() {
	path := s.writeFile(s.T().TempDir(), "schema.yaml", `
tables:
  orders:
    columns:
      customer_id:
        type: bigint
        references:
          table: missing
          column: id
`)
	err := ValidateSchema(path, "postgres")
	s.Assert().ErrorContains(err, "unknown table")
}

func (s *ValidateSuite) Test_validate_schema_unparseable() {
	path := s.writeFile(s.T().TempDir(), "schema.yaml", "this: [is: not: valid")
	s.Assert().Error(ValidateSchema(path, "postgres"))
}

func (s *ValidateSuite) Test_validate_migration_dir_empty_is_ok() {
	s.Assert().NoError(ValidateMigrationDir(s.T().TempDir()))
}

func (s *ValidateSuite) Test_validate_migration_dir_valid_sum() {
	dir := s.T().TempDir()
	_, err := ScaffoldMigration(dir, "init")
	s.Require().NoError(err)
	s.Assert().NoError(ValidateMigrationDir(dir))
}

func (s *ValidateSuite) Test_validate_migration_dir_missing_sum() {
	dir := s.T().TempDir()
	s.writeFile(dir, "20260101000000_init.sql", "CREATE TABLE t (id bigint);")
	// No atlas.sum written.
	s.Assert().Error(ValidateMigrationDir(dir))
}

func (s *ValidateSuite) Test_validate_migration_dir_tampered_file() {
	dir := s.T().TempDir()
	created, err := ScaffoldMigration(dir, "init")
	s.Require().NoError(err)
	// Modify the file after the sum was written.
	s.Require().NoError(os.WriteFile(created, []byte("CREATE TABLE tampered (id bigint);"), 0o644))
	s.Assert().Error(ValidateMigrationDir(dir))
}
