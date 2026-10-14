package build

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// NodeDependencyInstaller installs Node.js production dependencies
// using the project's package manager.
type NodeDependencyInstaller struct {
	Exec CommandExecutor
	PM   NodePackageManager
}

func (n *NodeDependencyInstaller) InstallDependencies(
	ctx context.Context,
	appDir string,
	targetDir string,
	packages []string,
	toolOutput io.Writer,
) error {
	if packages != nil {
		return n.installSubset(ctx, appDir, targetDir, packages, toolOutput)
	}
	return n.installAll(ctx, appDir, targetDir, toolOutput)
}

func (n *NodeDependencyInstaller) installAll(
	ctx context.Context,
	appDir string,
	targetDir string,
	toolOutput io.Writer,
) error {
	if err := copyFile(
		filepath.Join(appDir, "package.json"),
		filepath.Join(targetDir, "package.json"),
	); err != nil {
		return err
	}
	for _, lf := range n.PM.LockFiles() {
		src := filepath.Join(appDir, lf)
		if _, err := os.Stat(src); err == nil {
			if err := copyFile(src, filepath.Join(targetDir, lf)); err != nil {
				return err
			}
		}
	}

	cmd := n.PM.InstallCommand()
	return n.Exec.Run(ctx, targetDir, toolOutput, cmd[0], cmd[1:]...)
}

func (n *NodeDependencyInstaller) installSubset(
	ctx context.Context,
	appDir string,
	targetDir string,
	packages []string,
	toolOutput io.Writer,
) error {
	origPkgJSON, err := readPackageJSON(filepath.Join(appDir, "package.json"))
	if err != nil {
		return err
	}

	filteredDeps := filterDependencies(origPkgJSON, packages)

	minimalPkg := map[string]any{
		"name":         origPkgJSON["name"],
		"version":      origPkgJSON["version"],
		"dependencies": filteredDeps,
	}
	minimalJSON, err := json.MarshalIndent(minimalPkg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling filtered package.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "package.json"), minimalJSON, 0644); err != nil {
		return err
	}

	for _, lf := range n.PM.LockFiles() {
		src := filepath.Join(appDir, lf)
		if _, err := os.Stat(src); err == nil {
			if err := copyFile(src, filepath.Join(targetDir, lf)); err != nil {
				return err
			}
		}
	}

	cmd := n.PM.InstallCommand()
	return n.Exec.Run(ctx, targetDir, toolOutput, cmd[0], cmd[1:]...)
}

func readPackageJSON(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading package.json: %w", err)
	}
	var pkg map[string]any
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parsing package.json: %w", err)
	}
	return pkg, nil
}

func filterDependencies(pkgJSON map[string]any, packages []string) map[string]string {
	allDeps := map[string]string{}
	if deps, ok := pkgJSON["dependencies"].(map[string]any); ok {
		for name, ver := range deps {
			if v, ok := ver.(string); ok {
				allDeps[name] = v
			}
		}
	}

	filtered := map[string]string{}
	for _, pkg := range packages {
		if ver, ok := allDeps[pkg]; ok {
			filtered[pkg] = ver
		}
	}

	return filtered
}
