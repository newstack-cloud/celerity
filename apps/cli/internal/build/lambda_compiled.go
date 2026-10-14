package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/newstack-cloud/celerity/apps/cli/internal/handlertoolchain"
	"go.uber.org/zap"
)

const (
	// compiledBootstrapName is what a custom-runtime Lambda executes. The
	// name is fixed by the platform, not chosen here.
	compiledBootstrapName = "bootstrap"

	// compiledLambdaRuntime is the custom runtime a compiled executable runs
	// on. The blueprint's portable identifier (os-only.debian13) names a base
	// OS, and this is what it maps to for AWS.
	compiledLambdaRuntime = "provided.al2023"

	// compiledLambdaArch is the architecture a compiled handlers executable
	// is built for by default. Graviton costs less per millisecond and Go
	// cross-compiles to it at no toolchain cost, and it matches what the
	// maintenance runner already ships as.
	compiledLambdaArch = "arm64"
)

// Packages a compiled handlers executable for AWS Lambda.
//
// Almost none of the interpreted path applies. Dependencies are linked into the
// executable, the archive carries a binary rather than the source it was built
// from, and a custom runtime executes `bootstrap` and dispatches on an
// environment variable instead of calling a named function.
//
// One binary serves every handler. Each function gets the same archive and a
// different CELERITY_HANDLER_ID, which the runtime's handler tag is built
// from.
func (s *LambdaStrategy) buildCompiled(
	ctx context.Context,
	params *BuildParams,
	manifest *Manifest,
) error {
	toolchain := params.Toolchain
	arch := compiledLambdaArch

	params.Logger.Info("building the handlers executable",
		zap.String("toolchain", toolchain.Name()),
		zap.String("arch", arch),
	)

	stagingDir, err := os.MkdirTemp("", "celerity-handlers-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stagingDir)

	bootstrap := filepath.Join(stagingDir, compiledBootstrapName)
	err = toolchain.BuildBinary(ctx, handlertoolchain.BinaryParams{
		AppDir:       params.AppDir,
		OS:           "linux",
		Arch:         arch,
		OutputPath:   bootstrap,
		DeployTarget: params.DeployTarget,
		Mode:         handlertoolchain.ModeDeploy,
	})
	if err != nil {
		return fmt.Errorf("building the handlers executable: %w", err)
	}

	if _, err := os.Stat(bootstrap); err != nil {
		return fmt.Errorf(
			"the %s toolchain reported success but left no executable at %s: %w",
			toolchain.Name(), compiledBootstrapName, err,
		)
	}

	// Checked here as well as in the toolchain, because a custom build command
	// is free to produce whatever it likes and this is the last point before
	// the archive is sealed.
	if err := handlertoolchain.CheckArchitecture(bootstrap, arch); err != nil {
		return err
	}

	if err := writeResourceLinksFile(stagingDir, params.Blueprint); err != nil {
		return err
	}

	appCode, err := writeArtifact(ArtifactTypeLambdaArchive, stagingDir, params.OutputDir, "handlers")
	if err != nil {
		return err
	}

	manifest.Lambda.AppCode = appCode
	manifest.Lambda.EntryPoint = compiledBootstrapName
	manifest.Lambda.Architecture = arch
	manifest.Lambda.LambdaRuntime = compiledLambdaRuntime

	params.Logger.Info("packaged the handlers executable",
		zap.String("artifact", appCode.LocalPath),
		zap.Int64("size", appCode.Size),
	)
	return nil
}

// The whole of Build for a compiled language. The maintenance runner and the
// upload are shared with the interpreted path, since neither has anything to do
// with how the application itself was built.
func (s *LambdaStrategy) buildCompiledManifest(
	ctx context.Context,
	params *BuildParams,
) (*Manifest, error) {
	manifest := &Manifest{
		Version:  ManifestVersion,
		Runtime:  params.Runtime,
		Target:   params.DeployTarget,
		Handlers: map[string]*HandlerArtifacts{},
		Lambda:   &LambdaManifest{},
	}

	if err := s.buildCompiled(ctx, params, manifest); err != nil {
		return nil, err
	}

	runner, err := s.buildMaintenanceRunner(params)
	if err != nil {
		return nil, fmt.Errorf("building the database maintenance runner: %w", err)
	}
	manifest.Lambda.MaintenanceRunner = runner

	if params.Uploader != nil {
		if err := uploadAllArtifacts(ctx, params, manifest); err != nil {
			return nil, fmt.Errorf("uploading artifacts: %w", err)
		}
	}

	return manifest, nil
}
