package sqlschema

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"ariga.io/atlas/sql/migrate"
	atlasschema "ariga.io/atlas/sql/schema"
)

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

// ScaffoldMigration creates an empty, timestamped migration file for
// hand-written SQL and updates the atlas.sum integrity file. It returns the path
// of the created file.
func ScaffoldMigration(migrationsPath, description string) (string, error) {
	dir, err := openOrCreateDir(migrationsPath)
	if err != nil {
		return "", err
	}

	version, err := nextAvailableVersion(migrationsPath, time.Now())
	if err != nil {
		return "", err
	}
	name := migrationFileName(version, description)
	body := fmt.Sprintf(
		"%s-- %s\n-- Hand-written migration. Add your SQL statements below.\n"+
			"-- Downgrade the risk above to `safe` only if this migration is additive/backward-compatible.\n",
		riskHeader(RiskRisky, "hand-written"), description)
	if err := dir.WriteFile(name, []byte(body)); err != nil {
		return "", fmt.Errorf("writing migration file: %w", err)
	}
	if err := writeSumFile(dir); err != nil {
		return "", err
	}
	return filepath.Join(migrationsPath, name), nil
}

// GenerateMigration computes the difference between the schema YAML (desired
// state) and the current state, which it obtains by replaying the existing
// migration directory on the dev database at devConnStr. The difference is
// written as a new timestamped migration file and atlas.sum is updated. It
// returns the path of the created file, or "" if there is no difference.
//
// The dev database must be empty; the planner replays the migration directory
// into it to compute the current state.
func GenerateMigration(ctx context.Context, devConnStr, schemaPath, migrationsPath, engine, name string) (string, error) {
	parsed, err := ParseSchemaFile(schemaPath)
	if err != nil {
		return "", fmt.Errorf("parsing schema: %w", err)
	}
	desired, err := desiredSchema(parsed, engine)
	if err != nil {
		return "", fmt.Errorf("building desired schema: %w", err)
	}

	firstMigration, err := isEmptyMigrationDir(migrationsPath)
	if err != nil {
		return "", err
	}

	dir, err := openOrCreateDir(migrationsPath)
	if err != nil {
		return "", err
	}

	drv, devSchema, closeDev, err := openDevDriver(ctx, engine, devConnStr)
	if err != nil {
		return "", err
	}
	defer closeDev()
	desired.Name = devSchema

	planName := slugify(name)
	if planName == "" {
		planName = "changes"
	}

	before, err := migrationFileSet(migrationsPath)
	if err != nil {
		return "", err
	}

	planner := migrate.NewPlanner(drv, dir,
		migrate.PlanWithSchemaQualifier(""),
		// Objects the YAML does not model are filtered out of the replayed
		// state, so a hand-written index or trigger is never planned for
		// removal just because the schema YAML has no way to describe it.
		migrate.PlanWithExclude(excludePatterns(parsed)...),
	)
	plan, err := planner.PlanSchema(ctx, planName, migrate.Schema(desired))
	if errors.Is(err, migrate.ErrNoPlan) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("planning migration: %w", err)
	}

	// Emit CREATE EXTENSION for declared extensions on the initial migration.
	// Guarded with IF NOT EXISTS so a hand-edited history stays safe.
	if firstMigration {
		var extChanges []*migrate.Change
		for _, ext := range parsed.Extensions {
			extChanges = append(extChanges, &migrate.Change{
				Cmd:     fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS %q", ext),
				Comment: fmt.Sprintf("enable extension %q", ext),
			})
		}
		plan.Changes = append(extChanges, plan.Changes...)
	}

	// Classify the change set so the migration carries a risk header for the
	// apply-time safety gate. Prepended extension changes have a nil Source.
	var sources []atlasschema.Change
	for _, change := range plan.Changes {
		if change.Source != nil {
			sources = append(sources, change.Source)
		}
	}
	riskLevel, riskReason := classifyChanges(sources)

	if err := planner.WritePlan(plan); err != nil {
		return "", fmt.Errorf("writing migration: %w", err)
	}

	// WritePlan names the file itself (version generated at write time), so
	// locate the newly written migration by diffing the directory listing.
	newFile := ""
	after, err := migrationFileSet(migrationsPath)
	if err != nil {
		return "", err
	}
	for name := range after {
		if !before[name] {
			newFile = name
			break
		}
	}
	if newFile == "" {
		return "", fmt.Errorf("migration planned but no new file was written")
	}

	// Atlas versions to the second; if the new migration collides with an
	// existing one (e.g. `schema new` then `schema diff` in the same second),
	// rename it to a later version so the history stays ordered and unique.
	newFile, err = ensureUniqueVersion(migrationsPath, newFile, before)
	if err != nil {
		return "", err
	}

	newPath := filepath.Join(migrationsPath, newFile)
	if err := prependLine(newPath, riskHeader(riskLevel, riskReason)); err != nil {
		return "", err
	}
	if err := writeSumFile(dir); err != nil {
		return "", err
	}
	return newPath, nil
}

func prependLine(path, line string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading migration file: %w", err)
	}
	if err := os.WriteFile(path, append([]byte(line), content...), 0o644); err != nil {
		return fmt.Errorf("writing migration file: %w", err)
	}
	return nil
}

// ReadMigrationRisk returns the risk classification recorded in a migration
// file's header (falling back to risky when absent).
func ReadMigrationRisk(path string) (RiskLevel, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading migration file: %w", err)
	}
	return migrationRisk(content), nil
}

func ensureUniqueVersion(migrationsPath, fileName string, existing map[string]bool) (string, error) {
	version := parseVersion(fileName)
	prevMax := maxVersion(existing)
	if version == "" || version > prevMax {
		return fileName, nil
	}

	newName := bumpVersion(prevMax) + fileName[len(version):]
	if err := os.Rename(
		filepath.Join(migrationsPath, fileName),
		filepath.Join(migrationsPath, newName),
	); err != nil {
		return "", fmt.Errorf("renaming migration to a unique version: %w", err)
	}
	return newName, nil
}

func migrationFileSet(migrationsPath string) (map[string]bool, error) {
	set := map[string]bool{}
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return set, nil
		}
		return nil, fmt.Errorf("reading migrations directory %s: %w", migrationsPath, err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			set[e.Name()] = true
		}
	}
	return set, nil
}

// CheckPendingMigration reports whether the schema YAML has changes that are not
// yet captured by a committed migration, which is to say whether
// `celerity schema diff` would generate a new migration. It computes the diff
// against the dev database
// (replaying the existing migration directory) but writes nothing. Used by
// `schema diff --check` as a CI completeness gate.
func CheckPendingMigration(ctx context.Context, devConnStr, schemaPath, migrationsPath, engine string) (bool, error) {
	parsed, err := ParseSchemaFile(schemaPath)
	if err != nil {
		return false, fmt.Errorf("parsing schema: %w", err)
	}
	desired, err := desiredSchema(parsed, engine)
	if err != nil {
		return false, fmt.Errorf("building desired schema: %w", err)
	}

	path, cleanup, err := replayDir(migrationsPath)
	if err != nil {
		return false, err
	}
	defer cleanup()
	dir, err := migrate.NewLocalDir(path)
	if err != nil {
		return false, fmt.Errorf("opening migrations directory: %w", err)
	}

	drv, devSchema, closeDev, err := openDevDriver(ctx, engine, devConnStr)
	if err != nil {
		return false, err
	}
	defer closeDev()
	desired.Name = devSchema

	plan, err := migrate.NewPlanner(drv, dir,
		migrate.PlanWithSchemaQualifier(""),
		migrate.PlanWithExclude(excludePatterns(parsed)...),
	).PlanSchema(ctx, "check", migrate.Schema(desired))
	if errors.Is(err, migrate.ErrNoPlan) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking for pending schema changes: %w", err)
	}
	return len(plan.Changes) > 0, nil
}

// ChangedTables returns the tables the pending schema change would touch, those
// added, dropped or modified by the migration `schema diff` would generate. It
// computes the diff against the dev database (replaying the existing migration
// directory) but writes nothing. Used to evaluate schema contracts.
func ChangedTables(ctx context.Context, devConnStr, schemaPath, migrationsPath, engine string) ([]string, error) {
	parsed, err := ParseSchemaFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("parsing schema: %w", err)
	}
	desired, err := desiredSchema(parsed, engine)
	if err != nil {
		return nil, fmt.Errorf("building desired schema: %w", err)
	}

	path, cleanup, err := replayDir(migrationsPath)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	dir, err := migrate.NewLocalDir(path)
	if err != nil {
		return nil, fmt.Errorf("opening migrations directory: %w", err)
	}

	drv, devSchema, closeDev, err := openDevDriver(ctx, engine, devConnStr)
	if err != nil {
		return nil, err
	}
	defer closeDev()
	desired.Name = devSchema

	plan, err := migrate.NewPlanner(drv, dir, migrate.PlanWithSchemaQualifier("")).PlanSchema(ctx, "contracts", migrate.Schema(desired))
	if errors.Is(err, migrate.ErrNoPlan) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("computing schema changes: %w", err)
	}

	var sources []atlasschema.Change
	for _, change := range plan.Changes {
		if change.Source != nil {
			sources = append(sources, change.Source)
		}
	}
	return changedTables(sources), nil
}

// Returns the name of the schema Celerity manages within the dev database
// alongside the driver, since the caller has to align the desired schema with it.
func openDevDriver(ctx context.Context, engine, devConnStr string) (migrate.Driver, string, func(), error) {
	d, err := dialectForEngine(engine)
	if err != nil {
		return nil, "", nil, err
	}
	connStr, err := d.NormalizeConnString(devConnStr)
	if err != nil {
		return nil, "", nil, err
	}
	db, err := sql.Open(d.DriverName(), connStr)
	if err != nil {
		return nil, "", nil, fmt.Errorf("connecting to dev database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, "", nil, fmt.Errorf("pinging dev database: %w", err)
	}
	drv, err := d.OpenAtlas(db)
	if err != nil {
		db.Close()
		return nil, "", nil, err
	}
	schemaName, err := d.SchemaName(ctx, db)
	if err != nil {
		db.Close()
		return nil, "", nil, err
	}
	return drv, schemaName, func() { db.Close() }, nil
}

// Returns the real migrations directory when it exists, and a temporary empty
// one with a cleanup when it does not, so a check can run before any migration
// exists.
func replayDir(migrationsPath string) (string, func(), error) {
	if _, err := os.Stat(migrationsPath); err == nil {
		return migrationsPath, func() {}, nil
	} else if !os.IsNotExist(err) {
		return "", nil, err
	}
	tmp, err := os.MkdirTemp("", "celerity-schema-check-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temporary replay directory: %w", err)
	}
	return tmp, func() { _ = os.RemoveAll(tmp) }, nil
}

// RehashMigrationDir recomputes atlas.sum from the files currently in the
// migration directory. It is the deliberate "I edited a migration on purpose"
// step. Everything else in the CLI treats a checksum mismatch as a hard error,
// because on an already-applied migration it means the history no longer matches
// what ran.
func RehashMigrationDir(migrationsPath string) error {
	if err := ValidateMigrationFileNames(migrationsPath); err != nil {
		return err
	}
	dir, err := migrate.NewLocalDir(migrationsPath)
	if err != nil {
		return fmt.Errorf("opening migrations directory: %w", err)
	}
	return writeSumFile(dir)
}

// ValidateMigrationFileNames checks that every .sql file in the directory
// follows the <timestamp>_<description>.sql convention and that no two files
// share a version.
//
// Atlas derives a migration's identity from the leading segment of its file
// name. Files that do not follow the convention, such as a Flyway-style
// "V001__name.up.sql" pair, collide on version, and the collision surfaces deep
// inside the executor as a panic rather than an error, so this runs before
// anything is executed.
func ValidateMigrationFileNames(migrationsPath string) error {
	files, err := migrationFileSet(migrationsPath)
	if err != nil {
		return err
	}

	versions := map[string]string{}
	for _, name := range sortedKeys(files) {
		version := parseVersion(name)
		if version == "" {
			return fmt.Errorf(
				"migration file %q does not follow the <timestamp>_<description>.sql naming convention "+
					"(e.g. 20260713102400_add_customer_tier.sql); create migrations with "+
					"`celerity schema diff` or `celerity schema new` so they are named and ordered correctly",
				name,
			)
		}
		if other, ok := versions[version]; ok {
			return fmt.Errorf(
				"migration files %q and %q share the version %q; each migration needs its own version "+
					"(migrations are forward-only, so there are no paired up/down files)",
				other, name, version,
			)
		}
		versions[version] = name
	}
	return nil
}

func openOrCreateDir(migrationsPath string) (*migrate.LocalDir, error) {
	if err := os.MkdirAll(migrationsPath, 0o755); err != nil {
		return nil, fmt.Errorf("creating migrations directory: %w", err)
	}
	dir, err := migrate.NewLocalDir(migrationsPath)
	if err != nil {
		return nil, fmt.Errorf("opening migrations directory: %w", err)
	}
	return dir, nil
}

func isEmptyMigrationDir(migrationsPath string) (bool, error) {
	has, err := hasMigrationFiles(migrationsPath)
	return !has, err
}

func writeSumFile(dir *migrate.LocalDir) error {
	sum, err := dir.Checksum()
	if err != nil {
		return fmt.Errorf("computing directory checksum: %w", err)
	}
	if err := migrate.WriteSumFile(dir, sum); err != nil {
		return fmt.Errorf("writing %s: %w", sumFileName, err)
	}
	return nil
}

func migrationFileName(version, description string) string {
	slug := slugify(description)
	if slug == "" {
		slug = "migration"
	}
	return fmt.Sprintf("%s_%s.sql", version, slug)
}

const versionLayout = "20060102150405"

// Returns a timestamp version (to the second) strictly greater than every
// existing migration version in the directory, so migrations created within the
// same second do not collide. Atlas keys migrations by version, so uniqueness is
// required for correctness.
func nextAvailableVersion(migrationsPath string, base time.Time) (string, error) {
	files, err := migrationFileSet(migrationsPath)
	if err != nil {
		return "", err
	}
	baseVersion := base.UTC().Format(versionLayout)
	max := maxVersion(files)
	if max == "" || baseVersion > max {
		return baseVersion, nil
	}
	return bumpVersion(max), nil
}

func maxVersion(files map[string]bool) string {
	max := ""
	for name := range files {
		// Versions are fixed-width timestamps, so lexical comparison is ordinal.
		if v := parseVersion(name); v > max {
			max = v
		}
	}
	return max
}

func parseVersion(fileName string) string {
	version, _, found := strings.Cut(fileName, "_")
	if !found || len(version) != len(versionLayout) {
		return ""
	}
	if _, err := time.Parse(versionLayout, version); err != nil {
		return ""
	}
	return version
}

func bumpVersion(version string) string {
	t, err := time.Parse(versionLayout, version)
	if err != nil {
		return version
	}
	return t.Add(time.Second).Format(versionLayout)
}

func slugify(s string) string {
	return strings.Trim(nonSlugChars.ReplaceAllString(strings.ToLower(s), "_"), "_")
}
