package build

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// ExecCommandExecutor is the default CommandExecutor that shells out
// to external tools via os/exec.
type ExecCommandExecutor struct{}

// Run invokes a sub-process and writes all output to the provided toolOutput writer.
// The command is resolved to an absolute path via PATH and an error
// is returned if it cannot be found.
func (e *ExecCommandExecutor) Run(
	ctx context.Context,
	dir string,
	toolOutput io.Writer,
	name string,
	args ...string,
) error {
	bin, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%s not found on PATH: %w", name, err)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Stdout = toolOutput
	cmd.Stderr = toolOutput
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}
