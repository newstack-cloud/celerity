package sqlschema

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ExportSuite struct {
	suite.Suite
}

func TestExportSuite(t *testing.T) {
	suite.Run(t, new(ExportSuite))
}

func exportSample() *Schema {
	return &Schema{
		Extensions: []string{"uuid-ossp"},
		Tables: map[string]Table{
			"customers": {
				Tags: []string{"core-entity"},
				Columns: map[string]Column{
					"id":    {Type: "bigint", PrimaryKey: true},
					"email": {Type: "varchar(255)", Unique: true},
					"note":  {Type: "text", Nullable: true, Description: "free text", Classification: "internal", Tags: []string{"freeform"}},
				},
			},
			"orders": {
				Description: "customer orders",
				Columns: map[string]Column{
					"id": {Type: "bigint", PrimaryKey: true},
					"customer_id": {
						Type:       "bigint",
						References: &ForeignKey{Table: "customers", Column: "id", OnDelete: "cascade"},
					},
				},
				Indexes: []Index{{Name: "idx_orders_customer", Columns: []string{"customer_id"}}},
			},
		},
	}
}

func (s *ExportSuite) Test_sql_ddl() {
	out, err := ExportSchema(exportSample(), ExportSQL, "ordersDb")
	s.Require().NoError(err)
	s.Assert().Contains(out, `CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`)
	s.Assert().Contains(out, "CREATE TABLE customers (")
	s.Assert().Contains(out, "email varchar(255) NOT NULL UNIQUE")
	s.Assert().Contains(out, "note text")
	s.Assert().Contains(out, "PRIMARY KEY (id)")
	s.Assert().Contains(out, "FOREIGN KEY (customer_id) REFERENCES customers (id) ON DELETE CASCADE")
	s.Assert().Contains(out, "CREATE INDEX idx_orders_customer ON orders (customer_id);")
}

func (s *ExportSuite) Test_markdown() {
	out, err := ExportSchema(exportSample(), ExportMarkdown, "ordersDb")
	s.Require().NoError(err)
	s.Assert().Contains(out, "# ordersDb schema")
	s.Assert().Contains(out, "## customers")
	s.Assert().Contains(out, "| Column | Type | Nullable | Key | Classification | Tags | Description |")
	s.Assert().Contains(out, "| id | bigint | no | PK |")
	s.Assert().Contains(out, "internal")
	s.Assert().Contains(out, "FK → customers.id")
	s.Assert().Contains(out, "customer orders")
	// Table-level and column-level tags are surfaced.
	s.Assert().Contains(out, "Tags: core-entity")
	s.Assert().Contains(out, "freeform")
}

func (s *ExportSuite) Test_json_schema_is_valid_and_structured() {
	out, err := ExportSchema(exportSample(), ExportJSONSchema, "ordersDb")
	s.Require().NoError(err)

	var doc map[string]any
	s.Require().NoError(json.Unmarshal([]byte(out), &doc))
	s.Assert().Equal("ordersDb", doc["title"])

	defs, ok := doc["definitions"].(map[string]any)
	s.Require().True(ok)
	customers, ok := defs["customers"].(map[string]any)
	s.Require().True(ok)
	s.Assert().Equal("object", customers["type"])

	props := customers["properties"].(map[string]any)
	// Nullable column carries a ["string","null"] type union.
	note := props["note"].(map[string]any)
	noteTypes := note["type"].([]any)
	s.Assert().Contains(noteTypes, "null")

	// Non-nullable PK is required.
	required := customers["required"].([]any)
	s.Assert().Contains(required, "id")
	s.Assert().NotContains(required, "note")
}

func (s *ExportSuite) Test_mermaid_erd() {
	out, err := ExportSchema(exportSample(), ExportMermaid, "ordersDb")
	s.Require().NoError(err)
	s.Assert().Contains(out, "erDiagram")
	s.Assert().Contains(out, "customers {")
	s.Assert().Contains(out, "bigint id PK")
	// varchar(255) must not leak parentheses into a Mermaid identifier.
	s.Assert().Contains(out, "varchar email")
	s.Assert().NotContains(out, "varchar(255)")
	// Relationship from the foreign key.
	s.Assert().Contains(out, `customers ||--o{ orders : "customer_id"`)
}

func (s *ExportSuite) Test_unsupported_format() {
	_, err := ExportSchema(exportSample(), ExportFormat("xml"), "ordersDb")
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "unsupported export format")
}
