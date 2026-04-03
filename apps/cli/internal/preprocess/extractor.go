package preprocess

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"go.uber.org/zap"
)

// ExtractorConfig configures the language-specific handler extraction CLI.
type ExtractorConfig struct {
	Runtime     string // e.g. "nodejs22.x"
	ModulePath  string // e.g. "src/app-module.ts"
	ProjectRoot string // Absolute path to the project directory
}

// Extractor invokes the language-specific CLI to scan developer code
// and produce a HandlerManifest describing all discovered handlers.
type Extractor struct {
	config ExtractorConfig
	logger *zap.Logger
}

// NewExtractor creates an Extractor with the given config.
func NewExtractor(config ExtractorConfig, logger *zap.Logger) *Extractor {
	return &Extractor{config: config, logger: logger}
}

// Extract runs the extraction CLI and returns the parsed manifest.
func (e *Extractor) Extract(ctx context.Context) (*HandlerManifest, error) {
	args, err := e.buildArgs()
	if err != nil {
		return nil, err
	}

	e.logger.Debug("running extraction CLI", zap.Strings("args", args))

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = e.config.ProjectRoot
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("handler extraction failed: %w", err)
	}

	var manifest HandlerManifest
	if err := json.Unmarshal(output, &manifest); err != nil {
		return nil, fmt.Errorf("parsing extraction output: %w", err)
	}

	e.logger.Debug("extraction complete",
		zap.Int("classHandlers", len(manifest.Handlers)),
		zap.Int("functionHandlers", len(manifest.FunctionHandlers)),
	)

	return &manifest, nil
}

func (e *Extractor) buildArgs() ([]string, error) {
	switch {
	case isNodeRuntime(e.config.Runtime):
		return e.buildNodeArgs()
	case isPythonRuntime(e.config.Runtime):
		return e.buildPythonArgs()
	default:
		return nil, fmt.Errorf("unsupported runtime %q for handler extraction", e.config.Runtime)
	}
}

func (e *Extractor) buildNodeArgs() ([]string, error) {
	extractBin := filepath.Join(e.config.ProjectRoot, "node_modules", ".bin", "celerity-extract")
	if _, err := os.Stat(extractBin); err != nil {
		return nil, fmt.Errorf(
			"extraction CLI not found at %s; ensure @celerity-sdk/cli is installed",
			extractBin,
		)
	}

	return []string{
		"node",
		"--import", "tsx",
		extractBin,
		"--module", e.config.ModulePath,
		"--project-root", e.config.ProjectRoot,
	}, nil
}

func (e *Extractor) buildPythonArgs() ([]string, error) {
	venvDirs := []string{".venv", "venv"}
	for _, venvDir := range venvDirs {
		extractBin := filepath.Join(e.config.ProjectRoot, venvDir, "bin", "celerity-extract")
		if _, err := os.Stat(extractBin); err == nil {
			return []string{
				extractBin,
				"--module", e.config.ModulePath,
				"--project-root", e.config.ProjectRoot,
			}, nil
		}
	}

	return nil, fmt.Errorf(
		"extraction CLI not found in .venv/bin/ or venv/bin/; ensure celerity-sdk[cli] is installed in your virtual environment",
	)
}

func isNodeRuntime(runtime string) bool {
	return len(runtime) >= 6 && runtime[:6] == "nodejs"
}

func isPythonRuntime(runtime string) bool {
	return len(runtime) >= 6 && runtime[:6] == "python"
}
