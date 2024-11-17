package commands

import (
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/two-hundred/celerity/apps/cli/cmd/utils"
	"github.com/two-hundred/celerity/apps/cli/internal/config"
	"github.com/two-hundred/celerity/apps/cli/internal/engine"
	"github.com/two-hundred/celerity/apps/cli/internal/tui/validateui"
)

func setupValidateCommand(rootCmd *cobra.Command, confProvider *config.Provider) {
	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validates a Celerity application or blueprint",
		Long: `Carries out validation on a Celerity application or blueprint.
	You can use this command to check for issues with an application or blueprint
	before deployment.
	
	It's worth noting that validation is carried out as a part of the deploy command as well.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger, handle, err := utils.SetupLogger()
			if err != nil {
				return err
			}
			defer handle.Close()

			deployEngine := engine.Select(confProvider, logger)
			blueprintFile, isDefault := confProvider.GetString("validateBlueprintFile")

			if _, err := tea.LogToFile("debug.log", "simple"); err != nil {
				log.Fatal(err)
			}

			app, err := validateui.NewValidateApp(deployEngine, blueprintFile, isDefault)
			if err != nil {
				return err
			}
			_, err = tea.NewProgram(app).Run()
			return err
		},
	}

	validateCmd.PersistentFlags().StringP(
		"blueprint-file",
		"b",
		"app.blueprint.yaml",
		"The blueprint file to use in the validation process.",
	)
	confProvider.BindPFlag("validateBlueprintFile", validateCmd.PersistentFlags().Lookup("blueprint-file"))
	confProvider.BindEnvVar("validateBlueprintFile", "CELERITY_CLI_VALIDATE_BLUEPRINT_FILE")

	rootCmd.AddCommand(validateCmd)
}
