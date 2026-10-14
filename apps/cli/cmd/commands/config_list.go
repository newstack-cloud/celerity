package commands

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/newstack-cloud/celerity/apps/cli/internal/appconfig"
	"github.com/newstack-cloud/celerity/apps/cli/internal/blueprint"
	"github.com/newstack-cloud/celerity/apps/cli/internal/tui/configui"
	"github.com/newstack-cloud/deploy-cli-sdk/config"
	styles "github.com/newstack-cloud/deploy-cli-sdk/styles"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func setupConfigListCommand(configCmd *cobra.Command, confProvider *config.Provider) {
	listCmd := &cobra.Command{
		Use:   "list [store]",
		Short: "List config stores or keys",
		Long: `Without a store argument, lists all config stores from the blueprint.
With a store argument, lists all keys in that store.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inTerminal := term.IsTerminal(int(os.Stdout.Fd()))
			appDir, _ := confProvider.GetString("configAppDir")

			if len(args) == 0 {
				return listStores(confProvider, appDir, inTerminal)
			}
			return listKeys(cmd, confProvider, args[0], appDir, inTerminal)
		},
	}

	listCmd.Flags().Bool("show-secrets", false,
		"Reveal masked values.")
	confProvider.BindPFlag("configListShowSecrets", listCmd.Flags().Lookup("show-secrets"))
	confProvider.BindEnvVar("configListShowSecrets", "CELERITY_CLI_CONFIG_LIST_SHOW_SECRETS")

	configCmd.AddCommand(listCmd)
}

func listStores(
	confProvider *config.Provider,
	appDir string,
	inTerminal bool,
) error {
	bpFile, bpIsDefault := confProvider.GetString("configBlueprintFile")
	if bpIsDefault || bpFile == "" {
		bpFile = appDir + "/app.blueprint.yaml"
	}

	bp, _, err := blueprint.LoadForDev(bpFile)
	if err != nil {
		return err
	}

	stores := appconfig.CollectStoreInfo(bp)
	headless := !inTerminal

	if headless {
		output := configui.NewConfigOutput(os.Stdout, false)
		output.PrintStoreList(stores)
		return nil
	}

	celerityStyles := styles.NewStyles(lipgloss.NewRenderer(os.Stdout), styles.NewCelerityPalette())
	app := configui.NewListModel(configui.ListModelOptions{
		Stores:         stores,
		Styles:         celerityStyles,
		Headless:       false,
		HeadlessWriter: os.Stdout,
	})

	options := []tea.ProgramOption{
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	}
	_, err = tea.NewProgram(app, options...).Run()
	return err
}

func listKeys(
	cmd *cobra.Command,
	confProvider *config.Provider,
	storeName string,
	appDir string,
	inTerminal bool,
) error {
	envInfo, err := resolveEnv(confProvider, appDir, inTerminal)
	if err != nil {
		return err
	}

	store, _, err := resolveConfigStore(
		cmd.Context(), envInfo, storeName, confProvider,
	)
	if err != nil {
		return err
	}

	entries, err := store.List(cmd.Context())
	if err != nil {
		return err
	}

	showSecrets, _ := confProvider.GetBool("configListShowSecrets")
	output := configui.NewConfigOutput(os.Stdout, inTerminal)
	output.PrintEntryTable(entries, showSecrets)
	return nil
}
