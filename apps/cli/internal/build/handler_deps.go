package build

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// ReadHandlerDependencies reads per-handler dependency declarations from the
// language-appropriate manifest in appDir. Returns nil if no declarations exist.
func ReadHandlerDependencies(registry *Registry, runtime, appDir string) (map[string][]string, error) {
	reader := registry.GetHandlerDependencyReader(runtime)
	if reader == nil {
		return nil, nil
	}
	return reader.ReadHandlerDependencies(appDir)
}

// NodeHandlerDependencyReader reads celerity.handlerDependencies from package.json.
type NodeHandlerDependencyReader struct{}

func (r *NodeHandlerDependencyReader) ReadHandlerDependencies(appDir string) (map[string][]string, error) {
	path := filepath.Join(appDir, "package.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading package.json: %w", err)
	}

	var pkg struct {
		Celerity *struct {
			HandlerDependencies map[string][]string `json:"handlerDependencies"`
		} `json:"celerity"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parsing package.json: %w", err)
	}

	if pkg.Celerity == nil {
		return nil, nil
	}

	return pkg.Celerity.HandlerDependencies, nil
}

// PythonHandlerDependencyReader reads [tool.celerity.handlerDependencies]
// from pyproject.toml.
type PythonHandlerDependencyReader struct{}

func (r *PythonHandlerDependencyReader) ReadHandlerDependencies(appDir string) (map[string][]string, error) {
	path := filepath.Join(appDir, "pyproject.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading pyproject.toml: %w", err)
	}

	var pyproject struct {
		Tool struct {
			Celerity struct {
				HandlerDependencies map[string][]string `toml:"handlerDependencies"`
			} `toml:"celerity"`
		} `toml:"tool"`
	}
	if err := toml.Unmarshal(data, &pyproject); err != nil {
		return nil, fmt.Errorf("parsing pyproject.toml: %w", err)
	}

	deps := pyproject.Tool.Celerity.HandlerDependencies
	if len(deps) == 0 {
		return nil, nil
	}

	return deps, nil
}
