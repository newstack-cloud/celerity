package commands

import (
	"fmt"

	"github.com/newstack-cloud/celerity/apps/cli/internal/sqlschema"
	"github.com/newstack-cloud/deploy-cli-sdk/config"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func setupSchemaHistoryCommand(schemaCmd *cobra.Command, confProvider *config.Provider) {
	historyCmd := &cobra.Command{
		Use:   "history",
		Short: "Show the applied migration history of a database",
		Long: `List the migrations applied to a database, in order, from its
atlas_schema_revisions table.

Choose the target database with --url or --instance-name (a deployed instance,
resolved from its state).

Examples:
  celerity schema history --instance-name production-orders
  celerity schema history --url postgres://user:pass@localhost:5432/orders?sslmode=disable`,
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

			records, err := sqlschema.ReadMigrationHistory(cmd.Context(), applier)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Migration history for %s (%s)\n\n", resource.ResourceName, target)
			sqlschema.RenderMigrationHistory(out, records)
			return nil
		},
	}

	historyCmd.Flags().String("url", "", "Connection string of the target database.")
	historyCmd.Flags().String("instance-name", "", "Deployed blueprint instance whose database to read.")
	schemaCmd.AddCommand(historyCmd)
}
