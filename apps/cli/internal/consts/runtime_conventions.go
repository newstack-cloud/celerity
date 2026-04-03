package consts

import "strings"

// RuntimeConventions defines language-specific defaults for dev commands.
// Each supported runtime prefix (e.g. "nodejs", "python") maps to one of these.
type RuntimeConventions struct {
	// DefaultModulePaths are tried in order to find the module entry point.
	// First match on disk wins. If none exist, the first path is used as the default.
	DefaultModulePaths []string

	// WatchExtensions are the file extensions the watcher considers relevant.
	WatchExtensions []string

	// WatchSkipDirs are directory names the watcher should skip.
	// ".git" is always skipped and does not need to be listed here.
	WatchSkipDirs []string

	// SupportsExtraction indicates whether this runtime has a handler extraction CLI.
	// When false, the extraction step is skipped and the raw blueprint is used as-is.
	SupportsExtraction bool
}

var runtimeConventions = map[string]RuntimeConventions{
	LanguageNodeJS: {
		DefaultModulePaths: []string{"src/app-module.ts", "src/app.module.ts"},
		WatchExtensions:    []string{".ts", ".js", ".mjs"},
		WatchSkipDirs:      []string{"node_modules"},
		SupportsExtraction: true,
	},
	LanguagePython: {
		DefaultModulePaths: []string{"src/app_module.py"},
		WatchExtensions:    []string{".py"},
		WatchSkipDirs:      []string{"__pycache__", ".venv", "venv", ".mypy_cache"},
		SupportsExtraction: true,
	},
	LanguageGo: {
		DefaultModulePaths: []string{"main.go"},
		WatchExtensions:    []string{".go"},
		WatchSkipDirs:      []string{"vendor"},
		SupportsExtraction: false,
	},
}

// ConventionsForRuntime returns the RuntimeConventions for the given runtime string.
// The runtime string (e.g. "nodejs24.x", "python3.13") is matched by prefix
// against known language keys.
func ConventionsForRuntime(runtime string) (RuntimeConventions, bool) {
	for prefix, conv := range runtimeConventions {
		if strings.HasPrefix(runtime, prefix) {
			return conv, true
		}
	}
	return RuntimeConventions{}, false
}

// TestConventions defines language-specific defaults for test execution.
type TestConventions struct {
	// DefaultTestDirs maps suite names to directories containing tests.
	// The test runner uses these to scope which tests to run for each suite.
	DefaultTestDirs map[string][]string

	// DetectFiles are filenames checked to auto-detect the test framework
	// (e.g. "package.json" for Node.js, "pyproject.toml" for Python).
	DetectFiles []string
}

var testConventions = map[string]TestConventions{
	LanguageNodeJS: {
		DefaultTestDirs: map[string][]string{
			"unit":        {"src"},
			"integration": {"tests/integration"},
			"api":         {"tests/api"},
		},
		DetectFiles: []string{"package.json"},
	},
	LanguagePython: {
		DefaultTestDirs: map[string][]string{
			"unit":        {"tests/unit"},
			"integration": {"tests/integration"},
			"api":         {"tests/api"},
		},
		DetectFiles: []string{"pyproject.toml", "pytest.ini", "setup.cfg"},
	},
}

// TestConventionsForRuntime returns the TestConventions for the given runtime string.
// Matching works the same as ConventionsForRuntime — by prefix.
func TestConventionsForRuntime(runtime string) (TestConventions, bool) {
	for prefix, conv := range testConventions {
		if strings.HasPrefix(runtime, prefix) {
			return conv, true
		}
	}
	return TestConventions{}, false
}
