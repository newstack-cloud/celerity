package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/newstack-cloud/celerity/apps/cli/internal/config"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devlogs"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devrun"
	"github.com/newstack-cloud/celerity/apps/cli/internal/docker"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func setupDevLogsCommand(devCmd *cobra.Command, confProvider *config.Provider) {
	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "Stream or filter container logs",
		Long: `Stream logs from the running dev environment container.

Supports filtering by handler name and minimum log level.
When running in a terminal, output is colored by log level.
Log files are written to .celerity/logs/ alongside stdout streaming.

Use --sync to dump all logs to files without streaming to stdout.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDevLogs(cmd, confProvider)
		},
	}

	logsCmd.Flags().String("app-dir", ".", "Application root directory")
	confProvider.BindPFlag("devLogsAppDir", logsCmd.Flags().Lookup("app-dir"))
	confProvider.BindEnvVar("devLogsAppDir", "CELERITY_CLI_DEV_LOGS_APP_DIR")

	logsCmd.Flags().StringP("handler", "H", "", "Filter logs by handler name (substring match)")
	confProvider.BindPFlag("devLogsHandler", logsCmd.Flags().Lookup("handler"))

	logsCmd.Flags().StringP("level", "l", "", "Minimum log level (debug, info, warn, error)")
	confProvider.BindPFlag("devLogsLevel", logsCmd.Flags().Lookup("level"))

	logsCmd.Flags().BoolP("follow", "f", true, "Follow log output")
	confProvider.BindPFlag("devLogsFollow", logsCmd.Flags().Lookup("follow"))

	logsCmd.Flags().StringP("tail", "n", "100", "Number of historical lines (or \"all\")")
	confProvider.BindPFlag("devLogsTail", logsCmd.Flags().Lookup("tail"))

	logsCmd.Flags().StringP("since", "s", "", "Show logs since timestamp or duration (e.g. 5m, 2024-01-01T00:00:00Z)")
	confProvider.BindPFlag("devLogsSince", logsCmd.Flags().Lookup("since"))

	logsCmd.Flags().Bool("sync", false, "Dump all logs to files and exit (no stdout)")
	confProvider.BindPFlag("devLogsSync", logsCmd.Flags().Lookup("sync"))

	devCmd.AddCommand(logsCmd)
}

func runDevLogs(cmd *cobra.Command, confProvider *config.Provider) error {
	isColor := term.IsTerminal(int(os.Stdout.Fd()))
	output := devrun.NewOutput(os.Stdout, isColor)

	appDir, _ := confProvider.GetString("devLogsAppDir")
	if appDir == "" || appDir == "." {
		var err error
		appDir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	state, err := devrun.LoadStateForCommand(appDir)
	if err != nil {
		output.PrintNoEnvironment()
		return nil
	}

	logger, logHandle, err := setupDevLogger()
	if err != nil {
		return err
	}
	defer logHandle.Close()

	dockerMgr, err := docker.NewRuntimeContainer(logger)
	if err != nil {
		return err
	}

	syncMode, _ := confProvider.GetBool("devLogsSync")
	if syncMode {
		return runDevLogsSync(
			cmd, confProvider, appDir, state.ContainerID, state.Runtime, dockerMgr, output,
		)
	}

	return runDevLogsStream(
		cmd, confProvider, state.ContainerID, state.Runtime, dockerMgr, isColor,
	)
}

func runDevLogsSync(
	cmd *cobra.Command,
	confProvider *config.Provider,
	appDir string,
	containerID string,
	runtime string,
	dockerMgr docker.RuntimeContainerManager,
	output *devrun.Output,
) error {
	celerityDir := filepath.Join(appDir, ".celerity")

	devlogs.CleanLogDir(celerityDir)

	fileWriter, err := devlogs.NewLogFileWriter(celerityDir)
	if err != nil {
		return fmt.Errorf("creating log files: %w", err)
	}
	defer fileWriter.Close()

	since, _ := confProvider.GetString("devLogsSince")

	streamer := devlogs.NewStreamer(dockerMgr, runtime, false)
	streamer.FileWriter = fileWriter

	result, err := streamer.SyncToFiles(cmd.Context(), containerID, devlogs.StreamOptions{
		Since: since,
	})
	if err != nil {
		return err
	}

	output.PrintStep(fmt.Sprintf("Synced %d lines to %s", result.TotalLines, result.LogDir))
	if len(result.HandlerFiles) > 0 {
		output.PrintInfo(fmt.Sprintf("  %s", strings.Join(result.HandlerFiles, ", ")))
	}

	return nil
}

func runDevLogsStream(
	cmd *cobra.Command,
	confProvider *config.Provider,
	containerID string,
	runtime string,
	dockerMgr docker.RuntimeContainerManager,
	isColor bool,
) error {
	follow, _ := confProvider.GetBool("devLogsFollow")
	tail, _ := confProvider.GetString("devLogsTail")
	since, _ := confProvider.GetString("devLogsSince")
	handler, _ := confProvider.GetString("devLogsHandler")
	level, _ := confProvider.GetString("devLogsLevel")

	streamer := devlogs.NewStreamer(dockerMgr, runtime, isColor)

	return streamer.Stream(cmd.Context(), containerID, devlogs.StreamOptions{
		Follow:        follow,
		Tail:          tail,
		Since:         since,
		HandlerFilter: handler,
		LevelFilter:   level,
	}, os.Stdout)
}
