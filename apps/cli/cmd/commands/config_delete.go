package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/newstack-cloud/celerity/apps/cli/internal/tui/configui"
	"github.com/newstack-cloud/deploy-cli-sdk/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func setupConfigDeleteCommand(configCmd *cobra.Command, confProvider *config.Provider) {
	deleteCmd := &cobra.Command{
		Use:   "delete <store> <key>",
		Short: "Delete a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			storeName, key := args[0], args[1]
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

			yesFlag, _ := confProvider.GetBool("configDeleteYes")
			if !yesFlag {
				if !inTerminal {
					return fmt.Errorf("--yes is required in non-interactive mode")
				}
				fmt.Fprintf(os.Stderr,
					"Delete key %q from %s? [y/N]: ", key, storeInfo.StoreName,
				)
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(answer)), "y") {
					output.PrintInfo("Cancelled")
					return nil
				}
			}

			if err := store.Delete(cmd.Context(), key); err != nil {
				return err
			}

			output.PrintSuccess(fmt.Sprintf(
				"Deleted key %q from store %q", key, storeInfo.StoreName,
			))
			return nil
		},
	}

	deleteCmd.Flags().BoolP("yes", "y", false,
		"Skip confirmation (required in headless mode).")
	confProvider.BindPFlag("configDeleteYes", deleteCmd.Flags().Lookup("yes"))
	confProvider.BindEnvVar("configDeleteYes", "CELERITY_CLI_CONFIG_DELETE_YES")

	configCmd.AddCommand(deleteCmd)
}
