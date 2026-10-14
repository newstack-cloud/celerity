package commands

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/newstack-cloud/celerity/apps/cli/internal/appconfig"
	"github.com/newstack-cloud/celerity/apps/cli/internal/blueprint"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devconfig"
	"github.com/newstack-cloud/celerity/apps/cli/internal/tui/configui"
	"github.com/newstack-cloud/deploy-cli-sdk/config"
	styles "github.com/newstack-cloud/deploy-cli-sdk/styles"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func setupConfigCommand(rootCmd *cobra.Command, confProvider *config.Provider) {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage deployed application config and secrets",
		Long: `Manage runtime configuration and secrets for deployed Celerity applications.

Running without a subcommand opens an interactive session for browsing and
managing config values across environments and stores.

Subcommands are available for direct operations:
  celerity config set <store> <key> [value]
  celerity config get <store> <key>
  celerity config list [store]
  celerity config delete <store> <key>
  celerity config import <store> <file>
  celerity config export <store>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			inTerminal := term.IsTerminal(int(os.Stdout.Fd()))
			if !inTerminal {
				return fmt.Errorf(
					"interactive config management requires a terminal. Use a subcommand " +
						"instead (e.g. 'celerity config list --env staging')",
				)
			}

			appDir, _ := confProvider.GetString("configAppDir")
			blueprintFile, _ := confProvider.GetString("configBlueprintFile")
			celerityStyles := styles.NewStyles(lipgloss.NewRenderer(os.Stdout), styles.NewCelerityPalette())

			app := configui.NewMainModel(configui.MainModelOptions{
				Context:       cmd.Context(),
				AppDir:        appDir,
				BlueprintFile: blueprintFile,
				Styles:        celerityStyles,
			})

			options := []tea.ProgramOption{
				tea.WithContext(cmd.Context()),
				tea.WithAltScreen(),
				tea.WithMouseCellMotion(),
			}
			finalModel, err := tea.NewProgram(app, options...).Run()
			if err != nil {
				return err
			}

			final := finalModel.(configui.MainModel)
			if final.Error != nil {
				return final.Error
			}
			return nil
		},
	}

	configCmd.PersistentFlags().StringP(
		"env", "e", "",
		"Environment name. Loads app.deploy.{env}.jsonc if it exists, "+
			"otherwise falls back to app.deploy.jsonc with {env} as the store name prefix.",
	)
	confProvider.BindPFlag("configEnv", configCmd.PersistentFlags().Lookup("env"))
	confProvider.BindEnvVar("configEnv", "CELERITY_CLI_CONFIG_ENV")

	configCmd.PersistentFlags().String(
		"app-dir", ".",
		"Application root directory.",
	)
	confProvider.BindPFlag("configAppDir", configCmd.PersistentFlags().Lookup("app-dir"))
	confProvider.BindEnvVar("configAppDir", "CELERITY_CLI_CONFIG_APP_DIR")

	configCmd.PersistentFlags().StringP(
		"blueprint-file", "b", "",
		"Blueprint file path. Auto-detected if not specified.",
	)
	confProvider.BindPFlag("configBlueprintFile", configCmd.PersistentFlags().Lookup("blueprint-file"))
	confProvider.BindEnvVar("configBlueprintFile", "CELERITY_CLI_CONFIG_BLUEPRINT_FILE")

	configCmd.PersistentFlags().String(
		"region", "",
		"Cloud provider region override.",
	)
	confProvider.BindPFlag("configRegion", configCmd.PersistentFlags().Lookup("region"))
	confProvider.BindEnvVar("configRegion", "CELERITY_CLI_CONFIG_REGION")

	setupConfigSetCommand(configCmd, confProvider)
	setupConfigGetCommand(configCmd, confProvider)
	setupConfigListCommand(configCmd, confProvider)
	setupConfigDeleteCommand(configCmd, confProvider)
	setupConfigImportCommand(configCmd, confProvider)
	setupConfigExportCommand(configCmd, confProvider)

	rootCmd.AddCommand(configCmd)
}

func resolveEnv(
	confProvider *config.Provider,
	appDir string,
	inTerminal bool,
) (*devconfig.EnvironmentInfo, error) {
	envName, isDefault := confProvider.GetString("configEnv")

	if !isDefault && envName != "" {
		return resolveNamedEnv(appDir, envName)
	}

	if !inTerminal {
		return nil, fmt.Errorf("--env is required in non-interactive mode")
	}

	return nil, fmt.Errorf(
		"--env is required. Specify the target environment " +
			"(e.g. --env staging)",
	)
}

func resolveNamedEnv(
	appDir, envName string,
) (*devconfig.EnvironmentInfo, error) {
	envPath := devconfig.FindEnvDeployConfig(appDir, envName)
	if envPath != "" {
		result, err := devconfig.ReadDeployConfig(envPath)
		if err != nil {
			return nil, fmt.Errorf("reading deploy config for env %q: %w", envName, err)
		}
		info := &devconfig.EnvironmentInfo{
			Name:         envName,
			FilePath:     envPath,
			DeployTarget: result.DeployTarget,
			AppEnv:       result.AppEnv,
		}
		if result.Config != nil {
			info.Region = result.Config["aws.region"]
			info.Profile = result.Config["aws.profile"]
		}
		return info, nil
	}

	basePath := devconfig.FindDeployConfig(appDir)
	if basePath == "" {
		return nil, fmt.Errorf(
			"no deploy config found in %s. Create app.deploy.jsonc or app.deploy.%s.jsonc",
			appDir, envName,
		)
	}

	result, err := devconfig.ReadDeployConfig(basePath)
	if err != nil {
		return nil, fmt.Errorf("reading deploy config: %w", err)
	}

	return &devconfig.EnvironmentInfo{
		Name:         envName,
		FilePath:     basePath,
		DeployTarget: result.DeployTarget,
		AppEnv:       envName,
	}, nil
}

func resolveConfigStore(
	ctx context.Context,
	envInfo *devconfig.EnvironmentInfo,
	storeName string,
	confProvider *config.Provider,
) (appconfig.ConfigStore, *appconfig.StoreInfo, error) {
	appDir, _ := confProvider.GetString("configAppDir")
	bpFile, bpIsDefault := confProvider.GetString("configBlueprintFile")

	if bpIsDefault || bpFile == "" {
		bpFile = appDir + "/app.blueprint.yaml"
	}

	bp, _, err := blueprint.LoadForDev(bpFile)
	if err != nil {
		return nil, nil, fmt.Errorf("loading blueprint: %w", err)
	}

	stores := appconfig.CollectStoreInfo(bp)
	info := appconfig.FindStoreInfo(stores, storeName)
	if info == nil {
		return nil, nil, fmt.Errorf(
			"store %q not found in blueprint. The available stores are %s",
			storeName, storeNames(stores),
		)
	}

	appEnv := envInfo.AppEnv
	if appEnv == "" {
		appEnv = envInfo.Name
	}
	resolved := appconfig.StoreInfo{
		ResourceName:  info.ResourceName,
		StoreName:     appEnv + "-" + info.StoreName,
		StoreKind:     info.StoreKind,
		PlaintextKeys: info.PlaintextKeys,
	}

	regionOverride, regionIsDefault := confProvider.GetString("configRegion")
	region := envInfo.Region
	if !regionIsDefault && regionOverride != "" {
		region = regionOverride
	}

	opts := appconfig.ProviderOpts{
		Region:  region,
		Profile: envInfo.Profile,
	}

	store, err := appconfig.NewConfigStore(ctx, envInfo.DeployTarget, resolved, opts)
	if err != nil {
		return nil, nil, err
	}

	return store, &resolved, nil
}

func storeNames(stores []appconfig.StoreInfo) string {
	if len(stores) == 0 {
		return "(none)"
	}
	names := make([]string, len(stores))
	for i, s := range stores {
		names[i] = s.StoreName
	}
	return fmt.Sprintf("%v", names)
}
