package handlertoolchain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/newstack-cloud/celerity/apps/cli/internal/preprocess"
)

// Environment variables the CLI injects into every handlerBuild command, so a
// build script can key off what it is being asked for rather than hard-coding
// one platform and one deploy target.
const (
	EnvBuildOS           = "CELERITY_BUILD_OS"
	EnvBuildArch         = "CELERITY_BUILD_ARCH"
	EnvBuildOutput       = "CELERITY_BUILD_OUTPUT"
	EnvBuildDeployTarget = "CELERITY_BUILD_DEPLOY_TARGET"
	EnvBuildMode         = "CELERITY_BUILD_MODE"
)

// customToolchain drives a handlerBuild block.
type customToolchain struct {
	config *Config
	runner Runner
}

func newCustomToolchain(config *Config, runner Runner) *customToolchain {
	return &customToolchain{config: config, runner: runner}
}

func (t *customToolchain) Name() string { return "custom" }

func (t *customToolchain) Validate(string) error {
	// Resolve only selects this toolchain from a handlerBuild block it has
	// already validated, so there is nothing left to check here. A missing
	// compiler surfaces when the build command runs, which is where the error
	// means something.
	return nil
}

// Prepare does nothing, since code generation for a community SDK is the
// build command's own business.
func (t *customToolchain) Prepare(context.Context, PrepareParams) error { return nil }

func (t *customToolchain) BuildBinary(ctx context.Context, p BinaryParams) error {
	cfg := t.config.forTarget(p.DeployTarget)

	output, err := filepath.Abs(p.OutputPath)
	if err != nil {
		return fmt.Errorf("resolving the handlers executable output path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("creating the output directory: %w", err)
	}

	env := buildEnv(cfg.Env, p, output)
	if err := t.runner.Run(ctx, p.AppDir, env, toolOutput(ctx), "sh", "-c", cfg.Command); err != nil {
		return fmt.Errorf("handlerBuild.command failed: %w", err)
	}
	if cfg.Tidy != "" {
		if err := t.runner.Run(ctx, p.AppDir, env, toolOutput(ctx), "sh", "-c", cfg.Tidy); err != nil {
			return fmt.Errorf("handlerBuild.tidy failed: %w", err)
		}
	}

	// The command may have written straight to CELERITY_BUILD_OUTPUT, in
	// which case the declared binary path is the same file and there is
	// nothing to copy.
	built := cfg.Binary
	if !filepath.IsAbs(built) {
		built = filepath.Join(p.AppDir, built)
	}
	if sameFile(built, output) {
		return ensureExecutable(output)
	}

	if err := copyExecutable(built, output); err != nil {
		return fmt.Errorf(
			"handlerBuild.command ran but %s is not there: %w.\n"+
				"handlerBuild.binary must name where the command leaves the executable",
			cfg.Binary, err,
		)
	}
	return nil
}

func (t *customToolchain) Extract(ctx context.Context, p ExtractParams) (*preprocess.HandlerManifest, error) {
	cfg := t.config
	if cfg.Extract == nil {
		// With no extractor the blueprint's hand-authored handler resources
		// stand on their own.
		return nil, nil
	}

	out, err := t.runner.Output(ctx, p.AppDir, cfg.Env, toolOutput(ctx), "sh", "-c", cfg.Extract.Command)
	if err != nil {
		return nil, fmt.Errorf("handlerBuild.extract.command failed: %w", err)
	}

	data := out
	if cfg.Extract.ManifestFile != "" {
		path := cfg.Extract.ManifestFile
		if !filepath.IsAbs(path) {
			path = filepath.Join(p.AppDir, path)
		}
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading the manifest at handlerBuild.extract.manifestFile: %w", err)
		}
	}

	var manifest preprocess.HandlerManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing the handler manifest: %w", err)
	}
	if err := preprocess.ValidateManifest(&manifest); err != nil {
		return nil, fmt.Errorf("handlerBuild.extract.command produced an unusable manifest: %w", err)
	}
	return &manifest, nil
}

func (t *customToolchain) WatchPaths() WatchPaths { return t.config.watchPaths() }

func (t *customToolchain) TestCommand(suite string) ([]string, bool) {
	cmd, ok := t.config.Test[suite]
	if !ok || cmd == "" {
		return nil, false
	}
	return []string{"sh", "-c", cmd}, true
}

func buildEnv(base map[string]string, p BinaryParams, output string) map[string]string {
	env := make(map[string]string, len(base)+5)
	for key, value := range base {
		env[key] = value
	}
	env[EnvBuildOS] = p.OS
	env[EnvBuildArch] = p.Arch
	env[EnvBuildOutput] = output
	env[EnvBuildDeployTarget] = p.DeployTarget
	env[EnvBuildMode] = p.Mode
	return env
}

type toolOutputKey struct{}

// WithToolOutput returns a context carrying where a toolchain's command output
// should be streamed.
//
// On the context because the sink is a property of how the CLI is being run,
// a log file when a TUI owns the terminal and stdout when it does not, rather
// than of the build being asked for. Threading it through every params struct
// would put it in the signature of calls that never produce any output.
func WithToolOutput(ctx context.Context, w io.Writer) context.Context {
	return context.WithValue(ctx, toolOutputKey{}, w)
}

func toolOutput(ctx context.Context) io.Writer {
	if w, ok := ctx.Value(toolOutputKey{}).(io.Writer); ok && w != nil {
		return w
	}
	return io.Discard
}
