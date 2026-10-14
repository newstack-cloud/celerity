package commands

import (
	"fmt"
	"os"

	"github.com/newstack-cloud/deploy-cli-sdk/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func setupConfigGetCommand(configCmd *cobra.Command, confProvider *config.Provider) {
	getCmd := &cobra.Command{
		Use:   "get <store> <key>",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			storeName, key := args[0], args[1]
			inTerminal := term.IsTerminal(int(os.Stdout.Fd()))
			appDir, _ := confProvider.GetString("configAppDir")

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

			entry, err := store.Get(cmd.Context(), key)
			if err != nil {
				return err
			}

			fmt.Fprintln(os.Stdout, entry.Value)
			return nil
		},
	}

	configCmd.AddCommand(getCmd)
}
