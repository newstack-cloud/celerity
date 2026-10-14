package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/newstack-cloud/celerity/apps/cli/internal/sqlschema"
	"github.com/spf13/cobra"
)

func setupSchemaCodegenCommand(schemaCmd *cobra.Command) {
	codegenCmd := &cobra.Command{
		Use:   "codegen",
		Short: "Generate type-safe code from schema definitions",
		Long: `Generate row types and column/table constants from the schema YAML of every
celerity/sqlDatabase resource, written into the --out directory (one file per
database). There is no ORM coupling, so the types combine with any library.

Supported languages (v0): typescript (interfaces + constants), python (Pydantic
models).

In --check mode nothing is written; the command exits non-zero if the generated
code on disk is out of date, so it can gate CI.

Examples:
  celerity schema codegen --lang typescript --out ./src/generated
  celerity schema codegen --lang python --out ./src/generated
  celerity schema codegen --lang typescript --out ./src/generated --check`,
		RunE: func(cmd *cobra.Command, args []string) error {
			langFlag, _ := cmd.Flags().GetString("lang")
			out, _ := cmd.Flags().GetString("out")
			check, _ := cmd.Flags().GetBool("check")

			lang := sqlschema.CodegenLanguage(langFlag)
			if lang != sqlschema.LangTypeScript && lang != sqlschema.LangPython {
				return fmt.Errorf("--lang must be one of: typescript, python")
			}
			if out == "" {
				return fmt.Errorf("--out is required (the directory to write generated code into)")
			}

			resources, err := loadDatabaseResources(cmd)
			if err != nil {
				return err
			}

			stdout := cmd.OutOrStdout()
			var stale []string
			for _, r := range resources {
				file, err := generateResourceCode(r, lang)
				if err != nil {
					return err
				}
				target := filepath.Join(out, file.Name)

				if check {
					fresh, err := codegenUpToDate(target, file.Content)
					if err != nil {
						return err
					}
					if !fresh {
						stale = append(stale, target)
					}
					continue
				}

				if err := writeGeneratedFile(target, file.Content); err != nil {
					return err
				}
				fmt.Fprintf(stdout, "Generated %s\n", target)
			}

			if check {
				return reportCodegenCheck(stdout, stale)
			}
			return nil
		},
	}

	codegenCmd.Flags().String("lang", "", "Target language: typescript or python.")
	codegenCmd.Flags().String("out", "", "Directory to write generated code into (required).")
	codegenCmd.Flags().Bool("check", false, "Do not write; exit non-zero if generated code is out of date (CI gate).")
	schemaCmd.AddCommand(codegenCmd)
}

func generateResourceCode(r sqlschema.DatabaseResource, lang sqlschema.CodegenLanguage) (*sqlschema.GeneratedFile, error) {
	if r.SchemaPath == "" {
		return nil, fmt.Errorf("resource %q has no schemaPath configured", r.ResourceName)
	}
	parsed, err := sqlschema.ParseSchemaFile(r.SchemaPath)
	if err != nil {
		return nil, fmt.Errorf("parsing schema for %s: %w", r.ResourceName, err)
	}
	return sqlschema.GenerateCode(parsed, lang, r.ResourceName)
}

func writeGeneratedFile(target, content string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", target, err)
	}
	return nil
}

func codegenUpToDate(target, content string) (bool, error) {
	existing, err := os.ReadFile(target)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading %s: %w", target, err)
	}
	return string(existing) == content, nil
}

func reportCodegenCheck(out io.Writer, stale []string) error {
	if len(stale) > 0 {
		for _, path := range stale {
			fmt.Fprintf(out, "✗ %s is out of date\n", path)
		}
		return fmt.Errorf("%d generated file(s) out of date; run `celerity schema codegen` and commit the result", len(stale))
	}
	fmt.Fprintln(out, "Generated code is up to date.")
	return nil
}
