package commands

import (
	"fmt"

	"github.com/newstack-cloud/celerity/apps/cli/internal/sqlschema"
	"github.com/spf13/cobra"
)

func setupSchemaHashCommand(schemaCmd *cobra.Command) {
	hashCmd := &cobra.Command{
		Use:   "hash",
		Short: "Recompute the migration directory integrity file",
		Long: `Recompute atlas.sum from the migration files currently on disk.

Run this after editing a migration file by hand, typically the empty file that
` + "`celerity schema new`" + ` created, so the integrity file matches its
contents again. Every other command treats a mismatch as an error, because on a
migration that has already been applied it means the recorded history no longer
matches what actually ran.

Only re-hash migrations that have not been applied anywhere. Editing an applied
migration and re-hashing hides the divergence rather than fixing it; add a new
forward migration instead.

Example:
  celerity schema hash`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			resource, err := resolveDatabaseResource(cmd)
			if err != nil {
				return err
			}
			if resource.MigrationsPath == "" {
				return fmt.Errorf("resource %q has no migrationsPath configured", resource.ResourceName)
			}

			if err := sqlschema.RehashMigrationDir(resource.MigrationsPath); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated %s/atlas.sum\n", resource.MigrationsPath)
			return nil
		},
	}
	schemaCmd.AddCommand(hashCmd)
}
