package commands

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/newstack-cloud/celerity/apps/cli/internal/blueprint"
	"github.com/newstack-cloud/celerity/apps/cli/internal/build"
	"github.com/newstack-cloud/celerity/apps/cli/internal/deployconfig"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devconfig/resolve"
	"github.com/newstack-cloud/celerity/apps/cli/internal/handlerresolve"
	"github.com/newstack-cloud/celerity/apps/cli/internal/handlertoolchain"
	"github.com/newstack-cloud/deploy-cli-sdk/config"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func setupBuildCommand(rootCmd *cobra.Command, confProvider *config.Provider) {
	buildCmd := &cobra.Command{
		Use:   "build",
		Short: "Build application artifacts for deployment",
		Long: `Packages the application for the configured deploy target.

For serverless targets (e.g. aws-serverless), this creates per-handler
deployment archives and a shared dependency layer. Developers can opt-in
to per-handler dependency layers by declaring handlerDependencies in their
package manifest (package.json or pyproject.toml).

The build output is recorded in .celerity/build-manifest.json, which is
referenced by the deploy and stage commands.

Examples:
  # Build using default app.deploy.jsonc
  celerity build

  # Build for a specific environment
  celerity build --app-deploy-config-file app.deploy.staging.jsonc`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuild(cmd.Context(), confProvider, os.Stdout)
		},
	}

	buildCmd.Flags().String(
		"blueprint-file", "",
		"The blueprint file to read handler and runtime information from "+
			"(default: auto-detect).",
	)
	confProvider.BindPFlag("buildBlueprintFile", buildCmd.Flags().Lookup("blueprint-file"))
	confProvider.BindEnvVar("buildBlueprintFile", "CELERITY_CLI_BUILD_BLUEPRINT_FILE")

	rootCmd.AddCommand(buildCmd)
}

// defaultBuildInstanceName is the instance a build is attributed to when the
// command is run on its own rather than as part of a deploy.
const defaultBuildInstanceName = "default"

func runBuild(ctx context.Context, confProvider *config.Provider, w io.Writer) error {
	logger := newBuildLogger()

	appConfigFile, _ := confProvider.GetString("appDeployConfigFile")
	if appConfigFile == "" {
		appConfigFile = "app.deploy.jsonc"
	}

	appCfg, err := deployconfig.ReadAppDeployConfig(appConfigFile)
	if err != nil {
		return fmt.Errorf("reading deploy config: %w", err)
	}

	deployTarget := appCfg.DeployTarget.Name
	fmt.Fprintf(w, "Build target: %s\n", deployTarget)

	blueprintFile, _ := confProvider.GetString("buildBlueprintFile")
	if blueprintFile == "" {
		blueprintFile, err = resolve.BlueprintPath(".", "")
		if err != nil {
			return err
		}
	}

	info, err := collectBuildInfo(ctx, ".", blueprintFile, deployTarget, appCfg.HandlerBuild, logger)
	if err != nil {
		return err
	}
	runtime := info.Runtime
	fmt.Fprintf(w, "Runtime: %s\n", runtime)
	fmt.Fprintf(w, "Handlers: %d\n\n", len(info.Handlers))

	registry := build.NewDefaultRegistry(".")
	strategy, err := registry.GetStrategy(deployTarget)
	if err != nil {
		return err
	}

	handlerDeps, err := build.ReadHandlerDependencies(registry, runtime, ".")
	if err != nil {
		return fmt.Errorf("reading handler dependencies: %w", err)
	}

	if len(handlerDeps) > 0 {
		fmt.Fprintf(w, "Per-handler dependencies declared for %d handler(s)\n", len(handlerDeps))
	}

	uploader := build.ResolveUploader(deployTarget, appCfg.DeployTarget.Config)

	fmt.Fprintf(w, "Building...\n")

	manifest, err := strategy.Build(ctx, &build.BuildParams{
		AppDir:              ".",
		Runtime:             runtime,
		DeployTarget:        deployTarget,
		Handlers:            info.Handlers,
		HandlerDependencies: handlerDeps,
		OutputDir:           ".celerity/build",
		Uploader:            uploader,
		Logger:              logger,
		Blueprint:           info.Blueprint,
		ToolOutput:          w,
		Toolchain:           info.Toolchain,
	})
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Uploaded before the manifest is written, so the copy on disk records the
	// remote URL. A nil uploader makes this a no-op, which is every deploy
	// target that does not stage artefacts remotely.
	//
	// The instance name matches the one the deploy path uses, so a manifest
	// pre-built in CI lands at the key a later deploy reads from.
	if err := build.UploadManifest(ctx, uploader, manifest, defaultBuildInstanceName); err != nil {
		return fmt.Errorf("uploading build manifest: %w", err)
	}

	if err := build.WriteManifest(manifest); err != nil {
		return err
	}

	fmt.Fprintf(w, "\nBuild complete. %d handler(s) packaged.\n", len(manifest.Handlers))
	fmt.Fprintf(w, "Manifest: %s\n", build.ManifestFile)

	printLambdaManifest(w, manifest)

	return nil
}

func printLambdaManifest(w io.Writer, manifest *build.Manifest) {
	if manifest.Lambda == nil {
		return
	}
	if manifest.Lambda.AppCode != nil {
		fmt.Fprintf(w, "App code: %s (%d KB)\n",
			manifest.Lambda.AppCode.LocalPath, manifest.Lambda.AppCode.Size/1024)
	}
	if manifest.Lambda.EntryPoint != "" {
		fmt.Fprintf(w, "Entry point: %s\n", manifest.Lambda.EntryPoint)
	}
	if manifest.Lambda.SharedLayer != nil {
		fmt.Fprintf(w, "Shared layer: %s (%d KB)\n",
			manifest.Lambda.SharedLayer.LocalPath, manifest.Lambda.SharedLayer.Size/1024)
	}

	for name, arts := range manifest.Handlers {
		if arts.Lambda != nil && arts.Lambda.Dependencies != nil {
			fmt.Fprintf(w, "  %s: layer=%d KB\n", name, arts.Lambda.Dependencies.Size/1024)
		} else {
			fmt.Fprintf(w, "  %s: uses shared layer\n", name)
		}
	}
}

// buildInfo is what a build needs to read off the blueprint and the project
// before it can package anything.
type buildInfo struct {
	Runtime   string
	Handlers  []build.HandlerInfo
	Blueprint *schema.Blueprint
	Toolchain handlertoolchain.Toolchain
}

func collectBuildInfo(
	ctx context.Context,
	appDir, blueprintFile, deployTarget string,
	handlerBuild *handlertoolchain.Config,
	logger *zap.Logger,
) (*buildInfo, error) {
	runtime, err := buildRuntime(blueprintFile, appDir)
	if err != nil {
		return nil, err
	}

	toolchain, err := handlertoolchain.Resolve(handlertoolchain.Options{
		Runtime: runtime,
		AppDir:  appDir,
		Config:  handlerBuild,
	})
	if err != nil {
		return nil, err
	}

	resolved, err := handlerresolve.Resolve(ctx, handlerresolve.Options{
		AppDir:        appDir,
		BlueprintFile: blueprintFile,
		Toolchain:     toolchain,
		DeployTarget:  deployTarget,
		Mode:          handlertoolchain.ModeDeploy,
	}, logger)
	if err != nil {
		return nil, err
	}

	if len(resolved.Handlers) > 0 {
		if err := handlerresolve.RequireRuntimeSDK(appDir, resolved); err != nil {
			return nil, err
		}
	}

	handlers := make([]build.HandlerInfo, len(resolved.Handlers))
	for i, h := range resolved.Handlers {
		handlers[i] = build.HandlerInfo{
			ResourceName: h.ResourceName,
			HandlerName:  h.HandlerName,
			CodeLocation: h.CodeLocation,
		}
	}
	return &buildInfo{
		Runtime:   resolved.Runtime,
		Handlers:  handlers,
		Blueprint: resolved.MergedBlueprint,
		Toolchain: resolved.Toolchain,
	}, nil
}

// buildRuntime reads the runtime identifier the toolchain is resolved against,
// before handler resolution has run and can report it.
func buildRuntime(blueprintFile, appDir string) (string, error) {
	bp, _, err := blueprint.LoadForDev(blueprintFile)
	if err != nil {
		return "", err
	}
	if runtime, err := blueprint.DetectRuntime(bp); err == nil {
		return runtime, nil
	}
	return blueprint.DetectRuntimeFromProject(appDir)
}

func newBuildLogger() *zap.Logger {
	cfg := zap.NewDevelopmentConfig()
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.OutputPaths = []string{"stderr"}
	cfg.ErrorOutputPaths = []string{"stderr"}
	cfg.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	logger, _ := cfg.Build()
	return logger
}
