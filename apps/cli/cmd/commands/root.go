package commands

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/two-hundred/celerity/apps/cli/cmd/utils"
	"github.com/two-hundred/celerity/apps/cli/internal/config"
)

func Execute() {
	var configFile string

	confProvider := config.NewProvider()

	cobra.AddTemplateFunc("wrappedFlagUsages", utils.WrappedFlagUsages)
	rootCmd := &cobra.Command{
		Use:   "celerity",
		Short: "CLI for managing celerity applications",
		Long: `
	   ___     _           _ _         
	  / __\___| | ___ _ __(_) |_ _   _ 
	 / /  / _ \ |/ _ \ '__| | __| | | |
	/ /__|  __/ |  __/ |  | | |_| |_| |
	\____/\___|_|\___|_|  |_|\__|\__, |
			             |___/ 
																					   
The CLI for the backend toolkit that gets you moving fast.
This CLI validates, builds, and deploys celerity applications.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := confProvider.LoadConfigFile(configFile); err != nil {
				return err
			}

			connectProtocol, _ := confProvider.GetString("connectProtocol")
			validateConnectProtocol(connectProtocol)
			return nil
		},
	}

	rootCmd.SetUsageTemplate(utils.UsageTemplate)
	rootCmd.SetHelpTemplate(utils.HelpTemplate)

	rootCmd.Flags().StringVarP(
		&configFile,
		"config",
		"c",
		"celerity.config.toml",
		"Specify a config file to source config from as an alternative to flags",
	)

	rootCmd.PersistentFlags().BoolP(
		"embedded",
		"e",
		false,
		"Set this flag when you want to use the embedded build engine, "+
			"the http api is used by default.",
	)
	confProvider.BindPFlag("embeddedEngine", rootCmd.PersistentFlags().Lookup("embedded"))
	confProvider.BindEnvVar("embeddedEngine", "CELERITY_CLI_EMBEDDED_ENGINE")

	rootCmd.PersistentFlags().String(
		"connect-protocol",
		// Connect to a local instance of the build engine
		// via a unix socket by default.
		"unix-socket",
		"The protocol to connect to the build engine with, "+
			"can be either \"unix-socket\" or \"tcp\". This is ignored if the --embedded flag is set.",
	)
	confProvider.BindPFlag("connectProtocol", rootCmd.PersistentFlags().Lookup("connect-protocol"))
	confProvider.BindEnvVar("connectProtocol", "CELERITY_CLI_CONNECT_PROTOCOL")

	rootCmd.PersistentFlags().String(
		"api-endpoint",
		"http://localhost:8325",
		"The endpoint of the build engine api, this is used if --connect-protocol is set to tcp",
	)
	setupVersionCommand(rootCmd)
	setupInitCommand(rootCmd, confProvider)
	setupValidateCommand(rootCmd, confProvider)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func validateConnectProtocol(protocol string) error {
	if protocol == "unix-socket" || protocol == "tcp" {
		return nil
	}

	return fmt.Errorf(
		"invalid connect protocol \"%s\" provided, must be either \"unix-socket\" or \"tcp\"",
		protocol,
	)
}
