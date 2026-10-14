package sqlschema

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type MigrationVersionSuite struct {
	suite.Suite
}

func TestMigrationVersionSuite(t *testing.T) {
	suite.Run(t, new(MigrationVersionSuite))
}

func (s *MigrationVersionSuite) Test_parse_version() {
	s.Assert().Equal("20260721091215", parseVersion("20260721091215_init.sql"))
	s.Assert().Equal("", parseVersion("atlas.sum"))
	s.Assert().Equal("", parseVersion("not_a_version.sql"))
}

func (s *MigrationVersionSuite) Test_next_available_version_uses_base_when_empty_dir() {
	dir := s.T().TempDir()
	base := time.Date(2026, 7, 21, 9, 12, 15, 0, time.UTC)
	v, err := nextAvailableVersion(dir, base)
	s.Require().NoError(err)
	s.Assert().Equal("20260721091215", v)
}

func (s *MigrationVersionSuite) Test_next_available_version_bumps_on_collision() {
	dir := s.T().TempDir()
	// Existing migration at the same second the base falls on.
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "20260721091215_init.sql"), []byte("-- x"), 0o644))

	base := time.Date(2026, 7, 21, 9, 12, 15, 0, time.UTC)
	v, err := nextAvailableVersion(dir, base)
	s.Require().NoError(err)
	s.Assert().Equal("20260721091216", v, "must be strictly greater than the existing version")
}

func (s *MigrationVersionSuite) Test_scaffold_two_in_same_second_do_not_collide() {
	dir := s.T().TempDir()
	first, err := ScaffoldMigration(dir, "one")
	s.Require().NoError(err)
	second, err := ScaffoldMigration(dir, "two")
	s.Require().NoError(err)
	s.Assert().NotEqual(
		parseVersion(filepath.Base(first)),
		parseVersion(filepath.Base(second)),
		"back-to-back scaffolds must get distinct versions",
	)
}

type MigrationFileNameSuite struct {
	suite.Suite
}

func TestMigrationFileNameSuite(t *testing.T) {
	suite.Run(t, new(MigrationFileNameSuite))
}

func (s *MigrationFileNameSuite) Test_generated_names_are_accepted() {
	dir := s.T().TempDir()
	_, err := ScaffoldMigration(dir, "add audit trigger")
	s.Require().NoError(err)
	_, err = ScaffoldMigration(dir, "add search index")
	s.Require().NoError(err)

	s.Assert().NoError(ValidateMigrationFileNames(dir))
}

func (s *MigrationFileNameSuite) Test_flyway_style_names_are_rejected() {
	// A Flyway-style pair shares one version, which Atlas resolves deep inside
	// the executor as an out-of-range panic rather than an error.
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(
		filepath.Join(dir, "V001__add_search_index.up.sql"), []byte("CREATE INDEX i ON t (c);"), 0o644))
	s.Require().NoError(os.WriteFile(
		filepath.Join(dir, "V001__add_search_index.down.sql"), []byte("DROP INDEX i;"), 0o644))

	err := ValidateMigrationFileNames(dir)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "naming convention")
}

func (s *MigrationFileNameSuite) Test_duplicate_versions_are_rejected() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(
		filepath.Join(dir, "20260721091215_init.sql"), []byte("SELECT 1;"), 0o644))
	s.Require().NoError(os.WriteFile(
		filepath.Join(dir, "20260721091215_other.sql"), []byte("SELECT 1;"), 0o644))

	err := ValidateMigrationFileNames(dir)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "share the version")
}

func (s *MigrationFileNameSuite) Test_rehash_after_hand_editing() {
	dir := s.T().TempDir()
	path, err := ScaffoldMigration(dir, "add search index")
	s.Require().NoError(err)

	// Editing the scaffolded file invalidates the sum written when it was created.
	s.Require().NoError(os.WriteFile(path, []byte(
		"-- celerity:risk safe\nCREATE INDEX idx ON audit_log USING gin (details);\n"), 0o644))
	s.Require().Error(ValidateMigrationDir(dir))

	s.Require().NoError(RehashMigrationDir(dir))
	s.Assert().NoError(ValidateMigrationDir(dir))
}

func (s *MigrationFileNameSuite) Test_rehash_rejects_a_misnamed_directory() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(
		filepath.Join(dir, "V001__init.up.sql"), []byte("SELECT 1;"), 0o644))

	err := RehashMigrationDir(dir)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "naming convention")
}
