package commands

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/two-hundred/celerity/apps/cli/internal/config"
	"github.com/two-hundred/celerity/apps/cli/internal/consts"
	"github.com/two-hundred/celerity/apps/cli/internal/tui/initui"
	"github.com/two-hundred/celerity/libs/common/core"
)

func setupInitCommand(rootCmd *cobra.Command, confProvider *config.Provider) {
	supportedLanguagesStr := strings.Join(
		core.Map(consts.SupportedLanguages, quote),
		", ",
	)
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialises a new Celerity project",
		Long: `Initialises a new Celerity project, this will take you through an interactive set up
		process but you can also use flags to skip certain prompts.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			lang, _ := confProvider.GetString("initLanguage")
			err := validateLanguage(lang, supportedLanguagesStr)
			if err != nil {
				return err
			}
			_, err = tea.NewProgram(initui.NewInitApp(lang)).Run()
			return err
		},
	}

	initCmd.PersistentFlags().StringP(
		"language",
		"l",
		"",
		fmt.Sprintf("The programming language/framework you want to use for the new project. Can be one of %s.", supportedLanguagesStr),
	)
	confProvider.BindPFlag("initLanguage", initCmd.PersistentFlags().Lookup("language"))
	confProvider.BindEnvVar("initLanguage", "CELERITY_CLI_INIT_LANGUAGE")

	rootCmd.AddCommand(initCmd)
}

func validateLanguage(lang string, supportedLanguagesText string) error {
	if lang == "" {
		// Empty language is fine, it means the user will have to choose one
		// in the interactive TUI.
		return nil
	}
	if slices.Contains(consts.SupportedLanguages, lang) {
		return nil
	}

	return fmt.Errorf(
		"unsupported language: \"%s\", must be one of %s",
		lang,
		supportedLanguagesText,
	)
}

func quote(s string, _ int) string {
	return fmt.Sprintf(`"%s"`, s)
}
