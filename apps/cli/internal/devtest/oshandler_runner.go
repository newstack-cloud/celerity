package devtest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/newstack-cloud/celerity/apps/cli/internal/handlertoolchain"
	"go.uber.org/zap"
)

// OSHandlerRunner executes tests for an application on an OS-only runtime.
//
// Which command that is belongs to the toolchain rather than to this package.
// One OS-only runtime serves Go, Rust, Zig and C++, whose test layouts and
// runners have nothing in common, so there is no per-runtime convention to
// detect the way there is for Node.js and Python.
type OSHandlerRunner struct {
	toolchain handlertoolchain.Toolchain
	logger    *zap.Logger
}

// NewOSHandlerRunner creates a runner that defers to the given toolchain.
func NewOSHandlerRunner(toolchain handlertoolchain.Toolchain, logger *zap.Logger) *OSHandlerRunner {
	return &OSHandlerRunner{toolchain: toolchain, logger: logger}
}

// Run executes the configured test suites.
func (r *OSHandlerRunner) Run(ctx context.Context, config RunConfig) (*RunResult, error) {
	if config.TestCommand != "" {
		return r.run(ctx, config, []string{"sh", "-c", config.TestCommand})
	}

	if r.toolchain == nil {
		return nil, fmt.Errorf(
			"no toolchain resolved for runtime %q, so there is no test command to run.\n"+
				"Pass --test-command, or declare handlerBuild.test in your deploy config",
			config.Runtime,
		)
	}

	suites := expandSuites(config.Suites)
	var commands [][]string
	var unconfigured []string
	for _, suite := range suites {
		command, ok := r.toolchain.TestCommand(string(suite))
		if !ok {
			unconfigured = append(unconfigured, string(suite))
			continue
		}
		commands = append(commands, command)
	}

	if len(commands) == 0 {
		return nil, fmt.Errorf(
			"the %s toolchain has no test command for %s.\n"+
				"Declare handlerBuild.test for those suites, or pass --test-command",
			r.toolchain.Name(), strings.Join(unconfigured, ", "),
		)
	}
	for _, suite := range unconfigured {
		r.logger.Warn("no test command for suite, skipping",
			zap.String("suite", suite),
			zap.String("toolchain", r.toolchain.Name()),
		)
	}

	// The worst exit code wins, so one failing suite fails the run even where
	// a later one passes.
	result := &RunResult{}
	for _, command := range commands {
		suiteResult, err := r.run(ctx, config, command)
		if err != nil {
			return nil, err
		}
		if suiteResult.ExitCode != 0 && result.ExitCode == 0 {
			result.ExitCode = suiteResult.ExitCode
		}
	}
	return result, nil
}

func (r *OSHandlerRunner) run(ctx context.Context, config RunConfig, command []string) (*RunResult, error) {
	bin, err := exec.LookPath(command[0])
	if err != nil {
		return nil, fmt.Errorf("%s not found on PATH: %w", command[0], err)
	}

	cmd := exec.CommandContext(ctx, bin, command[1:]...)
	cmd.Dir = config.AppDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = r.buildEnv(config)

	r.logger.Debug("running tests", zap.Strings("command", command))
	err = cmd.Run()
	return &RunResult{ExitCode: exitCodeFromError(err)}, nil
}

func (r *OSHandlerRunner) buildEnv(config RunConfig) []string {
	env := os.Environ()
	for k, v := range config.Env {
		env = append(env, k+"="+v)
	}
	if config.HostPort != "" {
		env = append(env, "CELERITY_TEST_BASE_URL=http://localhost:"+config.HostPort)
	}
	return env
}
