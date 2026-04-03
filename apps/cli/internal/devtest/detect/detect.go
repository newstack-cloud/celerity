// Package detect provides pure detection and command-building logic for test
// runners, extracted so it can be unit-tested without executing shell commands.
package detect

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
)

// PackageJSON holds the fields we need from a Node.js package.json.
type PackageJSON struct {
	Type            string            `json:"type"`
	PackageManager  string            `json:"packageManager"`
	Scripts         map[string]string `json:"scripts"`
	DevDependencies map[string]string `json:"devDependencies"`
	Dependencies    map[string]string `json:"dependencies"`
}

// IsESM returns true if the package uses ES modules ("type": "module").
func (p *PackageJSON) IsESM() bool {
	return p.Type == "module"
}

// LoadPackageJSON reads and parses the package.json in appDir.
func LoadPackageJSON(appDir string) (*PackageJSON, error) {
	data, err := os.ReadFile(filepath.Join(appDir, "package.json"))
	if err != nil {
		return nil, err
	}
	var pkg PackageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}

// DetectPackageManager determines the Node.js package manager from the
// packageManager field or lock files on disk.
func DetectPackageManager(pkg *PackageJSON, appDir string) string {
	if pkg.PackageManager != "" {
		pm := strings.Split(pkg.PackageManager, "@")[0]
		if pm == "npm" || pm == "yarn" || pm == "pnpm" {
			return pm
		}
	}

	for _, candidate := range []struct {
		file string
		pm   string
	}{
		{"yarn.lock", "yarn"},
		{"pnpm-lock.yaml", "pnpm"},
		{"package-lock.json", "npm"},
	} {
		if _, err := os.Stat(filepath.Join(appDir, candidate.file)); err == nil {
			return candidate.pm
		}
	}

	return "npm"
}

// DetectFramework detects the test framework from package.json dependencies.
func DetectFramework(pkg *PackageJSON) string {
	allDeps := make(map[string]string)
	maps.Copy(allDeps, pkg.Dependencies)
	maps.Copy(allDeps, pkg.DevDependencies)

	for _, fw := range []string{"vitest", "jest", "ava"} {
		if _, ok := allDeps[fw]; ok {
			return fw
		}
	}
	return ""
}

// FrameworkCommand builds the test runner command for a given framework.
func FrameworkCommand(pm string, framework string, dirs []string, coverage bool) []string {
	exec := pmExecPrefix(pm)
	switch framework {
	case "vitest":
		args := append(exec, "vitest", "run")
		args = append(args, dirs...)
		if coverage {
			args = append(args, "--coverage")
		}
		return args
	case "jest":
		args := append(exec, "jest")
		for _, d := range dirs {
			args = append(args, "--roots", d)
		}
		if coverage {
			args = append(args, "--coverage")
		}
		return args
	case "ava":
		var patterns []string
		for _, d := range dirs {
			patterns = append(patterns, d+"/**/*.test.{ts,js}")
		}
		args := append(exec, "ava")
		args = append(args, patterns...)
		return args
	default:
		return []string{pm, "test"}
	}
}

func pmExecPrefix(pm string) []string {
	switch pm {
	case "yarn":
		return []string{"yarn"}
	case "pnpm":
		return []string{"pnpm", "exec"}
	default:
		return []string{"npm", "exec"}
	}
}

// FindPytest looks for pytest in common virtualenv locations, then falls back
// to checking if "pytest" is on the system PATH (returns empty string if not found).
func FindPytest(appDir string) string {
	for _, venv := range []string{".venv", "venv"} {
		candidate := filepath.Join(appDir, venv, "bin", "pytest")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// PytestArgs builds the pytest command-line arguments.
func PytestArgs(pytestBin string, dirs []string, coverage bool) []string {
	args := []string{pytestBin}
	args = append(args, dirs...)
	if coverage {
		args = append(args, "--cov", "--cov-report=term-missing")
	}
	args = append(args, "-v")
	return args
}

// FilterExistingDirs returns only the directories that exist relative to appDir.
func FilterExistingDirs(appDir string, dirs []string) []string {
	var existing []string
	for _, d := range dirs {
		abs := filepath.Join(appDir, d)
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			existing = append(existing, d)
		}
	}
	return existing
}

// ExpandSuites deduplicates a list of suite names.
func ExpandSuites[T comparable](suites []T) []T {
	seen := make(map[T]bool, len(suites))
	var result []T
	for _, s := range suites {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
