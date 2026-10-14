package build

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func isESMProject(appDir string) bool {
	data, err := os.ReadFile(filepath.Join(appDir, "package.json"))
	if err != nil {
		return false
	}
	var pkg struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}
	return pkg.Type == "module"
}

// NodePackageManager represents a detected Node.js package manager.
type NodePackageManager string

const (
	NodePMNpm  NodePackageManager = "npm"
	NodePMYarn NodePackageManager = "yarn"
	NodePMPnpm NodePackageManager = "pnpm"
)

// DetectNodePackageManager determines the Node.js package manager
// from the packageManager field in package.json or lock files on disk.
func DetectNodePackageManager(appDir string) NodePackageManager {
	if pm := detectFromPackageJSON(appDir); pm != "" {
		return pm
	}
	return detectFromLockFile(appDir)
}

func detectFromPackageJSON(appDir string) NodePackageManager {
	data, err := os.ReadFile(filepath.Join(appDir, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		PackageManager string `json:"packageManager"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil || pkg.PackageManager == "" {
		return ""
	}
	pm := strings.Split(pkg.PackageManager, "@")[0]
	switch pm {
	case "yarn":
		return NodePMYarn
	case "pnpm":
		return NodePMPnpm
	case "npm":
		return NodePMNpm
	default:
		return ""
	}
}

func detectFromLockFile(appDir string) NodePackageManager {
	lockFiles := []struct {
		file string
		pm   NodePackageManager
	}{
		{"yarn.lock", NodePMYarn},
		{"pnpm-lock.yaml", NodePMPnpm},
		{"package-lock.json", NodePMNpm},
	}
	for _, lf := range lockFiles {
		if _, err := os.Stat(filepath.Join(appDir, lf.file)); err == nil {
			return lf.pm
		}
	}
	return NodePMNpm
}

// ExecPrefix returns the command prefix for running a locally installed
// binary via the package manager (e.g. ["pnpm", "exec"] or ["yarn"]).
func (pm NodePackageManager) ExecPrefix() []string {
	switch pm {
	case NodePMYarn:
		return []string{"yarn"}
	case NodePMPnpm:
		return []string{"pnpm", "exec"}
	default:
		return []string{"npm", "exec", "--"}
	}
}

// InstallCommand returns the command to install production dependencies
// in a clean, reproducible way.
func (pm NodePackageManager) InstallCommand() []string {
	switch pm {
	case NodePMYarn:
		return []string{"yarn", "install", "--production"}
	case NodePMPnpm:
		return []string{"pnpm", "install", "--prod"}
	default:
		return []string{"npm", "ci", "--omit=dev"}
	}
}

// LockFiles returns the lock file names to copy for reproducible installs.
func (pm NodePackageManager) LockFiles() []string {
	switch pm {
	case NodePMYarn:
		return []string{"yarn.lock"}
	case NodePMPnpm:
		return []string{"pnpm-lock.yaml"}
	default:
		return []string{"package-lock.json"}
	}
}
