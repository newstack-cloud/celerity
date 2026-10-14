package commands

import (
	"fmt"
	"path/filepath"

	"github.com/newstack-cloud/celerity/apps/cli/internal/blueprint"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devconfig/resolve"
	"github.com/newstack-cloud/celerity/apps/cli/internal/sqlschema"
	"github.com/newstack-cloud/deploy-cli-sdk/config"
	"github.com/spf13/cobra"
)

func setupSchemaCommand(rootCmd *cobra.Command, confProvider *config.Provider) {
	schemaCmd := &cobra.Command{
		Use:   "schema",
		Short: "Manage SQL database schemas and migrations",
		Long: `Manage SQL database schemas for celerity/sqlDatabase resources.

Schema definitions are declarative YAML files describing the desired state of a
database. celerity schema diff generates versioned SQL migration files from
changes to those files, which you review, commit, and apply.

Subcommands:
  celerity schema new <description>   Create an empty migration file for hand-written SQL
  celerity schema hash                Recompute atlas.sum after hand-editing a migration
  celerity schema diff                Generate a migration file from schema YAML changes
  celerity schema apply               Apply pending migrations to a database
  celerity schema validate            Validate schema files and migration integrity
  celerity schema drift               Check a live database for out-of-band changes
  celerity schema codegen             Generate type-safe code from schema definitions
  celerity schema export              Export a schema as SQL, markdown, JSON Schema or Mermaid
  celerity schema show                Show the current schema of a deployed database
  celerity schema history             Show the applied migration history of a database`,
	}

	schemaCmd.PersistentFlags().String(
		"blueprint-file", "",
		"The blueprint file to read celerity/sqlDatabase resources from (default: auto-detect).",
	)
	schemaCmd.PersistentFlags().String(
		"database", "",
		"The celerity/sqlDatabase resource to target (required when the blueprint defines more than one).",
	)

	setupSchemaNewCommand(schemaCmd)
	setupSchemaHashCommand(schemaCmd)
	setupSchemaDiffCommand(schemaCmd)
	setupSchemaApplyCommand(schemaCmd, confProvider)
	setupSchemaValidateCommand(schemaCmd)
	setupSchemaDriftCommand(schemaCmd, confProvider)
	setupSchemaCodegenCommand(schemaCmd)
	setupSchemaExportCommand(schemaCmd)
	setupSchemaShowCommand(schemaCmd, confProvider)
	setupSchemaHistoryCommand(schemaCmd, confProvider)

	rootCmd.AddCommand(schemaCmd)
}

// loadDatabaseResources loads the blueprint and returns all celerity/sqlDatabase
// resources it defines.
func loadDatabaseResources(cmd *cobra.Command) ([]sqlschema.DatabaseResource, error) {
	blueprintFlag, _ := cmd.Flags().GetString("blueprint-file")

	bpPath, err := resolve.BlueprintPath(".", blueprintFlag)
	if err != nil {
		return nil, err
	}
	bp, _, err := blueprint.LoadForDev(bpPath)
	if err != nil {
		return nil, fmt.Errorf("loading blueprint: %w", err)
	}
	resources := sqlschema.CollectDatabaseResources(bp, ".")
	if len(resources) == 0 {
		return nil, fmt.Errorf("no celerity/sqlDatabase resources found in %s", bpPath)
	}
	return resources, nil
}

// loadSchemaContracts finds and loads the schema-contracts.yaml alongside the
// blueprint, returning nil (no error) when there is none.
func loadSchemaContracts(cmd *cobra.Command) (*sqlschema.ContractsFile, error) {
	blueprintFlag, _ := cmd.Flags().GetString("blueprint-file")
	bpPath, err := resolve.BlueprintPath(".", blueprintFlag)
	if err != nil {
		return nil, err
	}
	path, ok := sqlschema.FindContractsFile(filepath.Dir(bpPath))
	if !ok {
		return nil, nil
	}
	return sqlschema.LoadContractsFile(path)
}

// resolveTargetConnection resolves the target database for a read-only command
// from its --url / --instance-name flags, returning the connection string and a
// display label for the target.
func resolveTargetConnection(
	cmd *cobra.Command,
	confProvider *config.Provider,
	resource sqlschema.DatabaseResource,
) (connString, target string, err error) {
	url, _ := cmd.Flags().GetString("url")
	instanceName, _ := cmd.Flags().GetString("instance-name")

	if url == "" && instanceName == "" {
		return "", "", fmt.Errorf("provide a target database with --url or --instance-name")
	}
	if url != "" && instanceName != "" {
		return "", "", fmt.Errorf("--url and --instance-name are mutually exclusive")
	}
	if url != "" {
		return url, "url", nil
	}
	resolved, err := resolveDeployedConnection(cmd.Context(), confProvider, resource, instanceName)
	if err != nil {
		return "", "", err
	}
	return resolved.ConnectionString, resolved.Target, nil
}

// evaluateResourceContracts evaluates the schema contracts (if any) against a set
// of changed tables for a single database resource.
func evaluateResourceContracts(
	cmd *cobra.Command,
	resource sqlschema.DatabaseResource,
	changed []string,
) ([]sqlschema.ContractImpact, error) {
	if len(changed) == 0 {
		return nil, nil
	}
	contracts, err := loadSchemaContracts(cmd)
	if err != nil {
		return nil, err
	}
	if contracts == nil {
		return nil, nil
	}
	changedByName := map[string][]string{resource.ResourceName: changed}
	if resource.Name != "" {
		changedByName[resource.Name] = changed
	}
	return sqlschema.EvaluateContracts(contracts, changedByName), nil
}

// resourceSchemas parses every resource's schema YAML, keyed by both resource
// name and configured database name so contracts can reference either. Resources
// whose schema fails to parse are skipped (schema validation reports those
// separately).
func resourceSchemas(resources []sqlschema.DatabaseResource) map[string]*sqlschema.Schema {
	schemas := map[string]*sqlschema.Schema{}
	for _, r := range resources {
		if r.SchemaPath == "" {
			continue
		}
		parsed, err := sqlschema.ParseSchemaFile(r.SchemaPath)
		if err != nil {
			continue
		}
		schemas[r.ResourceName] = parsed
		if r.Name != "" {
			schemas[r.Name] = parsed
		}
	}
	return schemas
}

// resolveDatabaseResource loads the blueprint and returns the target
// celerity/sqlDatabase resource, selected by the --database flag when the
// blueprint defines more than one.
func resolveDatabaseResource(cmd *cobra.Command) (sqlschema.DatabaseResource, error) {
	blueprintFlag, _ := cmd.Flags().GetString("blueprint-file")
	databaseFlag, _ := cmd.Flags().GetString("database")

	bpPath, err := resolve.BlueprintPath(".", blueprintFlag)
	if err != nil {
		return sqlschema.DatabaseResource{}, err
	}

	bp, _, err := blueprint.LoadForDev(bpPath)
	if err != nil {
		return sqlschema.DatabaseResource{}, fmt.Errorf("loading blueprint: %w", err)
	}

	resources := sqlschema.CollectDatabaseResources(bp, ".")
	if len(resources) == 0 {
		return sqlschema.DatabaseResource{}, fmt.Errorf("no celerity/sqlDatabase resources found in %s", bpPath)
	}

	if databaseFlag != "" {
		for _, r := range resources {
			if r.ResourceName == databaseFlag || r.Name == databaseFlag {
				return r, nil
			}
		}
		return sqlschema.DatabaseResource{}, fmt.Errorf("no celerity/sqlDatabase resource named %q", databaseFlag)
	}

	if len(resources) > 1 {
		names := make([]string, len(resources))
		for i, r := range resources {
			names[i] = r.ResourceName
		}
		return sqlschema.DatabaseResource{}, fmt.Errorf(
			"blueprint defines multiple SQL databases (%v); specify one with --database", names)
	}

	return resources[0], nil
}
