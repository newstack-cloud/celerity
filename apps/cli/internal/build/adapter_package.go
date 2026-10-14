package build

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Reads whichever manifest the runtime uses. Only declaration is checked, not
// installation, because the build installs dependencies from the manifest; a
// declared package reaches the bundle and an undeclared one never will, however
// the local node_modules or virtualenv happens to look.
func appDeclaresPackage(runtime, appDir, pkg string) (bool, error) {
	switch {
	case strings.HasPrefix(runtime, "nodejs"):
		return nodePackageJSONDeclares(appDir, pkg)
	case strings.HasPrefix(runtime, "python"):
		return pythonProjectDeclares(appDir, pkg)
	}
	// An unknown runtime has no manifest this can read, so there is nothing to
	// contradict the bootstrap.
	return true, nil
}

func nodePackageJSONDeclares(appDir, pkg string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(appDir, "package.json"))
	if err != nil {
		return false, fmt.Errorf("reading package.json: %w", err)
	}

	var manifest struct {
		Dependencies     map[string]string `json:"dependencies"`
		DevDependencies  map[string]string `json:"devDependencies"`
		PeerDependencies map[string]string `json:"peerDependencies"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return false, fmt.Errorf("parsing package.json: %w", err)
	}

	// devDependencies and peerDependencies count as declarations here. The
	// runtime container provides the SDK packages as peers in some setups, and
	// reporting a package as missing when the manifest names it would send a
	// developer looking in the wrong place.
	for _, deps := range []map[string]string{
		manifest.Dependencies,
		manifest.DevDependencies,
		manifest.PeerDependencies,
	} {
		if _, has := deps[pkg]; has {
			return true, nil
		}
	}

	return false, nil
}

// Scanned line by line rather than parsed as TOML, matching how the SDK
// presence check reads the same file, since this is only a substring test.
func pythonProjectDeclares(appDir, distribution string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(appDir, "pyproject.toml"))
	if err != nil {
		return false, fmt.Errorf("reading pyproject.toml: %w", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		stripped := strings.TrimSpace(line)
		if strings.HasPrefix(stripped, "#") {
			continue
		}
		if strings.Contains(stripped, distribution) {
			return true, nil
		}
	}

	return false, nil
}
