package devconfig

import (
	"context"
	"fmt"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/newstack-cloud/celerity/apps/cli/internal/blueprint"
	"github.com/newstack-cloud/celerity/apps/cli/internal/handlertoolchain"
	"github.com/newstack-cloud/celerity/apps/cli/internal/preprocess"
	"go.uber.org/zap"
)

const (
	// DevHandlersPath is where a dev session's handlers executable is built,
	// relative to the application directory.
	//
	// Under .celerity because the application directory is already bind
	// mounted into the container, so an executable written there needs no
	// mount of its own, and .celerity is the directory the CLI already owns
	// and projects already ignore.
	DevHandlersPath = ".celerity/dev/handlers"

	// containerAppDir is where the application directory is mounted.
	containerAppDir = "/opt/celerity/app"
)

// ContainerHandlersExecutable is the path the core runtime starts, as seen
// from inside the container.
func ContainerHandlersExecutable() string {
	return containerAppDir + "/" + DevHandlersPath
}

// resolveToolchain resolves the build tooling for an OS-only runtime, reading
// the handlerBuild block from the deploy config where there is one.
func resolveToolchain(runtime, appDir, deployConfigPath string) (handlertoolchain.Toolchain, error) {
	if !handlertoolchain.IsOSOnly(runtime) {
		return nil, nil
	}

	var handlerBuild *handlertoolchain.Config
	if deployConfigPath != "" {
		var err error
		handlerBuild, err = ReadHandlerBuild(deployConfigPath)
		if err != nil {
			return nil, err
		}
	}

	return handlertoolchain.Resolve(handlertoolchain.Options{
		Runtime: runtime,
		AppDir:  appDir,
		Config:  handlerBuild,
	})
}

// extractRequest is what a toolchain needs to report an application's
// handlers.
type extractRequest struct {
	AppDir       string
	Blueprint    *schema.Blueprint
	DeployTarget string
	// Telemetry names what traces are exported with, or is empty for none.
	//
	// Carried here rather than read where Prepare is called, because the
	// watcher re-prepares on every change through an extractor built from the
	// resolved config. Left out of it, a session that started with tracing
	// would regenerate without it on the first file saved and stop exporting
	// with nothing said.
	Telemetry string
}

// extractViaToolchain prepares the project for a dev session and reports what
// its handlers executable serves.
func extractViaToolchain(
	ctx context.Context,
	toolchain handlertoolchain.Toolchain,
	req extractRequest,
	logger *zap.Logger,
) (*preprocess.HandlerManifest, error) {
	// ModeDev, so the executable links what a local session reads: its own
	// config store and the emulator clients a deployed artefact must not
	// carry.
	err := toolchain.Prepare(ctx, handlertoolchain.PrepareParams{
		AppDir:        req.AppDir,
		DeployTarget:  req.DeployTarget,
		Mode:          handlertoolchain.ModeDev,
		ResourceKinds: blueprint.ResourceKinds(req.Blueprint),
		SQLEngines:    blueprint.SQLEngines(req.Blueprint),
		Telemetry:     req.Telemetry,
		// The session this prepares for is the one `celerity dev test` runs
		// its integration suites against, and those suites reach the session's
		// own resources through the same provider the application does.
		TestPackages: true,
	})
	if err != nil {
		return nil, fmt.Errorf("preparing the %s toolchain: %w", toolchain.Name(), err)
	}

	manifest, err := toolchain.Extract(ctx, handlertoolchain.ExtractParams{AppDir: req.AppDir})
	if err != nil {
		return nil, fmt.Errorf("handler extraction: %w", err)
	}
	if manifest == nil {
		logger.Info("the toolchain does not extract handlers, using raw blueprint",
			zap.String("toolchain", toolchain.Name()),
		)
		return &preprocess.HandlerManifest{Version: preprocess.ManifestVersion}, nil
	}
	return manifest, nil
}

// BuildDevHandlers builds the handlers executable a dev session runs.
//
// arch is the architecture the container runs at, which is the Docker daemon's
// rather than the CLI's: the two differ on a remote context or a Linux VM, and
// an executable built for the wrong one is refused by the kernel with nothing
// to say why.
func BuildDevHandlers(
	ctx context.Context,
	cfg *ResolvedConfig,
	arch string,
) error {
	if cfg.Toolchain == nil {
		return nil
	}

	err := cfg.Toolchain.BuildBinary(ctx, handlertoolchain.BinaryParams{
		AppDir:       cfg.AppDir,
		OS:           "linux",
		Arch:         arch,
		OutputPath:   cfg.HandlersExecutable,
		DeployTarget: cfg.DeployTarget,
		Mode:         handlertoolchain.ModeDev,
	})
	if err != nil {
		return fmt.Errorf("building the handlers executable: %w", err)
	}

	return handlertoolchain.CheckArchitecture(cfg.HandlersExecutable, arch)
}

// ToolchainExtractor reports an application's handlers through its toolchain.
//
// A thin adapter so a dev session's watcher can re-extract the same way it
// first did, without holding the resolved config or knowing which toolchain
// answered.
type ToolchainExtractor struct {
	toolchain handlertoolchain.Toolchain
	req       extractRequest
	logger    *zap.Logger
}

// NewToolchainExtractor returns an extractor for a resolved dev session.
func NewToolchainExtractor(cfg *ResolvedConfig) *ToolchainExtractor {
	return &ToolchainExtractor{
		toolchain: cfg.Toolchain,
		req: extractRequest{
			AppDir:       cfg.AppDir,
			Blueprint:    cfg.Blueprint,
			DeployTarget: cfg.DeployTarget,
			Telemetry:    cfg.Telemetry,
		},
		logger: zap.NewNop(),
	}
}

// WithLogger returns the extractor reporting through the given logger.
func (e *ToolchainExtractor) WithLogger(logger *zap.Logger) *ToolchainExtractor {
	e.logger = logger
	return e
}

// Extract reports the handlers the application serves.
func (e *ToolchainExtractor) Extract(ctx context.Context) (*preprocess.HandlerManifest, error) {
	return extractViaToolchain(ctx, e.toolchain, e.req, e.logger)
}
