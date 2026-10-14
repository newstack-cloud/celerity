package commands

import (
	"fmt"
	"io"

	"github.com/newstack-cloud/celerity/apps/cli/internal/sqlschema"
	"github.com/spf13/cobra"
)

func setupSchemaValidateCommand(schemaCmd *cobra.Command) {
	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate schema files and migration integrity",
		Long: `Validate every celerity/sqlDatabase resource in the blueprint:

  - the schema YAML parses
  - foreign keys reference existing tables and columns
  - column types are understood by the engine
  - the migration directory's atlas.sum integrity file is present and matches

This runs without a database connection, so it is suitable as a CI gate.

Example:
  celerity schema validate`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if checkCodegen, _ := cmd.Flags().GetBool("check-codegen"); checkCodegen {
				// Codegen freshness depends on the output directory and language,
				// which validate is not given, so the freshness gate lives on
				// the codegen command, which has --out and --lang.
				return fmt.Errorf("check generated code with `celerity schema codegen --check --lang <lang> --out <dir>`")
			}

			resources, err := loadDatabaseResources(cmd)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			var problems int
			for _, r := range resources {
				problems += reportValidation(out, r.ResourceName, "schema", validateResourceSchema(r))
				problems += reportValidation(out, r.ResourceName, "migrations", sqlschema.ValidateMigrationDir(r.MigrationsPath))
			}

			problems += validateContracts(cmd, out, resources)

			if problems > 0 {
				return fmt.Errorf("%d schema validation problem(s) found", problems)
			}
			fmt.Fprintf(out, "All %d SQL database schema(s) valid.\n", len(resources))
			return nil
		},
	}

	validateCmd.Flags().Bool("check-codegen", false, "Verify generated code is up to date (points you to `schema codegen --check`).")
	schemaCmd.AddCommand(validateCmd)
}

// validateContracts checks the schema-contracts.yaml (if present) references only
// known databases and tables. It reports one line per contract file checked and
// returns the number of problems found.
func validateContracts(cmd *cobra.Command, out io.Writer, resources []sqlschema.DatabaseResource) int {
	contracts, err := loadSchemaContracts(cmd)
	if err != nil {
		return reportValidation(out, sqlschema.ContractsFileName, "contracts", err)
	}
	if contracts == nil {
		return 0
	}

	schemaProblems := sqlschema.ValidateContracts(contracts, resourceSchemas(resources))
	if len(schemaProblems) == 0 {
		return reportValidation(out, sqlschema.ContractsFileName, "contracts", nil)
	}
	for _, p := range schemaProblems {
		fmt.Fprintf(out, "✗ %s (contracts): %s\n", sqlschema.ContractsFileName, p)
	}
	return len(schemaProblems)
}

func validateResourceSchema(r sqlschema.DatabaseResource) error {
	if r.SchemaPath == "" {
		return nil
	}
	return sqlschema.ValidateSchema(r.SchemaPath, r.Engine)
}

func reportValidation(out io.Writer, resourceName, check string, err error) int {
	if err != nil {
		fmt.Fprintf(out, "✗ %s (%s): %v\n", resourceName, check, err)
		return 1
	}
	fmt.Fprintf(out, "✓ %s (%s)\n", resourceName, check)
	return 0
}
