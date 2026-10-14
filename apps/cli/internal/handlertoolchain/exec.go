package handlertoolchain

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
)

// Runner runs the external commands a toolchain drives.
//
// Declared here rather than reused from internal/build so this package stays a
// leaf. The two are the same shape by coincidence of both shelling out rather
// than because they share a contract.
type Runner interface {
	// Run executes name with args in dir, with env added to the process
	// environment, streaming output to toolOutput.
	Run(ctx context.Context, dir string, env map[string]string, toolOutput io.Writer, name string, args ...string) error
	// Output executes the command and returns its standard output, streaming
	// only standard error to toolOutput.
	Output(ctx context.Context, dir string, env map[string]string, toolOutput io.Writer, name string, args ...string) ([]byte, error)
}

// ExecRunner is the Runner backed by os/exec.
type ExecRunner struct{}

func (ExecRunner) Run(
	ctx context.Context,
	dir string,
	env map[string]string,
	toolOutput io.Writer,
	name string,
	args ...string,
) error {
	cmd := command(ctx, dir, env, name, args...)
	cmd.Stdout = toolOutput
	cmd.Stderr = toolOutput
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running %s: %w", name, err)
	}
	return nil
}

func (ExecRunner) Output(
	ctx context.Context,
	dir string,
	env map[string]string,
	toolOutput io.Writer,
	name string,
	args ...string,
) ([]byte, error) {
	cmd := command(ctx, dir, env, name, args...)
	cmd.Stderr = toolOutput
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running %s: %w", name, err)
	}
	return out, nil
}

func command(ctx context.Context, dir string, env map[string]string, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), envPairs(env)...)
	return cmd
}

// Sorted so a command line reproduces between runs and a test can assert on
// it.
func envPairs(env map[string]string) []string {
	pairs := make([]string, 0, len(env))
	for key, value := range env {
		pairs = append(pairs, key+"="+value)
	}
	sort.Strings(pairs)
	return pairs
}
