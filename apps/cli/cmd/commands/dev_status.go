package commands

import (
	"os"

	"github.com/newstack-cloud/celerity/apps/cli/internal/config"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devrun"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devstate"
	"github.com/newstack-cloud/celerity/apps/cli/internal/docker"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func setupDevStatusCommand(devCmd *cobra.Command, confProvider *config.Provider) {
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show the current dev environment status",
		Long:  `Displays the current state of the local dev environment including container status, handlers, and dependencies.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDevStatus(cmd, confProvider)
		},
	}

	statusCmd.Flags().String("app-dir", ".", "Application root directory")
	confProvider.BindPFlag("devStatusAppDir", statusCmd.Flags().Lookup("app-dir"))
	confProvider.BindEnvVar("devStatusAppDir", "CELERITY_CLI_DEV_STATUS_APP_DIR")

	devCmd.AddCommand(statusCmd)
}

func runDevStatus(cmd *cobra.Command, confProvider *config.Provider) error {
	isColor := term.IsTerminal(int(os.Stdout.Fd()))
	output := devrun.NewOutput(os.Stdout, isColor)

	appDir, _ := confProvider.GetString("devStatusAppDir")
	if appDir == "" || appDir == "." {
		var err error
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

	isRunning := checkContainerRunning(cmd, state)
	output.PrintStatus(state, isRunning)
	return nil
}

func checkContainerRunning(cmd *cobra.Command, state *devstate.DevState) bool {
	if state.IsProcessAlive() {
		return true
	}

	if state.ContainerID == "" {
		return false
	}

	logger, logHandle, err := setupDevLogger()
	if err != nil {
		return false
	}
	defer logHandle.Close()

	dockerMgr, err := docker.NewRuntimeContainer(logger)
	if err != nil {
		return false
	}

	// Check if the container is still accessible by trying to stream logs briefly.
	// If the container doesn't exist, this will error.
	reader, err := dockerMgr.StreamLogsWithOptions(
		cmd.Context(),
		state.ContainerID,
		docker.LogStreamOptions{Tail: "1"},
	)
	if err != nil {
		return false
	}
	reader.Close()
	return true
}
