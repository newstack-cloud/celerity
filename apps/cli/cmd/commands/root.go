package commands

import (
	"log"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
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
	Run: func(cmd *cobra.Command, args []string) {
		// Do Stuff Here
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
