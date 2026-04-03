// Package resolve provides pure path-resolution and config-parsing helpers
// extracted from devconfig so they can be unit-tested without side effects.
package resolve

import (
	"os"
	"path/filepath"
	"strings"
)

// AppDir resolves the application directory from a flag value.
// Empty or "." returns the current working directory; otherwise the
// path is resolved to an absolute path.
func AppDir(appDir string) (string, error) {
	if appDir == "" || appDir == "." {
		return os.Getwd()
	}
	return filepath.Abs(appDir)
}

// BlueprintPath resolves the blueprint file path.
// If flagValue is non-empty, it is resolved relative to appDir.
// Otherwise the function auto-detects from known filenames.
func BlueprintPath(appDir string, flagValue string) (string, error) {
	if flagValue != "" {
		abs := flagValue
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(appDir, abs)
		}
		if _, err := os.Stat(abs); err != nil {
			return "", &NotFoundError{Path: abs, Kind: "blueprint file"}
		}
		return abs, nil
	}

	for _, name := range []string{"app.blueprint.yaml", "app.blueprint.yml", "app.blueprint.jsonc"} {
		path := filepath.Join(appDir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", &NotFoundError{
		Path: appDir,
		Kind: "blueprint file (app.blueprint.yaml or app.blueprint.jsonc)",
	}
}

// ModulePath resolves the module entry point path.
// If flagValue is non-empty it is returned directly.
// Otherwise candidates are tried in order; the first that exists on disk wins.
// If none exist, the first candidate is returned as the default.
func ModulePath(appDir string, flagValue string, candidates []string) string {
	if flagValue != "" {
		return flagValue
	}

	if len(candidates) == 0 {
		return ""
	}

	for _, candidate := range candidates {
		abs := filepath.Join(appDir, candidate)
		if _, err := os.Stat(abs); err == nil {
			return candidate
		}
	}

	return candidates[0]
}

// DirWithTestFallback resolves a directory that may have a test-specific override.
// For "test" mode, it prefers <base>/test/ and falls back to <base>/local/.
// For other modes, it uses <base>/local/.
// Returns empty string if neither directory exists.
func DirWithTestFallback(appDir string, base string, mode string) string {
	if mode == "test" {
		testDir := filepath.Join(appDir, base, "test")
		if _, err := os.Stat(testDir); err == nil {
			return testDir
		}
	}
	localDir := filepath.Join(appDir, base, "local")
	if _, err := os.Stat(localDir); err == nil {
		return localDir
	}
	return ""
}

// DeployTargetToProvider maps a deploy target name to the base cloud provider
// identifier used by the runtime.
func DeployTargetToProvider(deployTarget string) string {
	switch {
	case strings.HasPrefix(deployTarget, "aws"):
		return "aws"
	case strings.HasPrefix(deployTarget, "gcloud"):
		return "gcp"
	case strings.HasPrefix(deployTarget, "azure"):
		return "azure"
	default:
		return deployTarget
	}
}

// NotFoundError is returned when an expected file or directory is not found.
type NotFoundError struct {
	Path string
	Kind string
}

func (e *NotFoundError) Error() string {
	return e.Kind + " not found: " + e.Path
}
