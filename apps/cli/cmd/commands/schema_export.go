package commands

import (
	"fmt"
	"os"

	"github.com/newstack-cloud/celerity/apps/cli/internal/sqlschema"
	"github.com/spf13/cobra"
)

func setupSchemaExportCommand(schemaCmd *cobra.Command) {
	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export a schema as SQL DDL, markdown, JSON Schema or Mermaid ERD",
		Long: `Render a celerity/sqlDatabase resource's schema in a data-team-friendly format.
The export is derived from the schema YAML, so no database connection is needed.

Formats:
  sql          CREATE TABLE / INDEX DDL
  markdown     human-readable tables (columns, keys, classification, descriptions)
  json-schema  a JSON Schema document (one object definition per table)
  mermaid      a Mermaid erDiagram of tables and relationships

Writes to stdout by default, or to --out.

Examples:
  celerity schema export --format markdown
  celerity schema export --format mermaid --out ./docs/orders-erd.mmd
  celerity schema export --format json-schema --out ./schema.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			formatFlag, _ := cmd.Flags().GetString("format")
			out, _ := cmd.Flags().GetString("out")

			format := sqlschema.ExportFormat(formatFlag)
			switch format {
			case sqlschema.ExportSQL, sqlschema.ExportMarkdown, sqlschema.ExportJSONSchema, sqlschema.ExportMermaid:
			default:
				return fmt.Errorf("--format must be one of: sql, markdown, json-schema, mermaid")
			}

			resource, err := resolveDatabaseResource(cmd)
			if err != nil {
				return err
			}
			if resource.SchemaPath == "" {
				return fmt.Errorf("resource %q has no schemaPath configured", resource.ResourceName)
			}

			parsed, err := sqlschema.ParseSchemaFile(resource.SchemaPath)
			if err != nil {
				return fmt.Errorf("parsing schema: %w", err)
			}
			content, err := sqlschema.ExportSchema(parsed, format, resource.ResourceName)
			if err != nil {
				return err
			}

			if out == "" {
				fmt.Fprint(cmd.OutOrStdout(), content)
				return nil
			}
			if err := os.WriteFile(out, []byte(content), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", out, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Exported %s (%s) to %s\n", resource.ResourceName, format, out)
			return nil
		},
	}

	exportCmd.Flags().String("format", "", "Output format: sql, markdown, json-schema or mermaid.")
	exportCmd.Flags().String("out", "", "File to write the export to (default: stdout).")
	schemaCmd.AddCommand(exportCmd)
}
