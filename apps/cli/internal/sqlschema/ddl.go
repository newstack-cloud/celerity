package sqlschema

import (
	"fmt"
	"sort"
	"strings"
)

// DDLResult holds both up (create) and down (drop) DDL statements.
type DDLResult struct {
	Up   []string
	Down []string
}

// GenerateDDL produces ordered up and down DDL statements from a parsed schema.
// The engine parameter is validated (v0: "postgres" only).
//
// Up statements are in dependency order:
// 1. Extensions
// 2. Tables (CREATE TABLE)
// 3. Foreign keys (ALTER TABLE ADD CONSTRAINT)
// 4. Indexes (CREATE INDEX)
// 5. Check constraints
//
// Down statements are in reverse dependency order:
// 1. Check constraints (DROP)
// 2. Indexes (DROP)
// 3. Foreign keys (DROP)
// 4. Tables (DROP) — reverse alphabetical
// 5. Extensions (DROP)
func GenerateDDL(s *Schema, engine string) (*DDLResult, error) {
	if engine != "postgres" {
		return nil, fmt.Errorf("unsupported SQL engine %q (supported: postgres)", engine)
	}

	tableNames := sortedKeys(s.Tables)

	// --- Up statements ---
	var up []string
	up = append(up, generateCreateExtensions(s.Extensions)...)

	for _, tableName := range tableNames {
		table := s.Tables[tableName]
		stmt, err := generateCreateTable(tableName, table)
		if err != nil {
			return nil, fmt.Errorf("table %s: %w", tableName, err)
		}
		up = append(up, stmt)
	}

	up = append(up, generateAddForeignKeys(s.Tables, tableNames)...)
	up = append(up, generateCreateIndexes(s.Tables, tableNames)...)
	up = append(up, generateAddCheckConstraints(s.Tables, tableNames)...)

	// --- Down statements (reverse dependency order) ---
	var down []string
	down = append(down, generateDropCheckConstraints(s.Tables, tableNames)...)
	down = append(down, generateDropIndexes(s.Tables, tableNames)...)
	down = append(down, generateDropForeignKeys(s.Tables, tableNames)...)

	// Drop tables in reverse order.
	for i := len(tableNames) - 1; i >= 0; i-- {
		down = append(down, fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", quoteIdent(tableNames[i])))
	}

	down = append(down, generateDropExtensions(s.Extensions)...)

	return &DDLResult{Up: up, Down: down}, nil
}

// --- Up generators ---

func generateCreateExtensions(extensions []string) []string {
	stmts := make([]string, len(extensions))
	for i, ext := range extensions {
		stmts[i] = fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS %q", ext)
	}
	return stmts
}

func generateCreateTable(name string, table Table) (string, error) {
	if len(table.Columns) == 0 {
		return "", fmt.Errorf("table has no columns")
	}

	colNames := sortedKeys(table.Columns)

	var colDefs []string
	var primaryKeys []string

	for _, colName := range colNames {
		col := table.Columns[colName]
		def := fmt.Sprintf("  %s %s", quoteIdent(colName), col.Type)

		if col.PrimaryKey {
			primaryKeys = append(primaryKeys, quoteIdent(colName))
		}

		if !col.Nullable && !col.PrimaryKey {
			def += " NOT NULL"
		}

		if col.Unique {
			def += " UNIQUE"
		}

		if col.Default != "" {
			def += " DEFAULT " + col.Default
		}

		colDefs = append(colDefs, def)
	}

	if len(primaryKeys) > 0 {
		colDefs = append(colDefs, fmt.Sprintf("  PRIMARY KEY (%s)", strings.Join(primaryKeys, ", ")))
	}

	return fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s (\n%s\n)",
		quoteIdent(name),
		strings.Join(colDefs, ",\n"),
	), nil
}

func generateAddForeignKeys(tables map[string]Table, tableNames []string) []string {
	var stmts []string
	for _, tableName := range tableNames {
		table := tables[tableName]
		colNames := sortedKeys(table.Columns)
		for _, colName := range colNames {
			col := table.Columns[colName]
			if col.References == nil {
				continue
			}
			fkName := fkConstraintName(tableName, colName)
			stmt := fmt.Sprintf(
				"ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s(%s)",
				quoteIdent(tableName),
				quoteIdent(fkName),
				quoteIdent(colName),
				quoteIdent(col.References.Table),
				quoteIdent(col.References.Column),
			)
			if col.References.OnDelete != "" {
				stmt += " ON DELETE " + strings.ToUpper(col.References.OnDelete)
			}
			stmts = append(stmts, stmt)
		}
	}
	return stmts
}

func generateCreateIndexes(tables map[string]Table, tableNames []string) []string {
	var stmts []string
	for _, tableName := range tableNames {
		table := tables[tableName]
		for _, idx := range table.Indexes {
			uniqueStr := ""
			if idx.Unique {
				uniqueStr = "UNIQUE "
			}
			quotedCols := make([]string, len(idx.Columns))
			for i, c := range idx.Columns {
				quotedCols[i] = quoteIdent(c)
			}
			stmts = append(stmts, fmt.Sprintf(
				"CREATE %sINDEX IF NOT EXISTS %s ON %s (%s)",
				uniqueStr,
				quoteIdent(idx.Name),
				quoteIdent(tableName),
				strings.Join(quotedCols, ", "),
			))
		}
	}
	return stmts
}

func generateAddCheckConstraints(tables map[string]Table, tableNames []string) []string {
	var stmts []string
	for _, tableName := range tableNames {
		table := tables[tableName]
		for _, c := range table.Constraints {
			if c.Type != "check" || c.Expression == "" {
				continue
			}
			stmts = append(stmts, fmt.Sprintf(
				"ALTER TABLE %s ADD CONSTRAINT %s CHECK (%s)",
				quoteIdent(tableName),
				quoteIdent(c.Name),
				c.Expression,
			))
		}
	}
	return stmts
}

// --- Down generators ---

func generateDropExtensions(extensions []string) []string {
	stmts := make([]string, len(extensions))
	// Drop in reverse order.
	for i, ext := range extensions {
		stmts[len(extensions)-1-i] = fmt.Sprintf("DROP EXTENSION IF EXISTS %q CASCADE", ext)
	}
	return stmts
}

func generateDropForeignKeys(tables map[string]Table, tableNames []string) []string {
	var stmts []string
	// Reverse table order for dropping.
	for i := len(tableNames) - 1; i >= 0; i-- {
		tableName := tableNames[i]
		table := tables[tableName]
		colNames := sortedKeys(table.Columns)
		for _, colName := range colNames {
			col := table.Columns[colName]
			if col.References == nil {
				continue
			}
			fkName := fkConstraintName(tableName, colName)
			stmts = append(stmts, fmt.Sprintf(
				"ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s",
				quoteIdent(tableName),
				quoteIdent(fkName),
			))
		}
	}
	return stmts
}

func generateDropIndexes(tables map[string]Table, tableNames []string) []string {
	var stmts []string
	for i := len(tableNames) - 1; i >= 0; i-- {
		table := tables[tableNames[i]]
		for _, idx := range table.Indexes {
			stmts = append(stmts, fmt.Sprintf("DROP INDEX IF EXISTS %s", quoteIdent(idx.Name)))
		}
	}
	return stmts
}

func generateDropCheckConstraints(tables map[string]Table, tableNames []string) []string {
	var stmts []string
	for i := len(tableNames) - 1; i >= 0; i-- {
		tableName := tableNames[i]
		table := tables[tableName]
		for _, c := range table.Constraints {
			if c.Type != "check" || c.Name == "" {
				continue
			}
			stmts = append(stmts, fmt.Sprintf(
				"ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s",
				quoteIdent(tableName),
				quoteIdent(c.Name),
			))
		}
	}
	return stmts
}

// --- Helpers ---

func fkConstraintName(tableName, colName string) string {
	return fmt.Sprintf("fk_%s_%s", tableName, colName)
}

// quoteIdent quotes a SQL identifier with double quotes.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
