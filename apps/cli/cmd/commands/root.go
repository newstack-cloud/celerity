package commands

import (
	"fmt"
	"os"
	"runtime"

	"github.com/newstack-cloud/celerity/apps/cli/cmd/utils"
	"github.com/newstack-cloud/celerity/apps/cli/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func NewRootCmd() *cobra.Command {
	var configFile string

	confProvider := config.NewProvider()

	cobra.AddTemplateFunc("wrappedFlagUsages", utils.WrappedFlagUsages)
	rootCmd := &cobra.Command{
		Use:   "celerity",
		Short: "CLI for managing celerity applications and blueprint deployments",
		Long: `The CLI for the backend toolkit that gets you moving fast.
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
		"deploy-config-file",
		"celerity.deploy.json",
		"The path to the deployment configuration JSON file that will be used as"+
			" a source of blueprint variable overrides, provider configuration, "+
			"transformer configuration and general configuration. "+
			"The contents of this file is sent in requests to the deploy engine for "+
			"validation, change staging and deployment.",
	)
	confProvider.BindPFlag("deployConfigFile", rootCmd.PersistentFlags().Lookup("deploy-config-file"))
	confProvider.BindEnvVar("deployConfigFile", "CELERITY_CLI_DEPLOY_CONFIG_FILE")

	rootCmd.PersistentFlags().String(
		"connect-protocol",
		// Connect to a local instance of the deploy engine
		// via a unix socket by default.
		"unix",
		"The protocol to connect to the deploy engine with, "+
			"this can be either \"unix\" or \"tcp\". A unix socket can only be used on linux, macos, and other unix-like operating systems. "+
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

	rootCmd.PersistentFlags().Bool(
		"skip-plugin-config-validation",
		false,
		"Skip validation of the plugin-specific entries in the deploy configuration file for commands that interact with the deploy engine.",
	)
	confProvider.BindPFlag("skipPluginConfigValidation", rootCmd.PersistentFlags().Lookup("skip-plugin-config-validation"))
	confProvider.BindEnvVar("skipPluginConfigValidation", "CELERITY_CLI_SKIP_PLUGIN_CONFIG_VALIDATION")

	setupVersionCommand(rootCmd)
	setupInitCommand(rootCmd, confProvider)
	setupValidateCommand(rootCmd, confProvider)

	return rootCmd
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

func OnInitialise() {
	asciiArt := `
	   ___     _           _ _         
	  / __\___| | ___ _ __(_) |_ _   _ 
	 / /  / _ \ |/ _ \ '__| | __| | | |
	/ /__|  __/ |  __/ |  | | |_| |_| |
	\____/\___|_|\___|_|  |_|\__|\__, |
				     |___/ 
	`

	inTerminal := term.IsTerminal(int(os.Stdout.Fd()))
	if inTerminal {
		// Only print the ASCII art if we're in an interactive terminal,
		// it can be a nuisance when in environments like CI/CD
		// workflows or where the only expected output is formatted JSON
		// or similar.
		fmt.Println(asciiArt)
	}
}
