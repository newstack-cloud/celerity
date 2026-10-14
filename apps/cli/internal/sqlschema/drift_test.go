package sqlschema

import (
	"testing"
	"time"

	"ariga.io/atlas/sql/migrate"
	atlasschema "ariga.io/atlas/sql/schema"
	"github.com/stretchr/testify/suite"
)

type DriftUnitSuite struct {
	suite.Suite
}

func TestDriftUnitSuite(t *testing.T) {
	suite.Run(t, new(DriftUnitSuite))
}

func rev(version, desc string, executedAt time.Time) *migrate.Revision {
	return &migrate.Revision{Version: version, Description: desc, ExecutedAt: executedAt}
}

func (s *DriftUnitSuite) Test_history_drift_clean() {
	base := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	revisions := []*migrate.Revision{
		rev("20260721100000", "init", base),
		rev("20260721100100", "add_tier", base.Add(time.Minute)),
	}
	dir := map[string]bool{"20260721100000": true, "20260721100100": true}

	s.Assert().Empty(historyDrift(revisions, dir))
}

func (s *DriftUnitSuite) Test_history_drift_applied_but_missing_from_dir() {
	base := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	revisions := []*migrate.Revision{
		rev("20260721100000", "init", base),
		rev("20260721100100", "hotfix", base.Add(time.Minute)),
	}
	// The hotfix migration is recorded as applied but absent from the directory.
	dir := map[string]bool{"20260721100000": true}

	lines := historyDrift(revisions, dir)
	s.Require().Len(lines, 1)
	s.Assert().Contains(lines[0], "20260721100100")
	s.Assert().Contains(lines[0], "missing from the migration directory")
}

func (s *DriftUnitSuite) Test_history_drift_out_of_order() {
	base := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	// The later version was applied before the earlier version.
	revisions := []*migrate.Revision{
		rev("20260721100000", "init", base.Add(time.Minute)),
		rev("20260721100100", "add_tier", base),
	}
	dir := map[string]bool{"20260721100000": true, "20260721100100": true}

	lines := historyDrift(revisions, dir)
	s.Require().Len(lines, 1)
	s.Assert().Contains(lines[0], "out of order")
}

func (s *DriftUnitSuite) Test_pending_migration_is_not_history_drift() {
	base := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	revisions := []*migrate.Revision{rev("20260721100000", "init", base)}
	// A second migration exists in the directory but has not been applied yet.
	dir := map[string]bool{"20260721100000": true, "20260721100100": true}

	s.Assert().Empty(historyDrift(revisions, dir))
}

func (s *DriftUnitSuite) Test_render_structural_added_column() {
	change := &atlasschema.ModifyTable{
		T: atlasschema.NewTable("orders"),
		Changes: []atlasschema.Change{
			&atlasschema.AddColumn{C: atlasschema.NewColumn("discount_cents")},
		},
	}
	lines := renderStructuralChanges([]atlasschema.Change{change})
	s.Require().Len(lines, 1)
	s.Assert().Contains(lines[0], "orders")
	s.Assert().Contains(lines[0], "discount_cents")
	s.Assert().Contains(lines[0], "exists in the database but not in the migrations")
}

func (s *DriftUnitSuite) Test_render_structural_added_table() {
	change := &atlasschema.AddTable{T: atlasschema.NewTable("audit_log")}
	lines := renderStructuralChanges([]atlasschema.Change{change})
	s.Require().Len(lines, 1)
	s.Assert().Contains(lines[0], "audit_log")
}

func (s *DriftUnitSuite) Test_render_structural_ignores_revision_table() {
	change := &atlasschema.AddTable{T: atlasschema.NewTable(revisionsTable)}
	s.Assert().Empty(renderStructuralChanges([]atlasschema.Change{change}))
}

func (s *DriftUnitSuite) Test_diff_fingerprints() {
	expected := map[string]string{
		`view "orders_summary"`:  "aaa",
		`trigger "orders.audit"`: "bbb",
	}
	live := map[string]string{
		`view "orders_summary"`:    "ccc", // changed
		`function "compute_total"`: "ddd", // unexpected
		// trigger orders.audit missing
	}

	lines := diffFingerprints(expected, live)
	s.Require().Len(lines, 3)
	joined := ""
	for _, l := range lines {
		joined += l + "\n"
	}
	s.Assert().Contains(joined, `function "compute_total"`)
	s.Assert().Contains(joined, "created out-of-band")
	s.Assert().Contains(joined, "has been changed out-of-band")
	s.Assert().Contains(joined, "missing from the database")
}

func (s *DriftUnitSuite) Test_report_has_drift_and_render() {
	report := &DriftReport{
		Resource:   "ordersDb",
		Target:     "postgres: orders",
		Structural: []string{"table orders: column discount_cents exists in the database but not in the migrations"},
	}
	s.Assert().True(report.HasDrift())

	empty := &DriftReport{Resource: "ordersDb", Target: "postgres: orders"}
	s.Assert().False(empty.HasDrift())
}
