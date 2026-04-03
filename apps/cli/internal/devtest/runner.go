package devtest

import "context"

// TestSuite identifies which test suites to run.
type TestSuite string

const (
	SuiteUnit        TestSuite = "unit"
	SuiteIntegration TestSuite = "integration"
	SuiteAPI         TestSuite = "api"
)

// ValidSuites are the accepted values for the --suite flag.
// Multiple suites can be comma-separated: --suite unit,integration
var ValidSuites = []TestSuite{SuiteUnit, SuiteIntegration, SuiteAPI}

// RunConfig holds configuration for a test run.
type RunConfig struct {
	// AppDir is the absolute path to the application root directory.
	AppDir string

	// Runtime is the detected runtime string (e.g. "nodejs22.x", "python3.12.x").
	Runtime string

	// Suites lists which test suites to execute.
	Suites []TestSuite

	// Coverage enables coverage reporting when true.
	Coverage bool

	// TestCommand is a user-provided override for the test command.
	// When non-empty, auto-detection is skipped and this command is
	// executed directly via the shell.
	TestCommand string

	// HostPort is the host port the app container is listening on.
	// Used to construct CELERITY_TEST_BASE_URL for API tests.
	HostPort string

	// Verbose enables debug-level output from the test runner.
	Verbose bool

	// Env holds additional environment variables to pass to the test process.
	// Typically contains infrastructure endpoints (databases, caches, etc.)
	// rewritten for host access.
	Env map[string]string
}

// RunResult holds the outcome of a test run.
type RunResult struct {
	// ExitCode is the exit code from the test runner process.
	// 0 means all tests passed.
	ExitCode int

	// CoveragePath is the path to the coverage report, if generated.
	CoveragePath string
}

// TestRunner executes tests for a specific runtime.
type TestRunner interface {
	// Run executes the configured test suites and returns the result.
	// The runner streams output to stdout/stderr in real time.
	Run(ctx context.Context, config RunConfig) (*RunResult, error)
}
