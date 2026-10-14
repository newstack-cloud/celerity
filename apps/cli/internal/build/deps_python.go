package build

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// PythonDependencyInstaller installs Python production dependencies using pip.
type PythonDependencyInstaller struct {
	Exec CommandExecutor
}

func (p *PythonDependencyInstaller) InstallDependencies(
	ctx context.Context,
	appDir string,
	targetDir string,
	packages []string,
	toolOutput io.Writer,
) error {
	if packages != nil {
		return p.installSubset(ctx, targetDir, packages, toolOutput)
	}
	return p.installAll(ctx, appDir, targetDir, toolOutput)
}

func (p *PythonDependencyInstaller) installAll(
	ctx context.Context,
	appDir string,
	targetDir string,
	toolOutput io.Writer,
) error {
	requirementsTxt := filepath.Join(appDir, "requirements.txt")
	pyprojectToml := filepath.Join(appDir, "pyproject.toml")

	if _, err := os.Stat(requirementsTxt); err == nil {
		return p.Exec.Run(ctx, appDir, toolOutput,
			"pip", "install", "-r", requirementsTxt, "-t", targetDir, "--no-user")
	}

	if _, err := os.Stat(pyprojectToml); err == nil {
		return p.Exec.Run(ctx, appDir, toolOutput,
			"pip", "install", appDir, "-t", targetDir, "--no-user")
	}

	return nil
}

func (p *PythonDependencyInstaller) installSubset(
	ctx context.Context,
	targetDir string,
	packages []string,
	toolOutput io.Writer,
) error {
	if len(packages) == 0 {
		return nil
	}

	args := []string{"install", "-t", targetDir, "--no-user"}
	args = append(args, packages...)

	err := p.Exec.Run(ctx, "", toolOutput, "pip", args...)
	if err != nil {
		return fmt.Errorf("pip install failed for packages [%s]: %w",
			strings.Join(packages, ", "), err)
	}

	return nil
}
