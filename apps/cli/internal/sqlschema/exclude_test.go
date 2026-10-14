package sqlschema

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type ExcludeTestSuite struct {
	suite.Suite
}

func TestExcludeTestSuite(t *testing.T) {
	suite.Run(t, new(ExcludeTestSuite))
}

func (s *ExcludeTestSuite) Test_history_table_is_always_excluded() {
	patterns := excludePatterns(&Schema{})
	s.Assert().Contains(patterns, revisionsTable,
		"the migration-history table must never be visible to a YAML-driven diff")
}

func (s *ExcludeTestSuite) Test_declared_patterns_are_kept() {
	patterns := excludePatterns(&Schema{
		Exclude: []string{"audit_log.idx_audit_fts[type=index]", "legacy_*"},
	})
	s.Assert().Contains(patterns, "audit_log.idx_audit_fts[type=index]")
	s.Assert().Contains(patterns, "legacy_*")
}

func (s *ExcludeTestSuite) Test_valid_patterns_accepted() {
	err := validateExcludePatterns([]string{
		"legacy_events",
		"legacy_*",
		"audit_log.idx_audit_fts[type=index]",
		"audit_log.tmp_*[type=column]",
	})
	s.Assert().NoError(err)
}

func (s *ExcludeTestSuite) Test_unparsable_pattern_rejected() {
	err := validateExcludePatterns([]string{"audit_log.a.b.c.d"})
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "invalid exclude pattern")
}
