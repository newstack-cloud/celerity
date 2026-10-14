package commands

import (
	"fmt"

	"github.com/newstack-cloud/celerity/apps/cli/internal/sqlschema"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func setupSchemaDiffCommand(schemaCmd *cobra.Command) {
	diffCmd := &cobra.Command{
		Use:   "diff",
		Short: "Generate a migration file from schema YAML changes",
		Long: `Compute the difference between the schema YAML (desired state) and the current
state, which is obtained by replaying the existing migration directory on a temporary
dev database, and then writing it as a new timestamped migration file.

By default a throwaway PostgreSQL container is started for the replay (requires
Docker). Pass --dev-url to use an existing empty database instead.

In --check mode, no migration is written; the command instead exits non-zero if
a migration *would* be generated. Use it in CI to prove the committed migrations
are complete for the committed schema (the counterpart to committing migration
files and atlas.sum alongside the schema change).

Examples:
  celerity schema diff --name add_customer_tier
  celerity schema diff --check
  celerity schema diff --dev-url postgres://user:pass@localhost:5432/dev?sslmode=disable`,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			devURL, _ := cmd.Flags().GetString("dev-url")
			check, _ := cmd.Flags().GetBool("check")

			resource, err := resolveDatabaseResource(cmd)
			if err != nil {
				return err
			}
			if resource.SchemaPath == "" {
				return fmt.Errorf("resource %q has no schemaPath configured", resource.ResourceName)
			}
			if resource.MigrationsPath == "" {
				return fmt.Errorf("resource %q has no migrationsPath configured", resource.ResourceName)
			}

			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			if devURL == "" {
				fmt.Fprintln(out, "Starting dev database for schema replay...")
				url, cleanup, err := provisionDevPostgres(ctx, zap.NewNop())
				if err != nil {
					return err
				}
				defer cleanup()
				devURL = url
			}

			// The tables the pending change touches, and any schema contracts
			// they affect. Computed before generating, so the migration
			// directory does not yet include the new migration.
			changed, err := sqlschema.ChangedTables(ctx, devURL, resource.SchemaPath, resource.MigrationsPath, resource.Engine)
			if err != nil {
				return err
			}
			impacts, err := evaluateResourceContracts(cmd, resource, changed)
			if err != nil {
				return err
			}

			if check {
				pending, err := sqlschema.CheckPendingMigration(
					ctx, devURL, resource.SchemaPath, resource.MigrationsPath, resource.Engine,
				)
				if err != nil {
					return err
				}
				if pending {
					sqlschema.RenderContractImpacts(out, impacts)
					return fmt.Errorf(
						"schema for %q has changes with no committed migration; run `celerity schema diff` and commit the generated file and atlas.sum",
						resource.ResourceName)
				}
				fmt.Fprintf(out, "%s: migrations are up to date with the schema.\n", resource.ResourceName)
				return nil
			}

			path, err := sqlschema.GenerateMigration(
				ctx, devURL, resource.SchemaPath, resource.MigrationsPath, resource.Engine, name,
			)
			if err != nil {
				return err
			}
			if path == "" {
				fmt.Fprintf(out, "No schema changes for %s, so no migration was generated.\n", resource.ResourceName)
				return nil
			}
			risk, _ := sqlschema.ReadMigrationRisk(path)
			fmt.Fprintf(out, "Generated %s  [risk: %s]\n", path, risk)
			if risk != sqlschema.RiskSafe {
				fmt.Fprintf(out, "This migration is %s, so `schema apply` will not auto-apply it without --allow-%s.\n", risk, risk)
			}
			sqlschema.RenderContractImpacts(out, impacts)
			fmt.Fprintln(out, "Review the file, then apply it with: celerity schema apply")
			return nil
		},
	}

	diffCmd.Flags().String("name", "", "Description used in the generated migration file name.")
	diffCmd.Flags().String(
		"dev-url", "",
		"Connection string for an empty dev database to replay migrations into "+
			"(default: start a throwaway container).",
	)
	diffCmd.Flags().Bool("check", false, "Do not write; exit non-zero if a migration would be generated (CI completeness gate).")
	schemaCmd.AddCommand(diffCmd)
}
