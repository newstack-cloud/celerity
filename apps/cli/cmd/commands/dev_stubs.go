package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/newstack-cloud/celerity/apps/cli/internal/config"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devstubs"
	"github.com/spf13/cobra"
)

func setupDevStubsCommand(devCmd *cobra.Command, confProvider *config.Provider) {
	stubsCmd := &cobra.Command{
		Use:   "stubs",
		Short: "Manage HTTP service stubs",
		Long: `Manage over-the-wire HTTP service stubs backed by mountebank.

  celerity dev stubs validate   Validate stub definitions in stubs/`,
	}

	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate stub definitions",
		Long:  `Load and validate all stub YAML files in the stubs/ directory. Checks for YAML syntax errors, missing required fields, port conflicts, and duplicate config keys.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStubsValidate(confProvider)
		},
	}

	validateCmd.Flags().String("app-dir", "", "Application root directory (default: current directory)")
	confProvider.BindPFlag("devStubsAppDir", validateCmd.Flags().Lookup("app-dir"))

	stubsCmd.AddCommand(validateCmd)
	devCmd.AddCommand(stubsCmd)
}

func runStubsValidate(confProvider *config.Provider) error {
	appDir, _ := confProvider.GetString("devStubsAppDir")
	if appDir == "" {
		var err error
		appDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("resolving working directory: %w", err)
		}
	}

	stubsDir := filepath.Join(appDir, "stubs")
	if _, err := os.Stat(stubsDir); os.IsNotExist(err) {
		fmt.Println("No stubs/ directory found — nothing to validate.")
		return nil
	}

	services, err := devstubs.LoadStubs(appDir)
	if err != nil {
		return fmt.Errorf("loading stubs: %w", err)
	}

	if len(services) == 0 {
		fmt.Println("No stub services found in stubs/ directory.")
		return nil
	}

	errs := devstubs.ValidateStubs(services)
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "Validation failed with %d error(s):\n", len(errs))
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  ✗ %s\n", e.Error())
		}
		return fmt.Errorf("stub validation failed")
	}

	totalEndpoints := 0
	totalStubs := 0
	for _, svc := range services {
		totalEndpoints += len(svc.Endpoints)
		for _, ep := range svc.Endpoints {
			totalStubs += len(ep.Stubs)
		}
	}

	fmt.Printf("Validated %d service(s), %d endpoint(s), %d stub(s) — all OK.\n",
		len(services), totalEndpoints, totalStubs)
	return nil
}
