package deployconfig

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/newstack-cloud/celerity/apps/cli/internal/blueprint"
	"github.com/newstack-cloud/celerity/apps/cli/internal/build"
	"github.com/newstack-cloud/celerity/apps/cli/internal/handlerresolve"
	"github.com/newstack-cloud/celerity/apps/cli/internal/handlertoolchain"
	"github.com/newstack-cloud/celerity/apps/cli/internal/preprocess"
	"github.com/newstack-cloud/deploy-cli-sdk/config"
	"github.com/newstack-cloud/deploy-cli-sdk/tui/precommand"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// CelerityPreCommandStep reads the app deploy config, converts it to
// Bluelink format, optionally runs a build, and sets the deployConfigFile
// config key so the SDK commands use it.
type CelerityPreCommandStep struct{}

func (s *CelerityPreCommandStep) Run(
	ctx context.Context,
	confProvider *config.Provider,
	commandName string,
	progress chan<- precommand.ProgressMsg,
) error {
	appConfigFile, _ := confProvider.GetString("appDeployConfigFile")
	if appConfigFile == "" {
		appConfigFile = "app.deploy.jsonc"
	}

	if _, err := os.Stat(appConfigFile); os.IsNotExist(err) {
		return nil
	}

	sendProgress(progress, "Reading app deploy configuration", appConfigFile)

	appCfg, err := ReadAppDeployConfig(appConfigFile)
	if err != nil {
		return fmt.Errorf("reading app deploy config: %w", err)
	}

	if commandName == "deploy" || commandName == "stage" {
		logger := newPreCommandLogger()

		resolved, err := collectHandlersForBuild(ctx, confProvider, appCfg, logger)
		if err != nil {
			return fmt.Errorf("collecting build info: %w", err)
		}

		skipBuild, _ := confProvider.GetBool("skipBuild")
		if !skipBuild {
			if err := autoBuild(ctx, confProvider, appCfg, resolved, logger, progress); err != nil {
				return err
			}
		}

		seeded, err := seedConfigValues(resolved.MergedBlueprint, ".", appCfg.DeployTarget.AppEnv)
		if err != nil {
			return err
		}
		if seeded > 0 {
			sendProgress(
				progress,
				"Seeding config values",
				fmt.Sprintf("%d value(s) from %s", seeded, appCfg.DeployTarget.AppEnv),
			)
		}

		if err := useMergedBlueprint(confProvider, resolved, progress); err != nil {
			return err
		}

		if err := ensureManifestUploaded(ctx, confProvider, appCfg, progress); err != nil {
			return err
		}
	}

	if err := injectBuildManifestPointer(appCfg, confProvider); err != nil {
		return err
	}

	sendProgress(progress, "Converting deploy configuration", "")

	blCfg, err := ConvertToBluelinkFormat(appCfg)
	if err != nil {
		return fmt.Errorf("converting app deploy config: %w", err)
	}

	outputDir := ".celerity"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating .celerity directory: %w", err)
	}

	outputPath := filepath.Join(outputDir, "deploy-config.json")
	if err := WriteBluelinkConfig(outputPath, blCfg); err != nil {
		return fmt.Errorf("writing converted deploy config: %w", err)
	}

	// Set rather than SetDefault, for the same reason as the blueprint keys
	// below. --deploy-config-file carries its own default, which a SetDefault
	// loses to, leaving the SDK commands reading a file this step never wrote.
	// A path the user named explicitly still wins, because an explicitly-set
	// flag beats an override.
	confProvider.Set("deployConfigFile", outputPath)
	return nil
}

func autoBuild(
	ctx context.Context,
	confProvider *config.Provider,
	appCfg *AppDeployConfig,
	resolved *handlerresolve.ResolvedHandlers,
	logger *zap.Logger,
	progress chan<- precommand.ProgressMsg,
) error {
	// An existing manifest is not grounds for skipping the build. The caller
	// has already checked --skip-build, which is the deliberate opt-out, and
	// skipping here as well silently deploys whatever was built last, such as
	// an old SDK version in the dependency layer or source from before the last
	// edit, with the deploy reporting success while running code nobody asked
	// for. Artefacts are content-addressed, so an unchanged application
	// rebuilds to the same hashes and uploads nothing new.

	deployTarget := appCfg.DeployTarget.Name
	sendProgress(progress, "Building application", deployTarget)

	const buildLogPath = ".celerity/build.log"
	if err := os.MkdirAll(".celerity", 0755); err != nil {
		return fmt.Errorf("creating .celerity directory: %w", err)
	}

	logFile, err := os.Create(buildLogPath)
	if err != nil {
		return fmt.Errorf("creating build log: %w", err)
	}
	defer logFile.Close()

	runtime := resolved.Runtime
	handlers := make([]build.HandlerInfo, len(resolved.Handlers))
	for i, h := range resolved.Handlers {
		handlers[i] = build.HandlerInfo{
			ResourceName: h.ResourceName,
			HandlerName:  h.HandlerName,
			CodeLocation: h.CodeLocation,
		}
	}

	registry := build.NewDefaultRegistry(".")
	strategy, err := registry.GetStrategy(deployTarget)
	if err != nil {
		return fmt.Errorf("no build strategy for target %s: %w", deployTarget, err)
	}

	sendProgress(progress, "Resolving handler dependencies", "")

	handlerDeps, err := build.ReadHandlerDependencies(registry, runtime, ".")
	if err != nil {
		return fmt.Errorf("reading handler dependencies: %w", err)
	}

	uploader := build.ResolveUploader(deployTarget, appCfg.DeployTarget.Config)

	sendProgress(progress, "Packaging handlers", fmt.Sprintf("%d handler(s)", len(handlers)))

	manifest, err := strategy.Build(ctx, &build.BuildParams{
		AppDir:              ".",
		Runtime:             runtime,
		DeployTarget:        deployTarget,
		Handlers:            handlers,
		HandlerDependencies: handlerDeps,
		OutputDir:           ".celerity/build",
		Uploader:            uploader,
		Logger:              logger,
		// Without this the build has no blueprint at all, so it bundles an
		// empty resource links file and an empty handler scope file and skips
		// the maintenance runner, all of it silently, because each of those
		// treats a missing blueprint as nothing to do. Every deploy goes
		// through here, so this is the path that matters most.
		Blueprint:  resolved.MergedBlueprint,
		ToolOutput: logFile,
		Toolchain:  resolved.Toolchain,
	})
	if err != nil {
		sendProgress(progress, "Build failed", "see "+buildLogPath)
		return fmt.Errorf("auto-build failed (see %s): %w", buildLogPath, err)
	}

	if err := build.WriteManifest(manifest); err != nil {
		return fmt.Errorf("writing build manifest: %w", err)
	}

	sendProgress(progress, "Build complete", "log: "+buildLogPath)
	return nil
}

// Uploads the local build manifest to cloud storage when the CLI is about to
// contact a remote deploy engine and the manifest has not already been
// uploaded. The resulting s3:// URL is written back to the local manifest so
// subsequent deploys can skip the upload.
//
// This is the single choke point that decouples the standalone build command
// from deploy engine configuration. Build writes a local manifest, and any
// deploy to a remote engine lifts it to cloud storage on the way out.
func ensureManifestUploaded(
	ctx context.Context,
	confProvider *config.Provider,
	appCfg *AppDeployConfig,
	progress chan<- precommand.ProgressMsg,
) error {
	if !IsRemoteEngine(confProvider) {
		return nil
	}

	manifest, err := build.ReadManifest()
	if err != nil {
		return fmt.Errorf("reading build manifest: %w", err)
	}
	if manifest == nil {
		return nil
	}
	if manifest.RemoteURL != "" {
		return nil
	}

	uploader := build.ResolveUploader(
		appCfg.DeployTarget.Name,
		appCfg.DeployTarget.Config,
	)
	if uploader == nil {
		return fmt.Errorf(
			"remote deploy engine configured but deploy target %q "+
				"does not support build manifest upload",
			appCfg.DeployTarget.Name,
		)
	}

	sendProgress(progress, "Uploading build manifest", "")

	if err := build.UploadManifest(ctx, uploader, manifest, "default"); err != nil {
		sendProgress(progress, "Manifest upload failed", err.Error())
		return err
	}

	if err := build.WriteManifest(manifest); err != nil {
		return fmt.Errorf("writing build manifest after upload: %w", err)
	}

	sendProgress(progress, "Build manifest uploaded", manifest.RemoteURL)
	return nil
}

func sendProgress(ch chan<- precommand.ProgressMsg, phase, detail string) {
	if ch == nil {
		return
	}
	ch <- precommand.ProgressMsg{Phase: phase, Detail: detail}
}

// For a local deploy engine the pointer is an absolute file path, and for a
// remote one the cloud storage URL recorded in manifest.RemoteURL by
// ensureManifestUploaded.
func injectBuildManifestPointer(appCfg *AppDeployConfig, confProvider *config.Provider) error {
	manifest, _ := build.ReadManifest()
	if manifest == nil {
		return nil
	}

	if appCfg.ContextVariables == nil {
		appCfg.ContextVariables = map[string]any{}
	}

	if IsRemoteEngine(confProvider) {
		if manifest.RemoteURL == "" {
			return fmt.Errorf(
				"remote deploy engine requires an uploaded build manifest, " +
					"but none is recorded; ensure the deploy target supports artifact upload",
			)
		}
		appCfg.ContextVariables["celerity.buildManifest"] = manifest.RemoteURL
		return nil
	}

	absPath, err := filepath.Abs(build.ManifestFile)
	if err != nil {
		absPath = build.ManifestFile
	}
	appCfg.ContextVariables["celerity.buildManifest"] = absPath
	return nil
}

// Extracts the handlers defined in code and merges them into the blueprint,
// producing what both the build and the deploy engine work from.
//
// Run for stage and deploy whether or not a build happens, because the merge is
// what puts handler resources in the blueprint and skipping it on the grounds
// that artifacts already exist would deploy an application with no handlers in
// it.
func collectHandlersForBuild(
	ctx context.Context,
	confProvider *config.Provider,
	appCfg *AppDeployConfig,
	logger *zap.Logger,
) (*handlerresolve.ResolvedHandlers, error) {
	blueprintFile := deployBlueprintFile(confProvider)

	// Resolved once here and carried on the result, so the build step uses
	// the toolchain the extraction step already prepared the project with
	// rather than resolving a second one.
	toolchain, err := resolveToolchain(appCfg, blueprintFile, logger)
	if err != nil {
		return nil, err
	}

	resolved, err := handlerresolve.Resolve(ctx, handlerresolve.Options{
		AppDir:        ".",
		BlueprintFile: blueprintFile,
		Toolchain:     toolchain,
		DeployTarget:  appCfg.DeployTarget.Name,
		Mode:          handlertoolchain.ModeDeploy,
	}, logger)
	if err != nil {
		return nil, err
	}

	if len(resolved.Handlers) > 0 {
		if err := handlerresolve.RequireRuntimeSDK(".", resolved); err != nil {
			return nil, err
		}
	}

	return resolved, nil
}

// Resolves the build tooling for an OS-only runtime.
//
// The runtime is read from the blueprint rather than taken from the resolved
// handlers, because resolution is what needs the toolchain. Extraction for a
// compiled language is the toolchain's own job.
func resolveToolchain(
	appCfg *AppDeployConfig,
	blueprintFile string,
	logger *zap.Logger,
) (handlertoolchain.Toolchain, error) {
	runtime, err := runtimeForToolchain(blueprintFile)
	if err != nil || runtime == "" {
		// Not an error worth stopping for, since a blueprint the runtime
		// cannot be read from is reported with far better context by Resolve,
		// which runs next.
		return nil, nil
	}

	toolchain, err := handlertoolchain.Resolve(handlertoolchain.Options{
		Runtime: runtime,
		AppDir:  ".",
		Config:  appCfg.HandlerBuild,
	})
	if err != nil {
		return nil, err
	}
	if toolchain != nil {
		logger.Info("resolved handler toolchain",
			zap.String("toolchain", toolchain.Name()),
			zap.String("runtime", runtime),
		)
	}
	return toolchain, nil
}

func runtimeForToolchain(blueprintFile string) (string, error) {
	bp, _, err := blueprint.LoadForDev(blueprintFile)
	if err != nil {
		return "", err
	}
	if runtime, err := blueprint.DetectRuntime(bp); err == nil {
		return runtime, nil
	}
	return blueprint.DetectRuntimeFromProject(".")
}

// The blueprint the command was pointed at, before any merged copy replaces it.
//
// stage and deploy each bind their own key, and only the running command's is
// set, so both are consulted.
func deployBlueprintFile(confProvider *config.Provider) string {
	for _, key := range []string{"deployBlueprintFile", "stageBlueprintFile"} {
		if value, _ := confProvider.GetString(key); value != "" {
			return value
		}
	}

	return "app.blueprint"
}

// Writes the merged blueprint out and points the command at it.
//
// A decorator-driven application declares no celerity/handler resources in its
// blueprint, so without this the transformer has nothing to expand and the
// deployed application has no handlers, silently.
func useMergedBlueprint(
	confProvider *config.Provider,
	resolved *handlerresolve.ResolvedHandlers,
	progress chan<- precommand.ProgressMsg,
) error {
	if resolved.MergedBlueprint == nil {
		return nil
	}

	sendProgress(
		progress,
		"Merging handlers into blueprint",
		fmt.Sprintf("%d handler(s)", len(resolved.Handlers)),
	)

	// 0600: the merged blueprint carries the environment's configuration values,
	// including secrets seeded so a first deploy can stand its store up.
	mergedPath, err := preprocess.WriteMergedWithMode(
		resolved.MergedBlueprint,
		resolved.SpecFormat,
		".celerity",
		0o600,
	)
	if err != nil {
		return fmt.Errorf("writing merged blueprint: %w", err)
	}

	// Set rather than SetDefault, because a default loses to the blueprint
	// flag's own default. A blueprint the user named explicitly still wins over
	// both.
	confProvider.Set("deployBlueprintFile", mergedPath)
	confProvider.Set("stageBlueprintFile", mergedPath)

	return nil
}

func newPreCommandLogger() *zap.Logger {
	cfg := zap.NewDevelopmentConfig()
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.OutputPaths = []string{"stdout"}
	cfg.ErrorOutputPaths = []string{"stderr"}
	logger, _ := cfg.Build()
	return logger
}
