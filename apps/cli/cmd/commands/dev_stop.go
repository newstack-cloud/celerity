package commands

import (
	"os"

	"github.com/newstack-cloud/celerity/apps/cli/internal/compose"
	"github.com/newstack-cloud/celerity/apps/cli/internal/config"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devrun"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devstate"
	"github.com/newstack-cloud/celerity/apps/cli/internal/docker"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func setupDevStopCommand(devCmd *cobra.Command, confProvider *config.Provider) {
	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the running dev environment",
		Long:  `Tears down the running dev environment by stopping the runtime container and dependency services.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDevStop(cmd, confProvider)
		},
	}

	stopCmd.Flags().String("app-dir", ".", "Application root directory")
	confProvider.BindPFlag("devStopAppDir", stopCmd.Flags().Lookup("app-dir"))
	confProvider.BindEnvVar("devStopAppDir", "CELERITY_CLI_DEV_STOP_APP_DIR")

	devCmd.AddCommand(stopCmd)
}

func runDevStop(cmd *cobra.Command, confProvider *config.Provider) error {
	logger, logHandle, err := setupDevLogger()
	if err != nil {
		return err
	}
	defer logHandle.Close()

	isColor := term.IsTerminal(int(os.Stdout.Fd()))
	output := devrun.NewOutput(os.Stdout, isColor)

	appDir, _ := confProvider.GetString("devStopAppDir")
	if appDir == "" || appDir == "." {
		appDir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	state, err := devstate.Load(appDir)
	if err != nil {
		return err
	}
	if state == nil {
		output.PrintNoEnvironment()
		return nil
	}

	dockerMgr, err := docker.NewRuntimeContainer(logger)
	if err != nil {
		return err
	}

	composeMgr, err := compose.NewComposeManager(
		state.ComposeProject, appDir, logger,
	)
	if err != nil {
		return err
	}

	return devrun.StopFromState(cmd.Context(), appDir, dockerMgr, composeMgr, output, logger)
}
