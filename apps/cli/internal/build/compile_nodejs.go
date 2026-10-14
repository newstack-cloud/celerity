package build

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

// NodeSourceCompiler compiles TypeScript source to JavaScript using tsc,
// preserving the directory structure for readability in cloud provider consoles.
// If no tsconfig.json is found, it assumes the project is plain JavaScript
// and does nothing.
type NodeSourceCompiler struct {
	Exec CommandExecutor
	PM   NodePackageManager
}

func (c *NodeSourceCompiler) Compile(
	ctx context.Context,
	srcDir string,
	outDir string,
	toolOutput io.Writer,
) error {
	tsconfig := filepath.Join(srcDir, "tsconfig.json")
	if _, err := os.Stat(tsconfig); os.IsNotExist(err) {
		return nil
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	cmd := c.PM.ExecPrefix()
	cmd = append(cmd, "tsc", "--outDir", outDir, "--declaration", "false", "--sourceMap", "false")

	return c.Exec.Run(ctx, srcDir, toolOutput, cmd[0], cmd[1:]...)
}
