// Package handlerresolve is the shared extract + merge + collect pipeline
// used by both the dev and build commands so handler discovery behaves
// identically across them.
package handlerresolve

import (
	"context"
	"fmt"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/newstack-cloud/celerity/apps/cli/internal/blueprint"
	"github.com/newstack-cloud/celerity/apps/cli/internal/consts"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devconfig/resolve"
	"github.com/newstack-cloud/celerity/apps/cli/internal/handlertoolchain"
	"github.com/newstack-cloud/celerity/apps/cli/internal/preprocess"
	"go.uber.org/zap"
)

// ResolvedHandlers is the output of Resolve.
type ResolvedHandlers struct {
	Runtime         string
	ModulePath      string
	Handlers        []blueprint.HandlerInfo
	Blueprint       *schema.Blueprint
	MergedBlueprint *schema.Blueprint
	SpecFormat      schema.SpecFormat
	Manifest        *preprocess.HandlerManifest
	// Toolchain builds the handlers executable, and is nil for a runtime that
	// hosts the language itself. Carried here so callers thread the one that
	// was resolved rather than resolving their own.
	Toolchain handlertoolchain.Toolchain
}

// Options configures Resolve.
type Options struct {
	AppDir             string
	BlueprintFile      string
	ModulePathOverride string
	// Toolchain builds the handlers executable for an OS-only runtime.
	// Resolved by the caller, which is the only place that has both the app
	// directory and the deploy config; nil for every other runtime.
	Toolchain handlertoolchain.Toolchain
	// DeployTarget and Mode tell the toolchain what it is generating for.
	DeployTarget string
	Mode         string
	// Telemetry names what traces are exported with, or is empty for none.
	Telemetry string
}

// Extractor is an interface (rather than the concrete preprocess.Extractor)
// so tests can inject a stub without depending on the celerity-extract
// binary being installed on the host system.
type Extractor interface {
	Extract(ctx context.Context) (*preprocess.HandlerManifest, error)
}

// ExtractorFactory allows tests to substitute a stub extractor.
type ExtractorFactory func(cfg preprocess.ExtractorConfig, logger *zap.Logger) Extractor

func defaultExtractorFactory(cfg preprocess.ExtractorConfig, logger *zap.Logger) Extractor {
	return preprocess.NewExtractor(cfg, logger)
}

// Resolve extracts an application's handlers and merges them into its
// blueprint. The merged blueprint is not persisted to disk, which is a
// dev-specific concern, so callers decide whether to write it out.
func Resolve(
	ctx context.Context,
	opts Options,
	logger *zap.Logger,
) (*ResolvedHandlers, error) {
	return resolveWith(ctx, opts, defaultExtractorFactory, logger)
}

func resolveWith(
	ctx context.Context,
	opts Options,
	factory ExtractorFactory,
	logger *zap.Logger,
) (*ResolvedHandlers, error) {
	bp, specFormat, err := loadBlueprint(opts.BlueprintFile)
	if err != nil {
		return nil, err
	}

	runtime, err := detectRuntime(bp, opts.AppDir, logger)
	if err != nil {
		return nil, err
	}

	modulePath := resolveModulePath(opts.AppDir, opts.ModulePathOverride, runtime)

	// Prepared before extraction rather than before the build, because
	// extraction compiles the application and so fails on a project whose
	// generated platform imports are absent or belong to another deploy
	// target.
	if opts.Toolchain != nil {
		if err := prepareToolchain(ctx, opts, bp); err != nil {
			return nil, err
		}
	}

	manifest, err := runExtraction(ctx, runtime, modulePath, opts, factory, logger)
	if err != nil {
		return nil, err
	}

	// Collapsed here as well as inside Merge so the manifest returned to
	// callers describes the same handler resources the merged blueprint does.
	// A caller reading the uncollapsed one would build artefacts for resources
	// that no longer exist. The operation is idempotent, so the second call is
	// a no-op.
	manifest = preprocess.CollapseRoutedConsumers(manifest, logger)

	merged, err := preprocess.Merge(bp, manifest, logger)
	if err != nil {
		return nil, fmt.Errorf("blueprint merge: %w", err)
	}

	return &ResolvedHandlers{
		Runtime:         runtime,
		ModulePath:      modulePath,
		Handlers:        blueprint.CollectHandlerInfo(merged),
		Blueprint:       bp,
		MergedBlueprint: merged,
		SpecFormat:      specFormat,
		Manifest:        manifest,
		Toolchain:       opts.Toolchain,
	}, nil
}

func loadBlueprint(blueprintFile string) (*schema.Blueprint, schema.SpecFormat, error) {
	if blueprintFile == "" {
		return nil, "", fmt.Errorf("blueprint file path is required")
	}
	bp, specFormat, err := blueprint.LoadForDev(blueprintFile)
	if err != nil {
		return nil, "", fmt.Errorf("loading blueprint %s: %w", blueprintFile, err)
	}
	return bp, specFormat, nil
}

func detectRuntime(bp *schema.Blueprint, appDir string, logger *zap.Logger) (string, error) {
	runtime, err := blueprint.DetectRuntime(bp)
	if err == nil {
		return runtime, nil
	}

	// Decorator-driven projects have no handler resources in the blueprint
	// until after extraction, so fall back to project files for runtime detection.
	runtime, projectErr := blueprint.DetectRuntimeFromProject(appDir)
	if projectErr != nil {
		return "", fmt.Errorf("detecting runtime: %w", projectErr)
	}
	logger.Info("runtime auto-detected from project files",
		zap.String("runtime", runtime),
	)
	return runtime, nil
}

func resolveModulePath(appDir, override, runtime string) string {
	conv, ok := consts.ConventionsForRuntime(runtime)
	if !ok {
		return override
	}
	return resolve.ModulePath(appDir, override, conv.DefaultModulePaths)
}

func prepareToolchain(ctx context.Context, opts Options, bp *schema.Blueprint) error {
	err := opts.Toolchain.Prepare(ctx, handlertoolchain.PrepareParams{
		AppDir:        opts.AppDir,
		DeployTarget:  opts.DeployTarget,
		Mode:          opts.Mode,
		ResourceKinds: blueprint.ResourceKinds(bp),
		SQLEngines:    blueprint.SQLEngines(bp),
		Telemetry:     opts.Telemetry,
	})
	if err != nil {
		return fmt.Errorf("preparing the %s toolchain: %w", opts.Toolchain.Name(), err)
	}
	return nil
}

func runExtraction(
	ctx context.Context,
	runtime, modulePath string,
	opts Options,
	factory ExtractorFactory,
	logger *zap.Logger,
) (*preprocess.HandlerManifest, error) {
	// An OS-only runtime hosts no language, so the extractor belongs to
	// whichever toolchain built the executable rather than to the runtime.
	if opts.Toolchain != nil {
		manifest, err := opts.Toolchain.Extract(ctx, handlertoolchain.ExtractParams{AppDir: opts.AppDir})
		if err != nil {
			return nil, fmt.Errorf("handler extraction: %w", err)
		}
		if manifest == nil {
			logger.Info("the toolchain does not extract handlers, using raw blueprint",
				zap.String("toolchain", opts.Toolchain.Name()),
			)
			return &preprocess.HandlerManifest{Version: preprocess.ManifestVersion}, nil
		}
		return manifest, nil
	}

	conv, _ := consts.ConventionsForRuntime(runtime)
	if !conv.SupportsExtraction {
		logger.Info("handler extraction not available for runtime, using raw blueprint",
			zap.String("runtime", runtime),
		)
		return &preprocess.HandlerManifest{Version: preprocess.ManifestVersion}, nil
	}

	extractor := factory(preprocess.ExtractorConfig{
		Runtime:     runtime,
		ModulePath:  modulePath,
		ProjectRoot: opts.AppDir,
	}, logger)
	manifest, err := extractor.Extract(ctx)
	if err != nil {
		return nil, fmt.Errorf("handler extraction: %w", err)
	}
	return manifest, nil
}
