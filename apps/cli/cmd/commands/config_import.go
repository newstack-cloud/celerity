package commands

import (
	"fmt"
	"os"

	"github.com/newstack-cloud/celerity/apps/cli/internal/appconfig"
	"github.com/newstack-cloud/celerity/apps/cli/internal/seed"
	"github.com/newstack-cloud/celerity/apps/cli/internal/tui/configui"
	"github.com/newstack-cloud/deploy-cli-sdk/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func setupConfigImportCommand(configCmd *cobra.Command, confProvider *config.Provider) {
	importCmd := &cobra.Command{
		Use:   "import <store> <file>",
		Short: "Import config values from a YAML/JSON file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			storeName, filePath := args[0], args[1]
			inTerminal := term.IsTerminal(int(os.Stdout.Fd()))
			appDir, _ := confProvider.GetString("configAppDir")
			output := configui.NewConfigOutput(os.Stdout, inTerminal)

			envInfo, err := resolveEnv(confProvider, appDir, inTerminal)
			if err != nil {
				return err
			}

			store, storeInfo, err := resolveConfigStore(
				cmd.Context(), envInfo, storeName, confProvider,
			)
			if err != nil {
				return err
			}

			values, err := seed.LoadConfigYAML(filePath)
			if err != nil {
				return fmt.Errorf("reading import file: %w", err)
			}

			if len(values) == 0 {
				output.PrintWarning("No values found in import file")
				return nil
			}

			entries := make([]appconfig.ConfigEntry, 0, len(values))
			for k, v := range values {
				sensitive := !storeInfo.PlaintextKeys[k]
				entries = append(entries, appconfig.ConfigEntry{
					Key:       k,
					Value:     v,
					Sensitive: sensitive,
				})
			}

			if err := store.Import(cmd.Context(), entries); err != nil {
				return err
			}

			output.PrintSuccess(fmt.Sprintf(
				"Imported %d values into store %q", len(entries), storeInfo.StoreName,
			))
			return nil
		},
	}

	configCmd.AddCommand(importCmd)
}
