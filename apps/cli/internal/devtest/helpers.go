package devtest

import (
	"os/exec"

	"github.com/newstack-cloud/celerity/apps/cli/internal/devtest/detect"
)

// testCommand is an internal representation of a test command to execute.
type testCommand struct {
	args  []string
	label string
}

// expandSuites deduplicates the requested suites.
func expandSuites(suites []TestSuite) []TestSuite {
	return detect.ExpandSuites(suites)
}

// filterExistingDirs returns only the directories that exist relative to appDir.
func filterExistingDirs(appDir string, dirs []string) []string {
	return detect.FilterExistingDirs(appDir, dirs)
}

// exitCodeFromError extracts the exit code from an exec error.
// Returns 0 if err is nil, 1 if the exit code cannot be determined.
func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return 1
}
