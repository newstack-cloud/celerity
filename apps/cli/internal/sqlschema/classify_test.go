package sqlschema

import (
	"testing"

	"ariga.io/atlas/sql/schema"
	"github.com/stretchr/testify/suite"
)

type ClassifySuite struct {
	suite.Suite
}

func TestClassifySuite(t *testing.T) {
	suite.Run(t, new(ClassifySuite))
}

func col(name string, nullable bool) *schema.Column {
	return schema.NewColumn(name).SetType(&schema.StringType{T: "varchar"}).SetNull(nullable)
}

func (s *ClassifySuite) classify(changes ...schema.Change) RiskLevel {
	level, _ := classifyChanges(changes)
	return level
}

func (s *ClassifySuite) Test_additive_changes_are_safe() {
	s.Assert().Equal(RiskSafe, s.classify(&schema.AddTable{T: schema.NewTable("t")}))
	s.Assert().Equal(RiskSafe, s.classify(&schema.AddColumn{C: col("x", true)}))
	s.Assert().Equal(RiskSafe, s.classify(&schema.AddIndex{I: schema.NewIndex("i")}))
	s.Assert().Equal(RiskSafe, s.classify(&schema.DropIndex{I: schema.NewIndex("i")}))
}

func (s *ClassifySuite) Test_data_dependent_changes_are_risky() {
	// NOT NULL column without a default.
	s.Assert().Equal(RiskRisky, s.classify(&schema.AddColumn{C: col("x", false)}))
	// Making an existing column NOT NULL.
	s.Assert().Equal(RiskRisky, s.classify(&schema.ModifyColumn{
		From: col("x", true), To: col("x", false), Change: schema.ChangeNull,
	}))
	// Changing a column's type.
	s.Assert().Equal(RiskRisky, s.classify(&schema.ModifyColumn{
		From: col("x", true), To: col("x", true), Change: schema.ChangeType,
	}))
	// Unique index / foreign key / check.
	s.Assert().Equal(RiskRisky, s.classify(&schema.AddIndex{I: schema.NewUniqueIndex("i")}))
	s.Assert().Equal(RiskRisky, s.classify(&schema.AddForeignKey{F: schema.NewForeignKey("fk")}))
	s.Assert().Equal(RiskRisky, s.classify(&schema.AddCheck{C: &schema.Check{Name: "c"}}))
}

func (s *ClassifySuite) Test_destructive_changes_are_breaking() {
	s.Assert().Equal(RiskBreaking, s.classify(&schema.DropTable{T: schema.NewTable("t")}))
	s.Assert().Equal(RiskBreaking, s.classify(&schema.DropColumn{C: col("x", false)}))
}

func (s *ClassifySuite) Test_highest_risk_wins() {
	level := s.classify(
		&schema.AddColumn{C: col("safe", true)},
		&schema.DropColumn{C: col("gone", false)},
	)
	s.Assert().Equal(RiskBreaking, level)
}

func (s *ClassifySuite) Test_modify_table_recurses() {
	mt := &schema.ModifyTable{
		T:       schema.NewTable("t"),
		Changes: []schema.Change{&schema.DropColumn{C: col("x", false)}},
	}
	s.Assert().Equal(RiskBreaking, s.classify(mt))
}

func (s *ClassifySuite) Test_sql_floor_flags_data_destructive_statements() {
	breaking := []string{
		"DROP TABLE users",
		"DROP TABLE IF EXISTS users",
		"DROP SCHEMA public CASCADE",
		"ALTER TABLE users DROP COLUMN email",
		"ALTER TABLE users DROP COLUMN IF EXISTS email",
		"ALTER TABLE users DROP email", // bare column drop (COLUMN keyword omitted)
	}
	for _, stmt := range breaking {
		level, reason := sqlRiskFloor([]string{stmt})
		s.Assert().Equal(RiskBreaking, level, stmt)
		s.Assert().NotEmpty(reason, stmt)
	}

	safe := []string{
		"CREATE TABLE t (id bigint)",
		"ALTER TABLE users ADD COLUMN nickname varchar(50)",
		"ALTER TABLE users DROP CONSTRAINT chk_total",
		"ALTER TABLE users ALTER COLUMN x DROP NOT NULL",
		"ALTER TABLE users ALTER COLUMN x DROP DEFAULT",
		"DROP INDEX idx_users_email",
		"-- DROP TABLE users\nCREATE TABLE t (id bigint)", // drop only in a comment
	}
	for _, stmt := range safe {
		level, _ := sqlRiskFloor([]string{stmt})
		s.Assert().Equal(RiskSafe, level, stmt)
	}
}

func (s *ClassifySuite) Test_effective_risk_backstops_spoofed_header() {
	// A "safe" header on SQL that drops data is treated as breaking.
	level, detail := effectiveRisk([]byte("-- celerity:risk safe\n"), []string{"ALTER TABLE users DROP COLUMN email"})
	s.Assert().Equal(RiskBreaking, level)
	s.Assert().NotEmpty(detail)

	// Header honoured when the SQL is not more destructive than it claims.
	level, detail = effectiveRisk([]byte("-- celerity:risk safe\n"), []string{"CREATE TABLE t (id bigint)"})
	s.Assert().Equal(RiskSafe, level)
	s.Assert().Empty(detail)

	// A header stricter than the SQL floor is kept (risky header, additive SQL).
	level, _ = effectiveRisk([]byte("-- celerity:risk risky\n"), []string{"CREATE TABLE t (id bigint)"})
	s.Assert().Equal(RiskRisky, level)
}

func (s *ClassifySuite) Test_migration_risk_header_parsing() {
	s.Assert().Equal(RiskSafe, migrationRisk([]byte("-- celerity:risk safe\nCREATE TABLE t (id bigint);")))
	s.Assert().Equal(RiskBreaking, migrationRisk([]byte("-- celerity:risk breaking  (drops column t.x)\nALTER TABLE t DROP COLUMN x;")))
	s.Assert().Equal(RiskRisky, migrationRisk([]byte("-- celerity:risk risky\n")))
	// Missing header → risky (fail-safe).
	s.Assert().Equal(RiskRisky, migrationRisk([]byte("CREATE TABLE t (id bigint);")))
	// Unrecognised level → risky.
	s.Assert().Equal(RiskRisky, migrationRisk([]byte("-- celerity:risk bogus\n")))
}
