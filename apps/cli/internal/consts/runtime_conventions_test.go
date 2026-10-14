package consts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConventionsForRuntime_nodejs(t *testing.T) {
	conv, ok := ConventionsForRuntime("nodejs24.x")
	assert.True(t, ok)
	assert.Equal(t, []string{"src/app-module.ts", "src/app.module.ts"}, conv.DefaultModulePaths)
	assert.Contains(t, conv.WatchExtensions, ".ts")
	assert.Contains(t, conv.WatchExtensions, ".js")
	assert.Contains(t, conv.WatchSkipDirs, "node_modules")
	assert.True(t, conv.SupportsExtraction)
}

func TestConventionsForRuntime_python(t *testing.T) {
	conv, ok := ConventionsForRuntime("python3.13")
	assert.True(t, ok)
	assert.Equal(t, []string{"src/app_module.py"}, conv.DefaultModulePaths)
	assert.Contains(t, conv.WatchExtensions, ".py")
	assert.Contains(t, conv.WatchSkipDirs, "__pycache__")
	assert.Contains(t, conv.WatchSkipDirs, ".venv")
	assert.True(t, conv.SupportsExtraction)
}

func TestConventionsForRuntime_osOnly(t *testing.T) {
	conv, ok := ConventionsForRuntime(RuntimeOSOnlyDebian13)
	assert.True(t, ok)
	assert.Equal(t, []string{"."}, conv.WatchRoots)
	assert.Contains(t, conv.WatchSkipDirs, ".celerity")
	assert.Contains(t, conv.WatchSkipDirs, "vendor")
	assert.True(t, conv.SupportsExtraction)
	// The language that built the handlers executable is not named by the
	// runtime identifier, so these come from the toolchain instead.
	assert.Empty(t, conv.WatchExtensions)
	assert.Empty(t, conv.DefaultModulePaths)
}

// Go handlers declare an OS-only runtime, so there is no Go runtime identifier
// for a "go"-keyed entry to match against.
func TestConventionsForRuntime_goIsNotARuntime(t *testing.T) {
	_, ok := ConventionsForRuntime("go1.22")
	assert.False(t, ok)
}

func TestConventionsForRuntime_nodeAndPythonWatchSrc(t *testing.T) {
	for _, runtime := range []string{"nodejs24.x", "python3.13"} {
		conv, ok := ConventionsForRuntime(runtime)
		assert.True(t, ok, runtime)
		assert.Equal(t, []string{"src"}, conv.WatchRoots, runtime)
	}
}

// Map iteration is unordered, so which of two overlapping keys wins must not
// come down to luck. Run it enough times that a first-match implementation
// would be overwhelmingly likely to return the shorter key at least once.
func TestConventionsForRuntime_longestPrefixWins(t *testing.T) {
	m := map[string]int{"os": 1, "os-only": 2, "os-only.debian13": 3}
	for range 100 {
		got, ok := longestPrefixMatch(m, "os-only.debian13")
		assert.True(t, ok)
		assert.Equal(t, 3, got)
	}
}

func TestConventionsForRuntime_unknown(t *testing.T) {
	_, ok := ConventionsForRuntime("ruby3.3")
	assert.False(t, ok)
}

func TestTestConventionsForRuntime_nodejs(t *testing.T) {
	conv, ok := TestConventionsForRuntime("nodejs22.x")
	assert.True(t, ok)
	assert.Contains(t, conv.DefaultTestDirs, "unit")
	assert.Contains(t, conv.DefaultTestDirs, "integration")
	assert.Contains(t, conv.DefaultTestDirs, "api")
	assert.Contains(t, conv.DetectFiles, "package.json")
}

func TestTestConventionsForRuntime_python(t *testing.T) {
	conv, ok := TestConventionsForRuntime("python3.12.x")
	assert.True(t, ok)
	assert.Contains(t, conv.DefaultTestDirs, "unit")
	assert.Contains(t, conv.DefaultTestDirs, "integration")
	assert.Contains(t, conv.DefaultTestDirs, "api")
	assert.Contains(t, conv.DetectFiles, "pyproject.toml")
}

func TestTestConventionsForRuntime_unknown(t *testing.T) {
	_, ok := TestConventionsForRuntime("ruby3.3")
	assert.False(t, ok)
}

// Test directories belong to the toolchain for an OS-only runtime, not to the
// runtime, which serves several languages at once.
func TestTestConventionsForRuntime_osOnly_notSupported(t *testing.T) {
	_, ok := TestConventionsForRuntime(RuntimeOSOnlyDebian13)
	assert.False(t, ok)
}
