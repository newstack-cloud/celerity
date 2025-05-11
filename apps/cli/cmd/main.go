package main

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/two-hundred/celerity/apps/cli/cmd/commands"
)

func init() {
	cobra.OnInitialize(commands.OnInitialise)
}

func main() {
	rootCmd := commands.NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
