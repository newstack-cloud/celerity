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

// NodeRunner executes tests for Node.js projects.
type NodeRunner struct {
	logger *zap.Logger
}

// NewNodeRunner creates a new Node.js test runner.
func NewNodeRunner(logger *zap.Logger) *NodeRunner {
	return &NodeRunner{logger: logger}
}

// Run executes the configured test suites for a Node.js project.
func (r *NodeRunner) Run(ctx context.Context, config RunConfig) (*RunResult, error) {
	pkg, err := detect.LoadPackageJSON(config.AppDir)
	if err != nil {
		return nil, fmt.Errorf("reading package.json: %w", err)
	}

	if config.TestCommand != "" {
		return r.runShellCommand(ctx, config, pkg)
	}

	pm := detect.DetectPackageManager(pkg, config.AppDir)
	commands := r.buildCommands(pkg, pm, config)

	if len(commands) == 0 {
		return nil, fmt.Errorf(
			"no test scripts found in package.json and no test framework detected; "+
				"add a \"test\" script to package.json or use --test-command",
		)
	}

	return r.runCommands(ctx, commands, config, pkg)
}

func (r *NodeRunner) runShellCommand(ctx context.Context, config RunConfig, pkg *detect.PackageJSON) (*RunResult, error) {
	shBin, err := exec.LookPath("sh")
	if err != nil {
		return nil, fmt.Errorf("sh not found on PATH: %w", err)
	}
	cmd := exec.CommandContext(ctx, shBin, "-c", config.TestCommand)
	cmd.Dir = config.AppDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = r.buildEnv(config, pkg)

	r.logger.Debug("running user test command", zap.String("command", config.TestCommand))
	err = cmd.Run()
	return &RunResult{ExitCode: exitCodeFromError(err)}, nil
}

func (r *NodeRunner) buildCommands(pkg *detect.PackageJSON, pm string, config RunConfig) []testCommand {
	suites := expandSuites(config.Suites)
	testConv, _ := consts.TestConventionsForRuntime(config.Runtime)
	framework := detect.DetectFramework(pkg)

	var combinable, isolated []TestSuite
	for _, s := range suites {
		if s == SuiteAPI {
			isolated = append(isolated, s)
		} else {
			combinable = append(combinable, s)
		}
	}

	var commands []testCommand

	if len(combinable) == 1 {
		if cmd := r.suiteCommand(pkg, pm, framework, testConv, combinable[0], config); cmd != nil {
			commands = append(commands, *cmd)
		}
	} else if len(combinable) > 1 {
		if cmd := r.combinedSuiteCommand(pm, framework, testConv, combinable, config); cmd != nil {
			commands = append(commands, *cmd)
		}
	}

	for _, suite := range isolated {
		if cmd := r.suiteCommand(pkg, pm, framework, testConv, suite, config); cmd != nil {
			commands = append(commands, *cmd)
		}
	}

	if len(commands) == 0 && pkg.Scripts["test"] != "" {
		args := []string{pm, "run", "test"}
		if config.Coverage && pkg.Scripts["test:cov"] != "" {
			args = []string{pm, "run", "test:cov"}
		}
		commands = append(commands, testCommand{args: args, label: "all"})
	}

	return commands
}

func (r *NodeRunner) suiteCommand(
	pkg *detect.PackageJSON,
	pm string,
	framework string,
	testConv consts.TestConventions,
	suite TestSuite,
	config RunConfig,
) *testCommand {
	if cmd := r.scriptCommandForSuite(pkg, pm, suite, config.Coverage); cmd != nil {
		return cmd
	}
	if framework == "" {
		return nil
	}
	dirs := filterExistingDirs(config.AppDir, testConv.DefaultTestDirs[string(suite)])
	if len(dirs) == 0 {
		return nil
	}
	args := detect.FrameworkCommand(pm, framework, dirs, config.Coverage)
	return &testCommand{args: args, label: string(suite)}
}

func (r *NodeRunner) combinedSuiteCommand(
	pm string,
	framework string,
	testConv consts.TestConventions,
	suites []TestSuite,
	config RunConfig,
) *testCommand {
	if framework == "" {
		return nil
	}
	var allDirs []string
	var labels []string
	for _, suite := range suites {
		dirs := filterExistingDirs(config.AppDir, testConv.DefaultTestDirs[string(suite)])
		if len(dirs) > 0 {
			allDirs = append(allDirs, dirs...)
			labels = append(labels, string(suite))
		}
	}
	if len(allDirs) == 0 {
		return nil
	}
	label := strings.Join(labels, "+")
	args := detect.FrameworkCommand(pm, framework, allDirs, config.Coverage)
	return &testCommand{args: args, label: label}
}

func (r *NodeRunner) scriptCommandForSuite(
	pkg *detect.PackageJSON,
	pm string,
	suite TestSuite,
	coverage bool,
) *testCommand {
	scriptKey := "test:" + string(suite)

	if coverage {
		covKey := scriptKey + ":cov"
		if pkg.Scripts[covKey] != "" {
			return &testCommand{args: []string{pm, "run", covKey}, label: string(suite)}
		}
	}

	if pkg.Scripts[scriptKey] != "" {
		return &testCommand{args: []string{pm, "run", scriptKey}, label: string(suite)}
	}

	return nil
}

func (r *NodeRunner) runCommands(ctx context.Context, commands []testCommand, config RunConfig, pkg *detect.PackageJSON) (*RunResult, error) {
	env := r.buildEnv(config, pkg)
	var lastExitCode int

	for _, tc := range commands {
		r.logger.Debug("running test command",
			zap.String("suite", tc.label),
			zap.Strings("args", tc.args),
		)

		bin, err := exec.LookPath(tc.args[0])
		if err != nil {
			return nil, fmt.Errorf("%s not found on PATH: %w", tc.args[0], err)
		}
		cmd := exec.CommandContext(ctx, bin, tc.args[1:]...)
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
		lastExitCode = 0
	}

	return &RunResult{ExitCode: lastExitCode}, nil
}

func (r *NodeRunner) buildEnv(config RunConfig, pkg *detect.PackageJSON) []string {
	env := os.Environ()
	for k, v := range config.Env {
		env = append(env, k+"="+v)
	}
	if config.HostPort != "" {
		env = append(env, "CELERITY_TEST_BASE_URL=http://localhost:"+config.HostPort)
	}
	if pkg.IsESM() {
		env = append(env, "NODE_OPTIONS=--experimental-vm-modules")
	}
	return env
}
