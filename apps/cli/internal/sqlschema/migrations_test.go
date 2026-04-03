package sqlschema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_DiscoverMigrationScripts_empty_path(t *testing.T) {
	scripts, err := DiscoverMigrationScripts("")
	require.NoError(t, err)
	assert.Nil(t, scripts)
}

func Test_DiscoverMigrationScripts_missing_dir(t *testing.T) {
	scripts, err := DiscoverMigrationScripts("/nonexistent/path")
	require.NoError(t, err)
	assert.Nil(t, scripts)
}

func Test_DiscoverMigrationScripts_sorted_with_pairs(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"V003__add_trigger.up.sql",
		"V003__add_trigger.down.sql",
		"V001__add_index.up.sql",
		"V001__add_index.down.sql",
		"V002__add_column.up.sql",
		"V002__add_column.down.sql",
	}
	for _, f := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, f), []byte("-- sql"), 0o644))
	}

	scripts, err := DiscoverMigrationScripts(dir)
	require.NoError(t, err)
	require.Len(t, scripts, 3)

	assert.Equal(t, 1, scripts[0].Version)
	assert.Equal(t, "add_index", scripts[0].Description)
	assert.Contains(t, scripts[0].UpPath, "V001__add_index.up.sql")
	assert.Contains(t, scripts[0].DownPath, "V001__add_index.down.sql")

	assert.Equal(t, 2, scripts[1].Version)
	assert.Equal(t, 3, scripts[2].Version)
}

func Test_DiscoverMigrationScripts_up_only(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "V001__init.up.sql"), []byte("-- up"), 0o644))

	scripts, err := DiscoverMigrationScripts(dir)
	require.NoError(t, err)
	require.Len(t, scripts, 1)
	assert.NotEmpty(t, scripts[0].UpPath)
	assert.Empty(t, scripts[0].DownPath)
}

func Test_DiscoverMigrationScripts_skips_non_matching(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"V001__valid.up.sql",
		"V001__valid.down.sql",
		"README.md",
		"notes.sql",         // No V prefix or direction.
		"V002__nodir.sql",   // Missing .up/.down suffix.
		"Vxyz__bad.up.sql",  // Non-numeric version.
	}
	for _, f := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, f), []byte("content"), 0o644))
	}

	scripts, err := DiscoverMigrationScripts(dir)
	require.NoError(t, err)
	require.Len(t, scripts, 1)
	assert.Equal(t, 1, scripts[0].Version)
}

func Test_DiscoverMigrationScripts_skips_directories(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "V001__subdir.up.sql"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "V002__real.up.sql"), []byte("-- sql"), 0o644))

	scripts, err := DiscoverMigrationScripts(dir)
	require.NoError(t, err)
	require.Len(t, scripts, 1)
	assert.Equal(t, 2, scripts[0].Version)
}

func Test_parseMigrationFilename_valid(t *testing.T) {
	tests := []struct {
		name        string
		wantVersion int
		wantDesc    string
		wantDir     string
	}{
		{"V001__add_index.up.sql", 1, "add_index", "up"},
		{"V001__add_index.down.sql", 1, "add_index", "down"},
		{"V123__complex_migration_name.up.sql", 123, "complex_migration_name", "up"},
		{"V0__init.down.sql", 0, "init", "down"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, d, dir, err := parseMigrationFilename(tt.name)
			require.NoError(t, err)
			assert.Equal(t, tt.wantVersion, v)
			assert.Equal(t, tt.wantDesc, d)
			assert.Equal(t, tt.wantDir, dir)
		})
	}
}

func Test_parseMigrationFilename_invalid(t *testing.T) {
	invalid := []string{
		"001__no_prefix.up.sql",
		"V001_single_underscore.up.sql",
		"V001__no_direction.sql",
		"Vabc__not_numeric.up.sql",
		"random.sql",
	}

	for _, name := range invalid {
		t.Run(name, func(t *testing.T) {
			_, _, _, err := parseMigrationFilename(name)
			assert.Error(t, err)
		})
	}
}
