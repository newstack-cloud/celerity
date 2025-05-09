package commands

import (
	"fmt"
	"log"
	"runtime"

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
		Short: "CLI for managing celerity applications and blueprint deployments",
		Long: `
	   ___     _           _ _         
	  / __\___| | ___ _ __(_) |_ _   _ 
	 / /  / _ \ |/ _ \ '__| | __| | | |
	/ /__|  __/ |  __/ |  | | |_| |_| |
	\____/\___|_|\___|_|  |_|\__|\__, |
			             |___/ 
																					   
The CLI for the backend toolkit that gets you moving fast.
This CLI validates, builds, and deploys celerity applications
along with blueprints used for Infrastructure as Code.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := confProvider.LoadConfigFile(configFile); err != nil {
				return err
			}

			connectProtocol, _ := confProvider.GetString("connectProtocol")
			err := validateConnectProtocol(connectProtocol)
			if err != nil {
				return err
			}

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

	rootCmd.PersistentFlags().String(
		"connect-protocol",
		// Connect to a local instance of the deploy engine
		// via a unix socket by default.
		"unix",
		"The protocol to connect to the deploy engine with, "+
			"can be either \"unix\" or \"tcp\". Unix socket can only be used on linux, macos, and other unix-like systems. "+
			"To use a \"unix\" socket on windows, you will need to use WSL 2 or above.",
	)
	confProvider.BindPFlag("connectProtocol", rootCmd.PersistentFlags().Lookup("connect-protocol"))
	confProvider.BindEnvVar("connectProtocol", "CELERITY_CLI_CONNECT_PROTOCOL")

	rootCmd.PersistentFlags().String(
		"engine-endpoint",
		"http://localhost:8325",
		"The endpoint of the deploy engine api, this is used if --connect-protocol is set to \"tcp\"",
	)
	confProvider.BindPFlag("engineEndpoint", rootCmd.PersistentFlags().Lookup("engine-endpoint"))
	confProvider.BindEnvVar("engineEndpoint", "CELERITY_CLI_ENGINE_ENDPOINT")

	setupVersionCommand(rootCmd)
	setupInitCommand(rootCmd, confProvider)
	setupValidateCommand(rootCmd, confProvider)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func validateConnectProtocol(protocol string) error {
	if protocol == "tcp" {
		return nil
	}

	if protocol == "unix" {
		os := runtime.GOOS
		if os == "windows" {
			return fmt.Errorf(
				"\"unix\" socket is not supported on windows, please use \"tcp\" " +
					"or set up Windows Subsystem for Linux (WSL) version 2 or above to use a unix socket",
			)
		}

		return nil
	}

	return fmt.Errorf(
		"invalid connect protocol \"%s\" provided, must be either \"unix\" or \"tcp\"",
		protocol,
	)
}
