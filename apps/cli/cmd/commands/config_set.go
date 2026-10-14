package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/newstack-cloud/celerity/apps/cli/internal/tui/configui"
	"github.com/newstack-cloud/deploy-cli-sdk/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func setupConfigSetCommand(configCmd *cobra.Command, confProvider *config.Provider) {
	setCmd := &cobra.Command{
		Use:   "set <store> <key> [value]",
		Short: "Set a config value",
		Long: `Set a config value in a deployed store.

If value is omitted and the key is sensitive, you will be prompted for
masked input in interactive mode, or the value is read from stdin in
headless mode.`,
		Args: cobra.RangeArgs(2, 3),
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

			secretFlag, _ := confProvider.GetBool("configSetSecret")
			sensitive := secretFlag || !storeInfo.PlaintextKeys[key]

			value, err := resolveValue(args, sensitive, inTerminal)
			if err != nil {
				return err
			}

			if err := store.Set(cmd.Context(), key, value, sensitive); err != nil {
				return err
			}

			output.PrintSuccess(fmt.Sprintf(
				"Set key %q in store %q", key, storeInfo.StoreName,
			))
			return nil
		},
	}

	setCmd.Flags().BoolP("secret", "s", false,
		"Mark value as sensitive (SecureString in Parameter Store).")
	confProvider.BindPFlag("configSetSecret", setCmd.Flags().Lookup("secret"))
	confProvider.BindEnvVar("configSetSecret", "CELERITY_CLI_CONFIG_SET_SECRET")

	configCmd.AddCommand(setCmd)
}

func resolveValue(
	args []string,
	sensitive bool,
	inTerminal bool,
) (string, error) {
	if len(args) >= 3 {
		return args[2], nil
	}

	if !sensitive {
		return "", fmt.Errorf("value argument is required for non-sensitive keys")
	}

	if inTerminal {
		fmt.Fprint(os.Stderr, "Enter secret value: ")
		bytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("reading secret input: %w", err)
		}
		return string(bytes), nil
	}

	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("reading value from stdin: %w", err)
	}
	return strings.TrimRight(value, "\n\r"), nil
}
