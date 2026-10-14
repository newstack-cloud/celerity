package sqlschema

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	atlasschema "ariga.io/atlas/sql/schema"
)

// MigrationRecord is a single applied migration read from the revision history.
type MigrationRecord struct {
	Version       string
	Description   string
	AppliedAt     time.Time
	ExecutionTime time.Duration
	Failed        bool
	Error         string
}

// ReadMigrationHistory returns the applied-migration history recorded in the
// database's atlas_schema_revisions table, ordered by version.
func ReadMigrationHistory(ctx context.Context, a *Applier) ([]MigrationRecord, error) {
	rrw, err := a.dialect.NewRevisions(ctx, a.db)
	if err != nil {
		return nil, err
	}
	revisions, err := rrw.ReadRevisions(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading migration history: %w", err)
	}

	records := make([]MigrationRecord, 0, len(revisions))
	for _, r := range revisions {
		records = append(records, MigrationRecord{
			Version:       r.Version,
			Description:   r.Description,
			AppliedAt:     r.ExecutedAt,
			ExecutionTime: r.ExecutionTime,
			Failed:        r.Error != "",
			Error:         r.Error,
		})
	}
	return records, nil
}

// RenderMigrationHistory writes a human-readable migration history table.
func RenderMigrationHistory(w io.Writer, records []MigrationRecord) {
	if len(records) == 0 {
		fmt.Fprintln(w, "No migrations have been applied.")
		return
	}
	for _, r := range records {
		status := "ok"
		if r.Failed {
			status = "FAILED"
		}
		fmt.Fprintf(w, "%s  %-40s  applied %s  (%s)  [%s]\n",
			r.Version, truncate(r.Description, 40), r.AppliedAt.UTC().Format(time.RFC3339),
			r.ExecutionTime.Round(time.Millisecond), status)
		if r.Failed {
			fmt.Fprintf(w, "    error: %s\n", r.Error)
		}
	}
}

// InspectCurrentSchema reads the live database's public schema and renders it as
// a readable summary of the current deployed state.
func InspectCurrentSchema(ctx context.Context, a *Applier) (string, error) {
	drv, err := a.atlasDriver()
	if err != nil {
		return "", err
	}
	name, err := a.dialect.SchemaName(ctx, a.db)
	if err != nil {
		return "", err
	}
	sch, err := drv.InspectSchema(ctx, name, nil)
	if err != nil {
		return "", fmt.Errorf("inspecting current schema: %w", err)
	}
	return renderInspectedSchema(sch), nil
}

func renderInspectedSchema(sch *atlasschema.Schema) string {
	var b strings.Builder
	fmt.Fprintf(&b, "schema %q (current deployed state):\n", sch.Name)

	for _, table := range sch.Tables {
		// Celerity's own revision-history table is not part of the schema the
		// author describes, so it is left out of the summary.
		if table.Name == revisionsTable {
			continue
		}
		fmt.Fprintf(&b, "\n  %s\n", table.Name)
		pk := primaryKeySet(table)
		for _, col := range table.Columns {
			null := "NOT NULL"
			if col.Type != nil && col.Type.Null {
				null = "NULL"
			}
			marker := ""
			if pk[col.Name] {
				marker = "  PK"
			}
			fmt.Fprintf(&b, "    %-24s %-20s %s%s\n", col.Name, columnRawType(col), null, marker)
		}
		for _, fk := range table.ForeignKeys {
			fmt.Fprintf(&b, "    → FK %s references %s\n", fkColumnNames(fk), fkRefTableName(fk))
		}
	}
	return b.String()
}

func primaryKeySet(t *atlasschema.Table) map[string]bool {
	set := map[string]bool{}
	if t.PrimaryKey != nil {
		for _, part := range t.PrimaryKey.Parts {
			if part.C != nil {
				set[part.C.Name] = true
			}
		}
	}
	return set
}

func columnRawType(c *atlasschema.Column) string {
	if c.Type == nil || c.Type.Raw == "" {
		return "?"
	}
	return c.Type.Raw
}

func fkColumnNames(fk *atlasschema.ForeignKey) string {
	names := make([]string, 0, len(fk.Columns))
	for _, c := range fk.Columns {
		names = append(names, c.Name)
	}
	return strings.Join(names, ", ")
}

func fkRefTableName(fk *atlasschema.ForeignKey) string {
	if fk.RefTable == nil {
		return "?"
	}
	return fk.RefTable.Name
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
