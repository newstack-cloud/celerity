package sqlschema

import (
	"fmt"

	atlasschema "ariga.io/atlas/sql/schema"
)

// excludePatterns returns the patterns that YAML-driven diffs must ignore: the
// schema's own `exclude` list plus the migration-history table, which Celerity
// owns and no schema YAML models.
//
// Patterns are Atlas globs relative to the managed schema, such as "legacy_*"
// for a table or "audit_log.idx_*" for objects within one. An optional type
// selector narrows which kind of object matches, as in
// "audit_log.idx_audit_fts[type=index]". See
// ariga.io/atlas/sql/schema.ExcludeSchema for the full syntax.
func excludePatterns(s *Schema) []string {
	patterns := make([]string, 0, len(s.Exclude)+1)
	patterns = append(patterns, s.Exclude...)
	// Databases migrated before the history table moved to its own schema still
	// carry it in the managed schema; without this the declarative diff would
	// drop it and wipe the migration history.
	return append(patterns, revisionsTable)
}

// A typo in a pattern silently matches nothing, so every pattern is parsed here
// to make it surface from `schema validate` instead.
func validateExcludePatterns(patterns []string) error {
	for _, p := range patterns {
		probe := atlasschema.New(defaultSchemaName)
		atlasschema.NewRealm(probe)
		if _, err := atlasschema.ExcludeSchema(probe, []string{p}); err != nil {
			return fmt.Errorf("invalid exclude pattern %q: %w", p, err)
		}
	}
	return nil
}
