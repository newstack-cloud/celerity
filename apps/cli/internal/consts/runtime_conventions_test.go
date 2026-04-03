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

func TestConventionsForRuntime_go(t *testing.T) {
	conv, ok := ConventionsForRuntime("go1.22")
	assert.True(t, ok)
	assert.Equal(t, []string{"main.go"}, conv.DefaultModulePaths)
	assert.Contains(t, conv.WatchExtensions, ".go")
	assert.Contains(t, conv.WatchSkipDirs, "vendor")
	assert.False(t, conv.SupportsExtraction)
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

func TestTestConventionsForRuntime_go_not_supported(t *testing.T) {
	_, ok := TestConventionsForRuntime("go1.22")
	assert.False(t, ok)
}
