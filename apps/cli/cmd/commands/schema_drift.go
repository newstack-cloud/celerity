package commands

import (
	"fmt"

	"github.com/newstack-cloud/celerity/apps/cli/internal/sqlschema"
	"github.com/newstack-cloud/deploy-cli-sdk/config"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func setupSchemaDriftCommand(schemaCmd *cobra.Command, confProvider *config.Provider) {
	driftCmd := &cobra.Command{
		Use:   "drift",
		Short: "Check a live database for out-of-band changes",
		Long: `Check a live database against the state its committed migrations should have
produced, at three layers:

  History     the atlas_schema_revisions table vs the migration directory
              (migrations applied out-of-band, missing, or applied out of order)
  Structural  live tables/columns/indexes/keys/constraints vs the expected state
  Advanced    triggers, functions, views and RLS policies fingerprinted from the
              catalog (created / changed / missing out-of-band)

Choose the target database with one of:
  --url            an explicit connection string
  --instance-name  a deployed blueprint instance (connection resolved from state)

Structural and advanced-object drift are reconstructed by replaying the applied
migrations into a temporary dev database (a throwaway container by default, or
--dev-url). Pass --history-only to skip that and check history drift alone (no
Docker or dev database required).

The command exits non-zero when drift is found, so it can run as a scheduled CI
job for continuous checking.

Examples:
  celerity schema drift --instance-name production-orders
  celerity schema drift --url postgres://user:pass@localhost:5432/orders?sslmode=disable
  celerity schema drift --instance-name production-orders --history-only`,
		RunE: func(cmd *cobra.Command, args []string) error {
			url, _ := cmd.Flags().GetString("url")
			instanceName, _ := cmd.Flags().GetString("instance-name")
			devURL, _ := cmd.Flags().GetString("dev-url")
			historyOnly, _ := cmd.Flags().GetBool("history-only")

			if url == "" && instanceName == "" {
				return fmt.Errorf("provide a target database with --url or --instance-name")
			}
			if url != "" && instanceName != "" {
				return fmt.Errorf("--url and --instance-name are mutually exclusive")
			}

			resource, err := resolveDatabaseResource(cmd)
			if err != nil {
				return err
			}
			if resource.MigrationsPath == "" {
				return fmt.Errorf("resource %q has no migrationsPath configured", resource.ResourceName)
			}

			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			target := "url"
			if url == "" {
				resolved, err := resolveDeployedConnection(ctx, confProvider, resource, instanceName)
				if err != nil {
					return err
				}
				target = resolved.Target
				url = resolved.ConnectionString
			}

			devConn := devURL
			if !historyOnly && devConn == "" {
				fmt.Fprintln(out, "Starting dev database for drift comparison...")
				u, cleanup, err := provisionDevPostgres(ctx, zap.NewNop())
				if err != nil {
					return err
				}
				defer cleanup()
				devConn = u
			}
			if historyOnly {
				devConn = ""
			}

			applier, err := sqlschema.NewApplier(resource.Engine, url, zap.NewNop())
			if err != nil {
				return err
			}
			defer applier.Close()

			report, err := sqlschema.DetectDrift(ctx, applier, devConn, resource.MigrationsPath, resource.Engine)
			if err != nil {
				return err
			}
			report.Resource = resource.ResourceName
			report.Target = target
			report.Contracts, err = evaluateResourceContracts(cmd, resource, report.StructuralTables)
			if err != nil {
				return err
			}
			report.Render(out)

			if report.HasDrift() {
				return fmt.Errorf("drift detected for %s", resource.ResourceName)
			}
			return nil
		},
	}

	driftCmd.Flags().String("url", "", "Connection string of the target database.")
	driftCmd.Flags().String("instance-name", "", "Deployed blueprint instance whose database to check.")
	driftCmd.Flags().String(
		"dev-url", "",
		"Connection string for an empty dev database used to reconstruct the expected "+
			"state (default: start a throwaway container).",
	)
	driftCmd.Flags().Bool("history-only", false, "Only check history drift (no dev database or Docker required).")
	schemaCmd.AddCommand(driftCmd)
}
