package commands

import (
	"fmt"

	"github.com/newstack-cloud/celerity/apps/cli/internal/sqlschema"
	"github.com/spf13/cobra"
)

func setupSchemaNewCommand(schemaCmd *cobra.Command) {
	newCmd := &cobra.Command{
		Use:   "new <description>",
		Short: "Create an empty migration file for hand-written SQL",
		Long: `Create an empty, timestamped migration file in the resource's migrationsPath
and update the atlas.sum integrity file.

Use this for structural SQL the schema YAML cannot express, such as triggers,
functions, stored procedures, partitioning and row-level security policies.

Example:
  celerity schema new add_audit_trigger`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resource, err := resolveDatabaseResource(cmd)
			if err != nil {
				return err
			}
			if resource.MigrationsPath == "" {
				return fmt.Errorf("resource %q has no migrationsPath configured", resource.ResourceName)
			}

			path, err := sqlschema.ScaffoldMigration(resource.MigrationsPath, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", path)
			return nil
		},
	}
	schemaCmd.AddCommand(newCmd)
}
