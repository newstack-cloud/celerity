package consts

import "strings"

// RuntimeConventions defines language-specific defaults for dev commands.
// Each supported runtime prefix (e.g. "nodejs", "python") maps to one of these.
type RuntimeConventions struct {
	// DefaultModulePaths are tried in order to find the module entry point.
	// First match on disk wins. If none exist, the first path is used as the default.
	DefaultModulePaths []string

	// WatchRoots are the directories, relative to the application directory,
	// that the watcher walks. Node.js and Python projects keep their handlers
	// under src/; a Go module has them at the module root, so this is not a
	// constant the watcher can assume.
	WatchRoots []string

	// WatchExtensions are the file extensions the watcher considers relevant.
	// Empty for an OS-only runtime, where the extensions belong to whichever
	// language built the handlers executable and come from the toolchain.
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
		WatchRoots:         []string{"src"},
		WatchExtensions:    []string{".ts", ".js", ".mjs"},
		WatchSkipDirs:      []string{"node_modules"},
		SupportsExtraction: true,
	},
	LanguagePython: {
		DefaultModulePaths: []string{"src/app_module.py"},
		WatchRoots:         []string{"src"},
		WatchExtensions:    []string{".py"},
		WatchSkipDirs:      []string{"__pycache__", ".venv", "venv", ".mypy_cache"},
		SupportsExtraction: true,
	},
	// Keyed by the OS-only prefix rather than by a language, because Go
	// handlers declare os-only.debian13 and no runtime identifier starts with
	// "go".
	//
	// WatchExtensions is empty and DefaultModulePaths unset because both are
	// properties of the language that built the handlers executable, which the
	// runtime identifier does not name. The resolved toolchain supplies them.
	RuntimeOSOnly: {
		WatchRoots:         []string{"."},
		WatchSkipDirs:      []string{".celerity", "vendor", "target", "node_modules"},
		SupportsExtraction: true,
	},
}

// ConventionsForRuntime returns the RuntimeConventions for the given runtime string.
// The runtime string (e.g. "nodejs24.x", "python3.13", "os-only.debian13") is
// matched by prefix against the known runtime keys.
func ConventionsForRuntime(runtime string) (RuntimeConventions, bool) {
	return longestPrefixMatch(runtimeConventions, runtime)
}

// The longest prefix wins rather than the first match, because Go map
// iteration is unordered and a first-match loop over two keys that both prefix
// a runtime identifier picks a different one from run to run. Nothing
// registered today overlaps, which is exactly why the bug would be found long
// after the key that introduced it was added.
func longestPrefixMatch[T any](m map[string]T, runtime string) (T, bool) {
	var (
		best    T
		found   bool
		longest int
	)
	for prefix, value := range m {
		if !strings.HasPrefix(runtime, prefix) {
			continue
		}
		if !found || len(prefix) > longest {
			best, found, longest = value, true, len(prefix)
		}
	}
	return best, found
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
// Matching works by longest prefix, the same as ConventionsForRuntime.
//
// There is deliberately no OS-only entry, because one OS-only runtime serves
// Go, Rust, Zig and C++, whose test layouts have nothing in common. The
// directories belong to the resolved toolchain rather than to the runtime.
func TestConventionsForRuntime(runtime string) (TestConventions, bool) {
	return longestPrefixMatch(testConventions, runtime)
}
