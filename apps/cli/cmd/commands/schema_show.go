package commands

import (
	"fmt"

	"github.com/newstack-cloud/celerity/apps/cli/internal/sqlschema"
	"github.com/newstack-cloud/deploy-cli-sdk/config"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func setupSchemaShowCommand(schemaCmd *cobra.Command, confProvider *config.Provider) {
	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show the current schema of a deployed database",
		Long: `Inspect a live database and print its current schema (tables, columns, keys)
as it actually exists, which is useful for confirming what is deployed.

Choose the target database with --url or --instance-name (a deployed instance,
resolved from its state).

Examples:
  celerity schema show --instance-name production-orders
  celerity schema show --url postgres://user:pass@localhost:5432/orders?sslmode=disable`,
		RunE: func(cmd *cobra.Command, args []string) error {
			resource, err := resolveDatabaseResource(cmd)
			if err != nil {
				return err
			}
			url, target, err := resolveTargetConnection(cmd, confProvider, resource)
			if err != nil {
				return err
			}

			applier, err := sqlschema.NewApplier(resource.Engine, url, zap.NewNop())
			if err != nil {
				return err
			}
			defer applier.Close()

			rendered, err := sqlschema.InspectCurrentSchema(cmd.Context(), applier)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s (%s)\n\n", resource.ResourceName, target)
			fmt.Fprint(out, rendered)
			return nil
		},
	}

	showCmd.Flags().String("url", "", "Connection string of the target database.")
	showCmd.Flags().String("instance-name", "", "Deployed blueprint instance whose database to read.")
	schemaCmd.AddCommand(showCmd)
}
