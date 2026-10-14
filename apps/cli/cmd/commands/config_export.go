package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/newstack-cloud/deploy-cli-sdk/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

func setupConfigExportCommand(configCmd *cobra.Command, confProvider *config.Provider) {
	exportCmd := &cobra.Command{
		Use:   "export <store>",
		Short: "Export config values to stdout or file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			storeName := args[0]
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

			entries, err := store.List(cmd.Context())
			if err != nil {
				return err
			}

			data := make(map[string]string, len(entries))
			for _, e := range entries {
				data[e.Key] = e.Value
			}

			format, _ := confProvider.GetString("configExportFormat")
			outFile, outIsDefault := confProvider.GetString("configExportOut")

			var encoded []byte
			switch format {
			case "json":
				encoded, err = json.MarshalIndent(data, "", "  ")
				if err != nil {
					return fmt.Errorf("encoding JSON: %w", err)
				}
				encoded = append(encoded, '\n')
			default:
				encoded, err = yaml.Marshal(data)
				if err != nil {
					return fmt.Errorf("encoding YAML: %w", err)
				}
			}

			if !outIsDefault && outFile != "" {
				if err := os.WriteFile(outFile, encoded, 0o644); err != nil {
					return fmt.Errorf("writing to %s: %w", outFile, err)
				}
				fmt.Fprintf(os.Stderr, "Exported %d values to %s\n", len(data), outFile)
				return nil
			}

			_, err = os.Stdout.Write(encoded)
			return err
		},
	}

	exportCmd.Flags().String("format", "yaml",
		"Output format: yaml (default) or json.")
	confProvider.BindPFlag("configExportFormat", exportCmd.Flags().Lookup("format"))
	confProvider.BindEnvVar("configExportFormat", "CELERITY_CLI_CONFIG_EXPORT_FORMAT")

	exportCmd.Flags().StringP("out", "o", "",
		"Output file path. Defaults to stdout.")
	confProvider.BindPFlag("configExportOut", exportCmd.Flags().Lookup("out"))
	confProvider.BindEnvVar("configExportOut", "CELERITY_CLI_CONFIG_EXPORT_OUT")

	configCmd.AddCommand(exportCmd)
}
