package sqlschema

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"ariga.io/atlas/sql/migrate"
	atlasschema "ariga.io/atlas/sql/schema"
)

// DriftReport describes how a live database has diverged from the state its
// committed migrations should have produced. It is reported at three layers,
// matching the schema-management spec:
//
//   - History: the revision table vs the migration directory (migrations applied
//     out-of-band, missing from the directory, or applied out of order).
//   - Structural: the live tables/columns/indexes/keys/constraints vs the
//     expected state (everything the schema YAML models).
//   - Advanced: triggers/functions/views/RLS policies fingerprinted from the
//     catalog (objects created by hand-written migrations), reported coarsely as
//     changed / missing / unexpected.
//
// Each layer is a list of human-readable lines; empty means no drift at that
// layer.
type DriftReport struct {
	Resource   string
	Target     string
	History    []string
	Structural []string
	Advanced   []string
	// StructuralTables lists the tables that drifted structurally, for schema
	// contract evaluation. Contracts, when populated by the caller, are rendered
	// in the report.
	StructuralTables []string
	Contracts        []ContractImpact
}

// HasDrift reports whether any layer found drift.
func (r *DriftReport) HasDrift() bool {
	return len(r.History)+len(r.Structural)+len(r.Advanced) > 0
}

// Render writes a human-readable report to w.
func (r *DriftReport) Render(w io.Writer) {
	if !r.HasDrift() {
		fmt.Fprintf(w, "No drift detected for %s (%s).\n", r.Resource, r.Target)
		return
	}

	fmt.Fprintf(w, "Drift detected for %s (%s)\n", r.Resource, r.Target)
	renderDriftSection(w, "History drift", r.History)
	renderDriftSection(w, "Structural drift", r.Structural)
	renderDriftSection(w, "Advanced-object drift", r.Advanced)

	RenderContractImpacts(w, r.Contracts)

	fmt.Fprintln(w, "\n  Resolution:")
	fmt.Fprintln(w, "    - To adopt a change: add it to the schema YAML, run `celerity schema diff`")
	fmt.Fprintln(w, "      (generated statements are guarded so they no-op where the object already")
	fmt.Fprintln(w, "      exists), then apply with --allow-drift.")
	fmt.Fprintln(w, "    - To remove a change: create a hand-written migration reverting it")
	fmt.Fprintln(w, "      (`celerity schema new`), then apply with --allow-drift.")
}

func renderDriftSection(w io.Writer, title string, lines []string) {
	if len(lines) == 0 {
		return
	}
	fmt.Fprintf(w, "\n  %s:\n", title)
	for _, line := range lines {
		fmt.Fprintf(w, "    ~ %s\n", line)
	}
}

// DetectDrift compares the live database behind `live` against the state its
// committed migrations should have produced.
//
// History drift is always computed (it needs only the revision table and the
// migration directory). Structural and advanced-object drift require a clean dev
// database at devConnStr: the migrations recorded as applied are replayed into it
// to reconstruct the expected state, which is then diffed against the live
// database. Pass devConnStr == "" to run history drift only.
func DetectDrift(ctx context.Context, live *Applier, devConnStr, migrationsPath, engine string) (*DriftReport, error) {
	report := &DriftReport{}

	rrw, err := live.dialect.NewRevisions(ctx, live.db)
	if err != nil {
		return nil, err
	}
	revisions, err := rrw.ReadRevisions(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading migration history: %w", err)
	}
	dirVersions, err := migrationVersionSet(migrationsPath)
	if err != nil {
		return nil, err
	}
	report.History = historyDrift(revisions, dirVersions)

	if devConnStr == "" {
		return report, nil
	}

	applied := make(map[string]bool, len(revisions))
	for _, r := range revisions {
		applied[r.Version] = true
	}
	stagedDir, cleanup, err := stageAppliedDir(migrationsPath, applied)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	devApplier, err := NewApplier(engine, devConnStr, live.logger)
	if err != nil {
		return nil, fmt.Errorf("connecting to dev database: %w", err)
	}
	defer devApplier.Close()

	// Replay the applied migrations to reconstruct the expected schema. The
	// dev database is throwaway, so every migration is allowed regardless of risk.
	if err := devApplier.ApplyMigrationDir(ctx, stagedDir, RiskBreaking); err != nil {
		return nil, fmt.Errorf("reconstructing expected schema on the dev database: %w", err)
	}

	expectedSchema, expectedName, err := inspectManagedSchema(ctx, devApplier)
	if err != nil {
		return nil, fmt.Errorf("inspecting expected schema: %w", err)
	}
	actualSchema, _, err := inspectManagedSchema(ctx, live)
	if err != nil {
		return nil, fmt.Errorf("inspecting live database: %w", err)
	}

	drv, err := live.atlasDriver()
	if err != nil {
		return nil, err
	}
	// The expected schema was reconstructed on a differently-named dev database
	// (MySQL scopes a schema to the database name); align the names so the diff
	// reflects only structural differences, not a schema rename.
	expectedSchema.Name = actualSchema.Name
	// The changes that would turn `expected` into `actual` are the drift: they
	// describe how the live database has diverged from what the migrations
	// produced.
	changes, err := drv.SchemaDiff(expectedSchema, actualSchema)
	if err != nil {
		return nil, fmt.Errorf("diffing live database against expected state: %w", err)
	}
	report.Structural = renderStructuralChanges(changes)
	report.StructuralTables = changedTables(changes)

	expectedFP, err := devApplier.dialect.FingerprintAdvancedObjects(ctx, devApplier.db, expectedName)
	if err != nil {
		return nil, fmt.Errorf("fingerprinting expected advanced objects: %w", err)
	}
	liveName, err := live.dialect.SchemaName(ctx, live.db)
	if err != nil {
		return nil, err
	}
	liveFP, err := live.dialect.FingerprintAdvancedObjects(ctx, live.db, liveName)
	if err != nil {
		return nil, fmt.Errorf("fingerprinting live advanced objects: %w", err)
	}
	report.Advanced = diffFingerprints(expectedFP, liveFP)

	return report, nil
}

func inspectManagedSchema(ctx context.Context, a *Applier) (*atlasschema.Schema, string, error) {
	drv, err := a.atlasDriver()
	if err != nil {
		return nil, "", err
	}
	name, err := a.dialect.SchemaName(ctx, a.db)
	if err != nil {
		return nil, "", err
	}
	sch, err := drv.InspectSchema(ctx, name, nil)
	return sch, name, err
}

// Flags revisions applied but no longer present in the directory, and revisions
// applied out of version order. Migration files that are present but not yet
// applied are simply pending rather than drift, so they are not reported.
func historyDrift(revisions []*migrate.Revision, dirVersions map[string]bool) []string {
	var lines []string
	for _, r := range revisions {
		if !dirVersions[r.Version] {
			lines = append(lines, fmt.Sprintf(
				"migration %s (%s) is recorded as applied but is missing from the migration directory",
				r.Version, r.Description))
		}
	}

	byExecuted := append([]*migrate.Revision(nil), revisions...)
	sort.Slice(byExecuted, func(i, j int) bool {
		return byExecuted[i].ExecutedAt.Before(byExecuted[j].ExecutedAt)
	})
	maxVersion := ""
	for _, r := range byExecuted {
		if maxVersion != "" && r.Version < maxVersion {
			lines = append(lines, fmt.Sprintf(
				"migration %s was applied after a later version (out of order)", r.Version))
		}
		if r.Version > maxVersion {
			maxVersion = r.Version
		}
	}
	return lines
}

// The changes are diffed expected → live, so Add* means the object exists in the
// database but not in the migrations, Drop* means it is defined by the
// migrations but missing from the database, and Modify* means it differs.
func renderStructuralChanges(changes []atlasschema.Change) []string {
	var lines []string
	for _, c := range changes {
		lines = append(lines, renderStructuralChange(c)...)
	}
	return lines
}

func renderStructuralChange(change atlasschema.Change) []string {
	switch c := change.(type) {
	case *atlasschema.AddTable:
		if isInternalTable(c.T) {
			return nil
		}
		return []string{fmt.Sprintf("table %s exists in the database but not in the migrations", tableName(c.T))}
	case *atlasschema.DropTable:
		if isInternalTable(c.T) {
			return nil
		}
		return []string{fmt.Sprintf("table %s is defined by the migrations but missing from the database", tableName(c.T))}
	case *atlasschema.ModifyTable:
		if isInternalTable(c.T) {
			return nil
		}
		var lines []string
		for _, sub := range c.Changes {
			lines = append(lines, renderTableChange(tableName(c.T), sub)...)
		}
		return lines
	default:
		return nil
	}
}

func renderTableChange(table string, change atlasschema.Change) []string {
	switch c := change.(type) {
	case *atlasschema.AddColumn:
		return []string{fmt.Sprintf("table %s: column %s exists in the database but not in the migrations", table, columnName(c.C))}
	case *atlasschema.DropColumn:
		return []string{fmt.Sprintf("table %s: column %s is defined by the migrations but missing from the database", table, columnName(c.C))}
	case *atlasschema.ModifyColumn:
		return []string{fmt.Sprintf("table %s: column %s differs from the migrations", table, columnName(c.To))}
	case *atlasschema.AddIndex:
		return []string{fmt.Sprintf("table %s: index %s exists in the database but not in the migrations", table, indexName(c.I))}
	case *atlasschema.DropIndex:
		return []string{fmt.Sprintf("table %s: index %s is defined by the migrations but missing from the database", table, indexName(c.I))}
	case *atlasschema.AddForeignKey:
		return []string{fmt.Sprintf("table %s: a foreign key exists in the database but not in the migrations", table)}
	case *atlasschema.DropForeignKey:
		return []string{fmt.Sprintf("table %s: a foreign key is defined by the migrations but missing from the database", table)}
	case *atlasschema.AddCheck:
		return []string{fmt.Sprintf("table %s: a check constraint exists in the database but not in the migrations", table)}
	case *atlasschema.DropCheck:
		return []string{fmt.Sprintf("table %s: a check constraint is defined by the migrations but missing from the database", table)}
	default:
		return []string{fmt.Sprintf("table %s: definition differs from the migrations", table)}
	}
}

func isInternalTable(t *atlasschema.Table) bool {
	return t != nil && t.Name == revisionsTable
}

func indexName(i *atlasschema.Index) string {
	if i == nil {
		return "?"
	}
	return i.Name
}

func diffFingerprints(expected, live map[string]string) []string {
	var lines []string
	for _, key := range sortedKeys(live) {
		want, ok := expected[key]
		if !ok {
			lines = append(lines, fmt.Sprintf("unexpected %s (created out-of-band)", key))
		} else if want != live[key] {
			lines = append(lines, fmt.Sprintf("%s has been changed out-of-band", key))
		}
	}
	for _, key := range sortedKeys(expected) {
		if _, ok := live[key]; !ok {
			lines = append(lines, fmt.Sprintf("%s is defined by the migrations but missing from the database", key))
		}
	}
	return lines
}

// Reads the objects the structural diff does not cover (views, triggers,
// functions and row-level-security policies) from the catalog and returns a map
// of `kind "name"` to a hash of the object's definition. Objects owned by an
// extension are excluded so they do not appear as drift when the extension is
// present on both sides.
func fingerprintPostgresAdvancedObjects(ctx context.Context, db *sql.DB) (map[string]string, error) {
	fp := map[string]string{}

	if err := scanFingerprints(ctx, db, fp, "view",
		`SELECT table_name, view_definition
		 FROM information_schema.views
		 WHERE table_schema = 'public'`); err != nil {
		return nil, err
	}

	if err := scanFingerprints(ctx, db, fp, "trigger",
		`SELECT event_object_table || '.' || trigger_name AS name,
		        action_timing || ' ' ||
		        string_agg(event_manipulation, ',' ORDER BY event_manipulation) || ' ' ||
		        action_statement AS def
		 FROM information_schema.triggers
		 WHERE trigger_schema = 'public'
		 GROUP BY event_object_table, trigger_name, action_timing, action_statement`); err != nil {
		return nil, err
	}

	if err := scanFingerprints(ctx, db, fp, "function",
		`SELECT p.proname, pg_get_functiondef(p.oid)
		 FROM pg_proc p
		 JOIN pg_namespace n ON n.oid = p.pronamespace
		 WHERE n.nspname = 'public'
		   AND NOT EXISTS (
		     SELECT 1 FROM pg_depend d WHERE d.objid = p.oid AND d.deptype = 'e'
		   )`); err != nil {
		return nil, err
	}

	if err := scanFingerprints(ctx, db, fp, "policy",
		`SELECT tablename || '.' || policyname AS name,
		        cmd || ' ' || COALESCE(qual, '') || ' ' || COALESCE(with_check, '') AS def
		 FROM pg_policies
		 WHERE schemaname = 'public'`); err != nil {
		return nil, err
	}

	return fp, nil
}

func scanFingerprints(ctx context.Context, db *sql.DB, fp map[string]string, kind, query string) error {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("reading %s definitions: %w", kind, err)
	}
	defer rows.Close()

	for rows.Next() {
		var name, def string
		if err := rows.Scan(&name, &def); err != nil {
			return fmt.Errorf("scanning %s definition: %w", kind, err)
		}
		sum := sha256.Sum256([]byte(def))
		fp[fmt.Sprintf("%s %q", kind, name)] = hex.EncodeToString(sum[:])
	}
	return rows.Err()
}

func migrationVersionSet(migrationsPath string) (map[string]bool, error) {
	files, err := migrationFileSet(migrationsPath)
	if err != nil {
		return nil, err
	}
	versions := map[string]bool{}
	for name := range files {
		if v := parseVersion(name); v != "" {
			versions[v] = true
		}
	}
	return versions, nil
}

// Copies the migration files whose versions are recorded as applied into a
// temporary Atlas directory, with a fresh atlas.sum, so the planner and executor
// can replay exactly the applied history to reconstruct the expected state.
func stageAppliedDir(migrationsPath string, applied map[string]bool) (string, func(), error) {
	tmp, err := os.MkdirTemp("", "celerity-drift-applied-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temporary replay directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }

	dir, err := migrate.NewLocalDir(tmp)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("opening temporary replay directory: %w", err)
	}

	files, err := migrationFileSet(migrationsPath)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	for name := range files {
		version := parseVersion(name)
		if version == "" || !applied[version] {
			continue
		}
		content, err := os.ReadFile(filepath.Join(migrationsPath, name))
		if err != nil {
			cleanup()
			return "", nil, fmt.Errorf("reading migration %s: %w", name, err)
		}
		if err := dir.WriteFile(name, content); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("staging migration %s: %w", name, err)
		}
	}
	if err := writeSumFile(dir); err != nil {
		cleanup()
		return "", nil, err
	}
	return tmp, cleanup, nil
}
