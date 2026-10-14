package sqlschema

import (
	"fmt"

	"ariga.io/atlas/sql/migrate"
)

// ValidateSchema parses a schema YAML file and checks that it maps cleanly to
// the target engine: the file must parse, foreign keys must reference existing
// tables and columns, and column types must be understood. It does not require
// a database connection.
func ValidateSchema(schemaPath, engine string) error {
	parsed, err := ParseSchemaFile(schemaPath)
	if err != nil {
		return err
	}
	if _, err := desiredSchema(parsed, engine); err != nil {
		return err
	}
	return validateExcludePatterns(parsed.Exclude)
}

// ValidateMigrationDir checks that the atlas.sum integrity file is present and
// matches the migration directory contents. Returns nil when the directory
// holds no migrations yet.
func ValidateMigrationDir(migrationsPath string) error {
	has, err := hasMigrationFiles(migrationsPath)
	if err != nil {
		return err
	}
	if !has {
		return nil
	}
	if err := ValidateMigrationFileNames(migrationsPath); err != nil {
		return err
	}

	dir, err := migrate.NewLocalDir(migrationsPath)
	if err != nil {
		return fmt.Errorf("opening migrations directory: %w", err)
	}
	if err := migrate.Validate(dir); err != nil {
		return fmt.Errorf(
			"migration directory integrity check failed (%w). If you edited a migration on purpose "+
				"(a hand-written file from `celerity schema new`), run `celerity schema hash` to "+
				"re-sum the directory. Otherwise the files no longer match the history that ran",
			err,
		)
	}
	return nil
}
