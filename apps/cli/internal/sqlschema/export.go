package sqlschema

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExportFormat is an output format for `schema export`.
type ExportFormat string

const (
	ExportSQL        ExportFormat = "sql"
	ExportMarkdown   ExportFormat = "markdown"
	ExportJSONSchema ExportFormat = "json-schema"
	ExportMermaid    ExportFormat = "mermaid"
)

// ExportSchema renders a parsed schema in the given format for data-team
// consumption. resourceName is used in headers/titles. The export is derived
// purely from the schema YAML, so it needs no database connection.
func ExportSchema(s *Schema, format ExportFormat, resourceName string) (string, error) {
	switch format {
	case ExportSQL:
		return exportSQL(s), nil
	case ExportMarkdown:
		return exportMarkdown(s, resourceName), nil
	case ExportJSONSchema:
		return exportJSONSchema(s, resourceName)
	case ExportMermaid:
		return exportMermaid(s), nil
	default:
		return "", fmt.Errorf("unsupported export format %q (supported: sql, markdown, json-schema, mermaid)", format)
	}
}

func exportSQL(s *Schema) string {
	var b strings.Builder
	b.WriteString("-- Exported by `celerity schema export --format sql`\n")

	for _, ext := range s.Extensions {
		fmt.Fprintf(&b, "CREATE EXTENSION IF NOT EXISTS %q;\n", ext)
	}
	if len(s.Extensions) > 0 {
		b.WriteString("\n")
	}

	for _, name := range sortedKeys(s.Tables) {
		table := s.Tables[name]
		fmt.Fprintf(&b, "CREATE TABLE %s (\n", name)

		var lines []string
		for _, colName := range orderedColumns(table) {
			lines = append(lines, "  "+columnDDL(colName, table.Columns[colName]))
		}
		if pk := primaryKeyColumns(table); len(pk) > 0 {
			lines = append(lines, fmt.Sprintf("  PRIMARY KEY (%s)", strings.Join(pk, ", ")))
		}
		for _, colName := range orderedColumns(table) {
			col := table.Columns[colName]
			if col.References != nil {
				lines = append(lines, fmt.Sprintf(
					"  FOREIGN KEY (%s) REFERENCES %s (%s)%s",
					colName, col.References.Table, col.References.Column, onDeleteClause(col.References.OnDelete)))
			}
		}
		for _, c := range table.Constraints {
			if c.Type == "check" && c.Expression != "" {
				lines = append(lines, fmt.Sprintf("  CONSTRAINT %s CHECK (%s)", c.Name, c.Expression))
			}
		}
		b.WriteString(strings.Join(lines, ",\n"))
		b.WriteString("\n);\n")

		for _, idx := range table.Indexes {
			unique := ""
			if idx.Unique {
				unique = "UNIQUE "
			}
			fmt.Fprintf(&b, "CREATE %sINDEX %s ON %s (%s);\n",
				unique, idx.Name, name, strings.Join(idx.Columns, ", "))
		}
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

func columnDDL(name string, col Column) string {
	parts := []string{name, col.Type}
	if !col.Nullable || col.PrimaryKey {
		parts = append(parts, "NOT NULL")
	}
	if col.Unique {
		parts = append(parts, "UNIQUE")
	}
	if col.Default != "" {
		parts = append(parts, "DEFAULT "+col.Default)
	}
	return strings.Join(parts, " ")
}

func onDeleteClause(onDelete string) string {
	if onDelete == "" {
		return ""
	}
	return " ON DELETE " + strings.ToUpper(onDelete)
}

func primaryKeyColumns(t Table) []string {
	var pk []string
	for _, name := range orderedColumns(t) {
		if t.Columns[name].PrimaryKey {
			pk = append(pk, name)
		}
	}
	return pk
}

func exportMarkdown(s *Schema, resourceName string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s schema\n", resourceName)

	for _, name := range sortedKeys(s.Tables) {
		table := s.Tables[name]
		fmt.Fprintf(&b, "\n## %s\n", name)
		if table.Description != "" {
			fmt.Fprintf(&b, "\n%s\n", table.Description)
		}
		if len(table.Tags) > 0 {
			fmt.Fprintf(&b, "\nTags: %s\n", strings.Join(table.Tags, ", "))
		}
		b.WriteString("\n| Column | Type | Nullable | Key | Classification | Tags | Description |\n")
		b.WriteString("|---|---|---|---|---|---|---|\n")
		for _, colName := range orderedColumns(table) {
			col := table.Columns[colName]
			nullable := "no"
			if col.Nullable && !col.PrimaryKey {
				nullable = "yes"
			}
			key := ""
			switch {
			case col.PrimaryKey:
				key = "PK"
			case col.References != nil:
				key = fmt.Sprintf("FK → %s.%s", col.References.Table, col.References.Column)
			case col.Unique:
				key = "unique"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s |\n",
				colName, col.Type, nullable, key, col.Classification, strings.Join(col.Tags, ", "), col.Description)
		}

		if len(table.Indexes) > 0 {
			b.WriteString("\nIndexes:\n")
			for _, idx := range table.Indexes {
				unique := ""
				if idx.Unique {
					unique = " (unique)"
				}
				fmt.Fprintf(&b, "- `%s`%s on (%s)\n", idx.Name, unique, strings.Join(idx.Columns, ", "))
			}
		}
	}

	return b.String()
}

// A minimal JSON Schema document with one object definition per table.
type jsonSchemaDoc struct {
	Schema      string                    `json:"$schema"`
	Title       string                    `json:"title"`
	Definitions map[string]jsonSchemaType `json:"definitions"`
}

type jsonSchemaType struct {
	Type                 string                    `json:"type"`
	Description          string                    `json:"description,omitempty"`
	Properties           map[string]jsonSchemaProp `json:"properties"`
	Required             []string                  `json:"required,omitempty"`
	AdditionalProperties bool                      `json:"additionalProperties"`
}

type jsonSchemaProp struct {
	Type        []string `json:"type"`
	Description string   `json:"description,omitempty"`
}

func exportJSONSchema(s *Schema, resourceName string) (string, error) {
	doc := jsonSchemaDoc{
		Schema:      "http://json-schema.org/draft-07/schema#",
		Title:       resourceName,
		Definitions: map[string]jsonSchemaType{},
	}

	for _, name := range sortedKeys(s.Tables) {
		table := s.Tables[name]
		def := jsonSchemaType{
			Type:                 "object",
			Description:          table.Description,
			Properties:           map[string]jsonSchemaProp{},
			AdditionalProperties: false,
		}
		for _, colName := range orderedColumns(table) {
			col := table.Columns[colName]
			jsonType := jsonSchemaColumnType(col.Type)
			types := []string{jsonType}
			if col.Nullable && !col.PrimaryKey {
				types = append(types, "null")
			} else {
				def.Required = append(def.Required, colName)
			}
			def.Properties[colName] = jsonSchemaProp{Type: types, Description: col.Description}
		}
		doc.Definitions[name] = def
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding json schema: %w", err)
	}
	return string(out) + "\n", nil
}

func jsonSchemaColumnType(sqlType string) string {
	switch baseSQLType(sqlType) {
	case "smallint", "int2", "integer", "int", "int4", "serial", "serial4",
		"bigint", "int8", "bigserial", "serial8":
		return "integer"
	case "numeric", "decimal", "real", "float4", "double precision", "float8", "money":
		return "number"
	case "boolean", "bool":
		return "boolean"
	case "json", "jsonb":
		return "object"
	default:
		return "string"
	}
}

func exportMermaid(s *Schema) string {
	var b strings.Builder
	b.WriteString("erDiagram\n")

	for _, name := range sortedKeys(s.Tables) {
		table := s.Tables[name]
		fmt.Fprintf(&b, "  %s {\n", name)
		for _, colName := range orderedColumns(table) {
			col := table.Columns[colName]
			key := ""
			switch {
			case col.PrimaryKey:
				key = "PK"
			case col.References != nil:
				key = "FK"
			}
			// Mermaid identifiers cannot contain parentheses; use the base type.
			line := fmt.Sprintf("    %s %s", mermaidType(col.Type), colName)
			if key != "" {
				line += " " + key
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("  }\n")
	}

	for _, name := range sortedKeys(s.Tables) {
		for _, colName := range orderedColumns(s.Tables[name]) {
			col := s.Tables[name].Columns[colName]
			if col.References != nil {
				fmt.Fprintf(&b, "  %s ||--o{ %s : %q\n", col.References.Table, name, colName)
			}
		}
	}

	return b.String()
}

func mermaidType(sqlType string) string {
	base := baseSQLType(sqlType)
	return strings.ReplaceAll(base, " ", "_")
}
