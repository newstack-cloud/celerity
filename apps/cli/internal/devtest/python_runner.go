package devtest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/newstack-cloud/celerity/apps/cli/internal/consts"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devtest/detect"
	"go.uber.org/zap"
)

// PythonRunner executes tests for Python projects.
type PythonRunner struct {
	logger *zap.Logger
}

// NewPythonRunner creates a new Python test runner.
func NewPythonRunner(logger *zap.Logger) *PythonRunner {
	return &PythonRunner{logger: logger}
}

// Run executes the configured test suites for a Python project.
func (r *PythonRunner) Run(ctx context.Context, config RunConfig) (*RunResult, error) {
	if config.TestCommand != "" {
		return r.runShellCommand(ctx, config)
	}

	pytestBin := detect.FindPytest(config.AppDir)
	if pytestBin == "" {
		// Check system PATH as fallback.
		if path, err := exec.LookPath("pytest"); err == nil {
			pytestBin = path
		}
	}
	if pytestBin == "" {
		return nil, fmt.Errorf(
			"pytest not found; install it in a virtualenv (.venv or venv) "+
				"or system-wide, or use --test-command",
		)
	}

	commands := r.buildCommands(pytestBin, config)
	if len(commands) == 0 {
		commands = []testCommand{
			{args: detect.PytestArgs(pytestBin, nil, config.Coverage), label: "all"},
		}
	}

	return r.runCommands(ctx, commands, config)
}

func (r *PythonRunner) runShellCommand(ctx context.Context, config RunConfig) (*RunResult, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", config.TestCommand)
	cmd.Dir = config.AppDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = r.buildEnv(config)

	r.logger.Debug("running user test command", zap.String("command", config.TestCommand))
	err := cmd.Run()
	return &RunResult{ExitCode: exitCodeFromError(err)}, nil
}

func (r *PythonRunner) buildCommands(pytestBin string, config RunConfig) []testCommand {
	suites := expandSuites(config.Suites)
	testConv, _ := consts.TestConventionsForRuntime(config.Runtime)

	var localDirs []string
	var localLabels []string
	var apiDirs []string

	for _, suite := range suites {
		dirs := testConv.DefaultTestDirs[string(suite)]
		if len(dirs) == 0 {
			continue
		}
		existingDirs := filterExistingDirs(config.AppDir, dirs)
		if len(existingDirs) == 0 {
			continue
		}
		if suite == SuiteAPI {
			apiDirs = append(apiDirs, existingDirs...)
		} else {
			localDirs = append(localDirs, existingDirs...)
			localLabels = append(localLabels, string(suite))
		}
	}

	var commands []testCommand

	if len(localDirs) > 0 {
		label := strings.Join(localLabels, "+")
		args := detect.PytestArgs(pytestBin, localDirs, config.Coverage)
		commands = append(commands, testCommand{args: args, label: label})
	}

	if len(apiDirs) > 0 {
		args := detect.PytestArgs(pytestBin, apiDirs, false)
		commands = append(commands, testCommand{args: args, label: "api"})
	}

	return commands
}

func (r *PythonRunner) runCommands(ctx context.Context, commands []testCommand, config RunConfig) (*RunResult, error) {
	env := r.buildEnv(config)

	for _, tc := range commands {
		r.logger.Debug("running test command",
			zap.String("suite", tc.label),
			zap.Strings("args", tc.args),
		)

		cmd := exec.CommandContext(ctx, tc.args[0], tc.args[1:]...)
		cmd.Dir = config.AppDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = env

		if err := cmd.Run(); err != nil {
			exitCode := exitCodeFromError(err)
			if exitCode != 0 {
				return &RunResult{ExitCode: exitCode}, nil
			}
		}
	}

	return &RunResult{ExitCode: 0}, nil
}

func (r *PythonRunner) buildEnv(config RunConfig) []string {
	env := os.Environ()
	for k, v := range config.Env {
		env = append(env, k+"="+v)
	}
	if config.HostPort != "" {
		env = append(env, "CELERITY_TEST_BASE_URL=http://localhost:"+config.HostPort)
	}
	return env
}
