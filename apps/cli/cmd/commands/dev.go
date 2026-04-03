package commands

import (
	"github.com/newstack-cloud/celerity/apps/cli/internal/config"
	"github.com/spf13/cobra"
)

func setupDevCommand(rootCmd *cobra.Command, confProvider *config.Provider) {
	devCmd := &cobra.Command{
		Use:   "dev",
		Short: "Local development environment commands",
		Long: `Manage a local development environment for your Celerity application.

  celerity dev run      Start the dev server and dependencies (Docker-based)
  celerity dev stop     Tear down the running environment
  celerity dev status   Show the current environment status
  celerity dev logs     Stream or filter container logs
  celerity dev test     Run tests with automatic infrastructure setup/teardown
  celerity dev stubs    Manage HTTP service stubs`,
	}

	setupDevRunCommand(devCmd, confProvider)
	setupDevStopCommand(devCmd, confProvider)
	setupDevStatusCommand(devCmd, confProvider)
	setupDevLogsCommand(devCmd, confProvider)
	setupDevTestCommand(devCmd, confProvider)
	setupDevStubsCommand(devCmd, confProvider)

	rootCmd.AddCommand(devCmd)
}
