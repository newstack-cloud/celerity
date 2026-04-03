package devconfig

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/newstack-cloud/celerity/apps/cli/internal/blueprint"
	"github.com/newstack-cloud/celerity/apps/cli/internal/compose"
	"github.com/newstack-cloud/celerity/apps/cli/internal/consts"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devconfig/resolve"
	"github.com/newstack-cloud/celerity/apps/cli/internal/docker"
	"github.com/newstack-cloud/celerity/apps/cli/internal/preprocess"
	"github.com/newstack-cloud/celerity/apps/cli/internal/seed"
	"go.uber.org/zap"
)

// DefaultRuntimeImageVersion is used when the SDK version cannot be
// detected from the project's dependency file.
const DefaultRuntimeImageVersion = "0.8.1"

// ResolveOpts holds the CLI flag values for config resolution.
type ResolveOpts struct {
	BlueprintFile string
	DeployConfig  string
	Port          string
	AppDir        string
	ModulePath    string
	Image         string
	ServiceName   string
	Mode          string // "run" or "test"
	Verbose       bool
	LocalAuth     bool // true (default) = patch issuer to dev auth sidecar; false = --no-local-auth
}

// ResolvedConfig is the fully resolved configuration for a dev run.
type ResolvedConfig struct {
	Blueprint           *schema.Blueprint
	SpecFormat          schema.SpecFormat
	MergedBlueprintPath string
	DeployTarget        string
	RuntimeImage        string
	Runtime             string
	HandlerInfos        []blueprint.HandlerInfo
	Manifest            *preprocess.HandlerManifest
	ContainerConfig     *docker.ContainerConfig
	ComposeConfig       *compose.ComposeConfig
	SeedDir             string
	ConfigDir           string
	SecretsDir          string
	Mode                string
	AppDir              string
	ModulePath          string
	Port                string
	ServiceName         string
}

// Resolve assembles the complete configuration by loading the blueprint,
// extracting handlers, merging, resolving the image, and generating compose config.
func Resolve(ctx context.Context, opts ResolveOpts, logger *zap.Logger) (*ResolvedConfig, error) {
	appDir, err := resolveAppDir(opts.AppDir)
	if err != nil {
		return nil, err
	}

	bpPath, err := resolveBlueprintPath(appDir, opts.BlueprintFile)
	if err != nil {
		return nil, err
	}

	bp, specFormat, err := blueprint.LoadForDev(bpPath)
	if err != nil {
		return nil, err
	}

	deployTarget, err := resolveDeployTarget(appDir, opts.DeployConfig)
	if err != nil {
		return nil, err
	}

	runtime, err := blueprint.DetectRuntime(bp)
	if err != nil {
		// Detect from project files for decorator-driven projects
		// where the blueprint has no handler resources yet.
		runtime, err = blueprint.DetectRuntimeFromProject(appDir)
		if err != nil {
			return nil, err
		}
		logger.Info("runtime auto-detected from project files",
			zap.String("runtime", runtime),
		)
	}

	modulePath := resolveModulePath(appDir, opts.ModulePath, runtime)

	conv, _ := consts.ConventionsForRuntime(runtime)
	var manifest *preprocess.HandlerManifest
	if conv.SupportsExtraction {
		extractor := preprocess.NewExtractor(preprocess.ExtractorConfig{
			Runtime:     runtime,
			ModulePath:  modulePath,
			ProjectRoot: appDir,
		}, logger)
		manifest, err = extractor.Extract(ctx)
		if err != nil {
			return nil, fmt.Errorf("handler extraction: %w", err)
		}
	} else {
		logger.Info("handler extraction not available for runtime, using raw blueprint",
			zap.String("runtime", runtime),
		)
		manifest = &preprocess.HandlerManifest{Version: "1"}
	}

	merged, err := preprocess.Merge(bp, manifest, logger)
	if err != nil {
		return nil, fmt.Errorf("blueprint merge: %w", err)
	}

	// Patch JWT issuer to point to the local dev auth sidecar when localAuth
	// is enabled. This must happen before WriteMerged so the runtime reads
	// the patched issuer for OIDC discovery.
	if opts.LocalAuth {
		portOffset := 0
		if opts.Mode == "test" {
			portOffset = 100
		}
		PatchJWTIssuer(merged, portOffset)
	}

	outputDir := filepath.Join(appDir, ".celerity")
	mergedPath, err := preprocess.WriteMerged(merged, specFormat, outputDir)
	if err != nil {
		return nil, fmt.Errorf("writing merged blueprint: %w", err)
	}

	runtimeImage := opts.Image
	if runtimeImage == "" {
		imageVersion := DefaultRuntimeImageVersion
		if detected, detectErr := blueprint.DetectSDKVersion(appDir, runtime); detectErr == nil {
			imageVersion = detected
		} else {
			logger.Debug("SDK version detection failed, using default",
				zap.String("default", DefaultRuntimeImageVersion),
				zap.Error(detectErr),
			)
		}
		runtimeImage, err = blueprint.ResolveRuntimeImage(runtime, imageVersion)
		if err != nil {
			return nil, err
		}
	}

	handlerInfos := blueprint.CollectHandlerInfo(merged)

	serviceName := opts.ServiceName
	if serviceName == "" {
		serviceName = filepath.Base(appDir)
	}

	mode := opts.Mode
	if mode == "" {
		mode = "run"
	}

	port := opts.Port
	if port == "" {
		port = "8080"
	}

	composeProjectPrefix := "celerity-dev-"
	portOffset := 0
	if mode == "test" {
		composeProjectPrefix = "celerity-test-"
		portOffset = 100
	}

	composeCfg, err := compose.GenerateComposeConfig(
		merged, deployTarget, composeProjectPrefix+serviceName, appDir, portOffset, opts.LocalAuth, logger,
	)
	if err != nil {
		return nil, fmt.Errorf("generating compose config: %w", err)
	}

	containerCfg := buildContainerConfig(
		runtimeImage, serviceName, port, appDir, mergedPath,
		modulePath, composeCfg, merged, deployTarget, opts.Verbose,
	)

	return &ResolvedConfig{
		Blueprint:           merged,
		SpecFormat:          specFormat,
		MergedBlueprintPath: mergedPath,
		DeployTarget:        deployTarget,
		RuntimeImage:        runtimeImage,
		Runtime:             runtime,
		HandlerInfos:        handlerInfos,
		Manifest:            manifest,
		ContainerConfig:     containerCfg,
		ComposeConfig:       composeCfg,
		SeedDir:             resolveSeedDir(appDir, mode),
		ConfigDir:           resolveConfigDir(appDir, mode),
		SecretsDir:          resolveSecretsDir(appDir, mode),
		Mode:                mode,
		AppDir:              appDir,
		ModulePath:          modulePath,
		Port:                port,
		ServiceName:         serviceName,
	}, nil
}

func resolveAppDir(appDir string) (string, error) {
	return resolve.AppDir(appDir)
}

func resolveBlueprintPath(appDir string, flagValue string) (string, error) {
	return resolve.BlueprintPath(appDir, flagValue)
}

func resolveDeployTarget(appDir string, flagValue string) (string, error) {
	path := flagValue
	if path == "" {
		path = FindDeployConfig(appDir)
	}
	if path == "" {
		return "", fmt.Errorf(
			"no deploy config found in %s (expected app.deploy.jsonc); "+
				"create one with a deployTarget.name field or use --deploy-config",
			appDir,
		)
	}
	return ReadDeployTarget(path)
}

func resolveModulePath(appDir string, flagValue string, runtime string) string {
	conv, ok := consts.ConventionsForRuntime(runtime)
	if !ok {
		if flagValue != "" {
			return flagValue
		}
		return ""
	}
	return resolve.ModulePath(appDir, flagValue, conv.DefaultModulePaths)
}

func resolveSeedDir(appDir string, mode string) string {
	return resolve.DirWithTestFallback(appDir, "seed", mode)
}

func resolveConfigDir(appDir string, mode string) string {
	return resolve.DirWithTestFallback(appDir, "config", mode)
}

func resolveSecretsDir(appDir string, mode string) string {
	return resolve.DirWithTestFallback(appDir, "secrets", mode)
}

func deployTargetToProvider(deployTarget string) string {
	return resolve.DeployTargetToProvider(deployTarget)
}

func buildContainerConfig(
	image string,
	serviceName string,
	port string,
	appDir string,
	mergedBlueprintPath string,
	modulePath string,
	composeCfg *compose.ComposeConfig,
	bp *schema.Blueprint,
	deployTarget string,
	verbose bool,
) *docker.ContainerConfig {
	envVars := map[string]string{
		"CELERITY_BLUEPRINT":          "/opt/celerity/merged.blueprint.yaml",
		"CELERITY_MODULE_PATH":        "app/" + modulePath,
		"CELERITY_SERVICE_NAME":       serviceName,
		"CELERITY_RUNTIME_PLATFORM":   "local",
		"CELERITY_DEPLOY_TARGET":      deployTargetToProvider(deployTarget),
		"CELERITY_PLATFORM":           "local",
		"CELERITY_RUNTIME":            "true",
		"CELERITY_LOG_FORMAT":         "json",
		"CELERITY_CONFIG_VALKEY_HOST": "valkey",
	}

	if verbose {
		envVars["CELERITY_MAX_DIAGNOSTICS_LEVEL"] = "debug"
		envVars["RUST_LOG"] = "debug"
		envVars["DEBUG"] = "celerity:*"
	}

	// Add compose-generated runtime env vars (endpoints for local services).
	if composeCfg != nil {
		maps.Copy(envVars, composeCfg.RuntimeEnvVars)
	}

	// Add config store ID env vars so the runtime knows where to
	// find each config namespace in Valkey.
	maps.Copy(envVars, seed.ConfigStoreIDEnvVars(bp))

	// Add "resources" config namespace env vars so the SDK's resource layers
	// (datastore, bucket, queue, topic, cache, sql-database) can discover
	// connection credentials.
	maps.Copy(envVars, seed.ResourcesConfigStoreEnvVars())

	// Add resource links JSON so the SDK knows which resources are available.
	if linksJSON, err := seed.ResourceLinksJSON(bp); err == nil && linksJSON != "" {
		envVars["CELERITY_RESOURCE_LINKS"] = linksJSON
	}

	binds := []string{
		appDir + ":/opt/celerity/app",
		mergedBlueprintPath + ":/opt/celerity/merged.blueprint.yaml:ro",
	}

	networkName := ""
	if composeCfg != nil && composeCfg.ProjectName != "" {
		networkName = composeCfg.ProjectName + "_default"
	}

	return &docker.ContainerConfig{
		Image:         image,
		ContainerName: "celerity-dev-" + serviceName,
		Cmd:           []string{"dev"},
		HostPort:      port,
		ContainerPort: "8080",
		AppDir:        appDir,
		EnvVars:       envVars,
		Binds:         binds,
		NetworkName:   networkName,
	}
}
