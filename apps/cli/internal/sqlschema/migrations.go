package sqlschema

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// DiscoverMigrationScripts finds versioned SQL migration pairs
// (V<number>__<description>.up.sql / V<number>__<description>.down.sql)
// in the given directory, sorted by version number.
func DiscoverMigrationScripts(migrationsPath string) ([]MigrationScript, error) {
	if migrationsPath == "" {
		return nil, nil
	}

	if _, err := os.Stat(migrationsPath); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return nil, fmt.Errorf("reading migrations directory %s: %w", migrationsPath, err)
	}

	// Group files by version+description key.
	type migrationKey struct {
		version     int
		description string
	}
	scripts := map[migrationKey]*MigrationScript{}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}

		version, description, direction, err := parseMigrationFilename(entry.Name())
		if err != nil {
			continue // Skip files that don't match the naming convention.
		}

		key := migrationKey{version: version, description: description}
		ms, ok := scripts[key]
		if !ok {
			ms = &MigrationScript{Version: version, Description: description}
			scripts[key] = ms
		}

		fullPath := filepath.Join(migrationsPath, entry.Name())
		switch direction {
		case "up":
			ms.UpPath = fullPath
		case "down":
			ms.DownPath = fullPath
		}
	}

	// Flatten to sorted slice.
	result := make([]MigrationScript, 0, len(scripts))
	for _, ms := range scripts {
		result = append(result, *ms)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})

	return result, nil
}

// parseMigrationFilename parses a filename like "V001__add_search_index.up.sql"
// into version number, description, and direction (up/down).
func parseMigrationFilename(name string) (version int, description string, direction string, err error) {
	// Strip .sql extension.
	base := strings.TrimSuffix(name, ".sql")

	// Extract direction suffix (.up or .down).
	if strings.HasSuffix(base, ".up") {
		direction = "up"
		base = strings.TrimSuffix(base, ".up")
	} else if strings.HasSuffix(base, ".down") {
		direction = "down"
		base = strings.TrimSuffix(base, ".down")
	} else {
		return 0, "", "", fmt.Errorf("filename %q missing .up or .down suffix", name)
	}

	if !strings.HasPrefix(base, "V") {
		return 0, "", "", fmt.Errorf("filename %q does not start with V", name)
	}

	parts := strings.SplitN(base[1:], "__", 2)
	if len(parts) != 2 {
		return 0, "", "", fmt.Errorf("filename %q does not match V<number>__<description>.<up|down>.sql", name)
	}

	version, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", "", fmt.Errorf("invalid version number in %q: %w", name, err)
	}

	return version, parts[1], direction, nil
}
