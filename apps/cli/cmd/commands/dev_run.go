package commands

import (
	"context"
	"os"
	"path/filepath"

	"github.com/newstack-cloud/celerity/apps/cli/internal/compose"
	"github.com/newstack-cloud/celerity/apps/cli/internal/config"
	"github.com/newstack-cloud/celerity/apps/cli/internal/consts"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devconfig"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devrun"
	"github.com/newstack-cloud/celerity/apps/cli/internal/docker"
	"github.com/newstack-cloud/celerity/apps/cli/internal/preprocess"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/term"
)

func setupDevRunCommand(devCmd *cobra.Command, confProvider *config.Provider) {
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Start the local development server",
		Long: `Start a local development server for your Celerity application.

The command starts a Docker container with the Celerity runtime, along with
any dependency services (datastore emulator, object storage, cache) defined
in your blueprint and deploy target.

By default, the command runs in the foreground streaming container logs to stdout.
Use --detached to run in the background.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDevRun(cmd.Context(), confProvider)
		},
	}

	runCmd.Flags().StringP("blueprint-file", "b", "", "Blueprint file path (default: auto-detect)")
	confProvider.BindPFlag("devRunBlueprintFile", runCmd.Flags().Lookup("blueprint-file"))
	confProvider.BindEnvVar("devRunBlueprintFile", "CELERITY_CLI_DEV_RUN_BLUEPRINT_FILE")

	runCmd.Flags().StringP("deploy-config", "d", "", "Deploy config file path (default: auto-detect)")
	confProvider.BindPFlag("devRunDeployConfig", runCmd.Flags().Lookup("deploy-config"))
	confProvider.BindEnvVar("devRunDeployConfig", "CELERITY_CLI_DEV_RUN_DEPLOY_CONFIG")

	runCmd.Flags().StringP("port", "p", "8080", "Host port to expose the runtime on")
	confProvider.BindPFlag("devRunPort", runCmd.Flags().Lookup("port"))
	confProvider.BindEnvVar("devRunPort", "CELERITY_CLI_DEV_RUN_PORT")

	runCmd.Flags().String("app-dir", ".", "Application root directory")
	confProvider.BindPFlag("devRunAppDir", runCmd.Flags().Lookup("app-dir"))
	confProvider.BindEnvVar("devRunAppDir", "CELERITY_CLI_DEV_RUN_APP_DIR")

	runCmd.Flags().String("module-path", "", "Module entry point (auto-detected from blueprint runtime)")
	confProvider.BindPFlag("devRunModulePath", runCmd.Flags().Lookup("module-path"))
	confProvider.BindEnvVar("devRunModulePath", "CELERITY_CLI_DEV_RUN_MODULE_PATH")

	runCmd.Flags().String("image", "", "Override Docker image (skips GHCR lookup)")
	confProvider.BindPFlag("devRunImage", runCmd.Flags().Lookup("image"))
	confProvider.BindEnvVar("devRunImage", "CELERITY_CLI_DEV_RUN_IMAGE")

	runCmd.Flags().String("service-name", "", "Override service name (default: directory name)")
	confProvider.BindPFlag("devRunServiceName", runCmd.Flags().Lookup("service-name"))
	confProvider.BindEnvVar("devRunServiceName", "CELERITY_CLI_DEV_RUN_SERVICE_NAME")

	runCmd.Flags().Bool("detached", false, "Run in the background")
	confProvider.BindPFlag("devRunDetached", runCmd.Flags().Lookup("detached"))
	confProvider.BindEnvVar("devRunDetached", "CELERITY_CLI_DEV_RUN_DETACHED")

	runCmd.Flags().BoolP("verbose", "v", false, "Enable debug logging in the runtime and SDK")
	confProvider.BindPFlag("devRunVerbose", runCmd.Flags().Lookup("verbose"))
	confProvider.BindEnvVar("devRunVerbose", "CELERITY_CLI_DEV_RUN_VERBOSE")

	runCmd.Flags().Bool("no-local-auth", false, "Do not override the blueprint JWT issuer with the local dev auth server")
	confProvider.BindPFlag("devRunNoLocalAuth", runCmd.Flags().Lookup("no-local-auth"))
	confProvider.BindEnvVar("devRunNoLocalAuth", "CELERITY_CLI_DEV_RUN_NO_LOCAL_AUTH")

	devCmd.AddCommand(runCmd)
}

func runDevRun(ctx context.Context, confProvider *config.Provider) error {
	logger, logHandle, err := setupDevLogger()
	if err != nil {
		return err
	}
	defer logHandle.Close()

	isColor := term.IsTerminal(int(os.Stdout.Fd()))
	output := devrun.NewOutput(os.Stdout, isColor)

	opts := resolveOptsFromFlags(confProvider)
	resolved, err := devconfig.Resolve(ctx, opts, logger)
	if err != nil {
		output.PrintError("Config resolution failed", err)
		return err
	}

	dockerMgr, err := docker.NewRuntimeContainer(logger)
	if err != nil {
		return err
	}

	composeMgr, err := compose.NewComposeManager(
		resolved.ComposeConfig.ProjectName, resolved.AppDir, logger,
	)
	if err != nil {
		return err
	}

	if err := devrun.HandleStaleState(ctx, resolved.AppDir, dockerMgr, composeMgr, output); err != nil {
		return err
	}

	var extractor *preprocess.Extractor
	conv, _ := consts.ConventionsForRuntime(resolved.Runtime)
	if conv.SupportsExtraction {
		extractor = preprocess.NewExtractor(preprocess.ExtractorConfig{
			Runtime:     resolved.Runtime,
			ModulePath:  resolved.ModulePath,
			ProjectRoot: resolved.AppDir,
		}, logger)
	} else {
		logger.Info("handler extraction not supported, watcher will only restart on file changes",
			zap.String("runtime", resolved.Runtime),
		)
	}

	orch := devrun.NewOrchestrator(
		devrun.OrchestratorConfig{
			AppDir:              resolved.AppDir,
			Port:                resolved.Port,
			ServiceName:         resolved.ServiceName,
			Image:               resolved.RuntimeImage,
			DeployTarget:        resolved.DeployTarget,
			Runtime:             resolved.Runtime,
			MergedBlueprintPath: resolved.MergedBlueprintPath,
			ModulePath:          resolved.ModulePath,
			SeedDir:             resolved.SeedDir,
			ConfigDir:           resolved.ConfigDir,
			SecretsDir:          resolved.SecretsDir,
			Blueprint:           resolved.Blueprint,
			SpecFormat:          resolved.SpecFormat,
			HandlerInfos:        resolved.HandlerInfos,
			Manifest:            resolved.Manifest,
			ContainerCfg:        resolved.ContainerConfig,
			ComposeCfg:          resolved.ComposeConfig,
		},
		dockerMgr,
		composeMgr,
		extractor,
		output,
		logger,
	)

	detached, _ := confProvider.GetBool("devRunDetached")
	if detached {
		return orch.RunDetached(ctx)
	}

	return orch.RunForeground(ctx)
}

func resolveOptsFromFlags(confProvider *config.Provider) devconfig.ResolveOpts {
	blueprintFile, _ := confProvider.GetString("devRunBlueprintFile")
	deployConfig, _ := confProvider.GetString("devRunDeployConfig")
	port, _ := confProvider.GetString("devRunPort")
	appDir, _ := confProvider.GetString("devRunAppDir")
	modulePath, _ := confProvider.GetString("devRunModulePath")
	image, _ := confProvider.GetString("devRunImage")
	serviceName, _ := confProvider.GetString("devRunServiceName")

	verbose, _ := confProvider.GetBool("devRunVerbose")
	noLocalAuth, _ := confProvider.GetBool("devRunNoLocalAuth")

	return devconfig.ResolveOpts{
		BlueprintFile: blueprintFile,
		DeployConfig:  deployConfig,
		Port:          port,
		AppDir:        appDir,
		ModulePath:    modulePath,
		Image:         image,
		ServiceName:   serviceName,
		Mode:          "run",
		Verbose:       verbose,
		LocalAuth:     !noLocalAuth,
	}
}

func setupDevLogger() (*zap.Logger, *os.File, error) {
	logDir := ".celerity"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, nil, err
	}

	logFile, err := os.OpenFile(
		filepath.Join(logDir, "celerity-dev.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644,
	)
	if err != nil {
		return nil, nil, err
	}

	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{logFile.Name()}
	cfg.ErrorOutputPaths = []string{logFile.Name()}
	logger, err := cfg.Build()
	if err != nil {
		logFile.Close()
		return nil, nil, err
	}

	return logger, logFile, nil
}
