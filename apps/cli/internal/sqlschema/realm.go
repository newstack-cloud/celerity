package sqlschema

import (
	"fmt"
	"sort"
	"strings"

	atlasschema "ariga.io/atlas/sql/schema"
)

// The name the desired schema is built with, matching Postgres's "public". For
// engines whose managed schema is the database itself (MySQL), DB-backed callers
// override the built schema's name with the real one before diffing.
const defaultSchemaName = "public"

// desiredSchema maps a parsed Celerity schema YAML to an Atlas schema object
// (the desired end state) for the given engine.
//
// A column is NOT NULL unless it declares `nullable: true`, and primary-key
// columns are always NOT NULL.
func desiredSchema(s *Schema, engine string) (*atlasschema.Schema, error) {
	d, err := dialectForEngine(engine)
	if err != nil {
		return nil, err
	}

	sch := atlasschema.New(defaultSchemaName)
	tables := map[string]*atlasschema.Table{}
	columns := map[string]map[string]*atlasschema.Column{}

	// Step 1: tables, columns, primary keys, unique columns, indexes, checks.
	for _, tableName := range sortedKeys(s.Tables) {
		at, cols, err := buildTable(d, tableName, s.Tables[tableName])
		if err != nil {
			return nil, fmt.Errorf("table %s: %w", tableName, err)
		}
		sch.AddTables(at)
		tables[tableName] = at
		columns[tableName] = cols
	}

	// Step 2: foreign keys (need the referenced tables/columns to exist first).
	for _, tableName := range sortedKeys(s.Tables) {
		if err := addForeignKeys(tableName, s.Tables[tableName], tables, columns); err != nil {
			return nil, fmt.Errorf("table %s: %w", tableName, err)
		}
	}

	return sch, nil
}

func buildTable(d dialect, name string, tbl Table) (*atlasschema.Table, map[string]*atlasschema.Column, error) {
	at := atlasschema.NewTable(name)
	cols := map[string]*atlasschema.Column{}
	var pk []*atlasschema.Column

	for _, colName := range sortedKeys(tbl.Columns) {
		col := tbl.Columns[colName]
		colType, err := d.ParseType(col.Type)
		if err != nil {
			return nil, nil, fmt.Errorf("column %s: parsing type %q: %w", colName, col.Type, err)
		}

		ac := atlasschema.NewColumn(colName).SetType(colType)
		ac.SetNull(col.Nullable && !col.PrimaryKey)
		if col.Default != "" {
			ac.SetDefault(&atlasschema.RawExpr{X: col.Default})
		}
		at.AddColumns(ac)
		cols[colName] = ac
		if col.PrimaryKey {
			pk = append(pk, ac)
		}
	}

	if len(pk) > 0 {
		at.SetPrimaryKey(atlasschema.NewPrimaryKey(pk...))
	}

	// Column-level UNIQUE becomes a unique index using Postgres's default
	// naming convention, so it round-trips against an inspected database.
	for _, colName := range sortedKeys(tbl.Columns) {
		if tbl.Columns[colName].Unique {
			idxName := fmt.Sprintf("%s_%s_key", name, colName)
			at.AddIndexes(atlasschema.NewUniqueIndex(idxName).AddColumns(cols[colName]))
		}
	}

	for _, idx := range tbl.Indexes {
		ix := atlasschema.NewIndex(idx.Name)
		if idx.Unique {
			ix = atlasschema.NewUniqueIndex(idx.Name)
		}
		for _, c := range idx.Columns {
			col, ok := cols[c]
			if !ok {
				return nil, nil, fmt.Errorf("index %s references unknown column %q", idx.Name, c)
			}
			ix.AddColumns(col)
		}
		typeAttr, err := d.IndexTypeAttr(idx.Type)
		if err != nil {
			return nil, nil, fmt.Errorf("index %s: %w", idx.Name, err)
		}
		if typeAttr != nil {
			ix.AddAttrs(typeAttr)
		}
		at.AddIndexes(ix)
	}

	for _, c := range tbl.Constraints {
		if c.Type != "check" || c.Expression == "" {
			continue
		}
		at.AddChecks(atlasschema.NewCheck().SetName(c.Name).SetExpr(c.Expression))
	}

	return at, cols, nil
}

func addForeignKeys(
	tableName string,
	tbl Table,
	tables map[string]*atlasschema.Table,
	columns map[string]map[string]*atlasschema.Column,
) error {
	for _, colName := range sortedKeys(tbl.Columns) {
		col := tbl.Columns[colName]
		if col.References == nil {
			continue
		}

		refTable, ok := tables[col.References.Table]
		if !ok {
			return fmt.Errorf("column %s references unknown table %q", colName, col.References.Table)
		}
		refCol, ok := columns[col.References.Table][col.References.Column]
		if !ok {
			return fmt.Errorf("column %s references unknown column %s.%s", colName, col.References.Table, col.References.Column)
		}

		fk := atlasschema.NewForeignKey(fkConstraintName(tableName, colName)).
			AddColumns(columns[tableName][colName]).
			SetRefTable(refTable).
			AddRefColumns(refCol)
		if col.References.OnDelete != "" {
			fk.SetOnDelete(atlasschema.ReferenceOption(strings.ToUpper(col.References.OnDelete)))
		}
		tables[tableName].AddForeignKeys(fk)
	}
	return nil
}

func fkConstraintName(tableName, colName string) string {
	return fmt.Sprintf("fk_%s_%s", tableName, colName)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
