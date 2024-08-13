package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(validateCmd)
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validates a Celerity application or blueprint",
	Long: `Carries out validation on a Celerity application or blueprint.
You can use this command to check for issues with an application or blueprint
before deployment.

It's worth noting that validation is carried out as a part of the deploy command as well.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Celerity CLI v0.1")
	},
}
