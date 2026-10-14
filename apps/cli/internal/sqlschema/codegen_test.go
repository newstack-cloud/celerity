package sqlschema

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type CodegenSuite struct {
	suite.Suite
}

func TestCodegenSuite(t *testing.T) {
	suite.Run(t, new(CodegenSuite))
}

func sampleSchema() *Schema {
	return &Schema{
		Tables: map[string]Table{
			"customers": {
				Description: "customers table row",
				Columns: map[string]Column{
					"id":         {Type: "bigint", PrimaryKey: true},
					"email":      {Type: "varchar(255)", Unique: true},
					"tier":       {Type: "integer"},
					"created_at": {Type: "timestamptz"},
					"meta":       {Type: "jsonb", Nullable: true},
				},
			},
			"order_events": {
				Columns: map[string]Column{
					"id": {Type: "bigint", PrimaryKey: true},
					"customer_id": {
						Type:       "bigint",
						References: &ForeignKey{Table: "customers", Column: "id", OnDelete: "cascade"},
					},
				},
			},
		},
	}
}

func (s *CodegenSuite) Test_typescript_row_interface_and_types() {
	file, err := GenerateCode(sampleSchema(), LangTypeScript, "ordersDb")
	s.Require().NoError(err)
	s.Assert().Equal("orders-db.ts", file.Name)

	c := file.Content
	// Singular entity name, no Row suffix.
	s.Assert().Contains(c, "export type Customer = {")
	// bigint → string (JS precision), integer → number, timestamptz → Date, jsonb → unknown.
	s.Assert().Contains(c, "id: string;")
	s.Assert().Contains(c, "tier: number;")
	s.Assert().Contains(c, "created_at: Date;")
	// Nullable non-PK column gets a `| null`.
	s.Assert().Contains(c, "meta: unknown | null;")
	// Singularised + PascalCase for snake_case table names.
	s.Assert().Contains(c, "export type OrderEvent = {")
	// In the constants, Tables keys stay the actual (plural) table names.
	s.Assert().Contains(c, "export const Tables = {")
	s.Assert().Contains(c, `customers: "customers",`)
	s.Assert().Contains(c, "export const CustomerColumns = {")
	s.Assert().Contains(c, `email: "email",`)
	s.Assert().Contains(c, "do not edit")
}

func (s *CodegenSuite) Test_typescript_primary_key_column_first() {
	file, err := GenerateCode(sampleSchema(), LangTypeScript, "ordersDb")
	s.Require().NoError(err)
	// id (PK) must appear before the alphabetically-earlier created_at.
	s.Assert().Less(index(file.Content, "id: string;"), index(file.Content, "created_at: Date;"))
}

func (s *CodegenSuite) Test_python_pydantic_models_and_imports() {
	file, err := GenerateCode(sampleSchema(), LangPython, "ordersDb")
	s.Require().NoError(err)
	s.Assert().Equal("orders_db.py", file.Name)

	c := file.Content
	s.Assert().Contains(c, "from pydantic import BaseModel")
	s.Assert().Contains(c, "from datetime import datetime")
	s.Assert().Contains(c, "from typing import Any, Optional")
	s.Assert().Contains(c, "class Customer(BaseModel):")
	s.Assert().Contains(c, "class OrderEvent(BaseModel):")
	s.Assert().Contains(c, "id: str")
	s.Assert().Contains(c, "tier: int")
	s.Assert().Contains(c, "created_at: datetime")
	s.Assert().Contains(c, "meta: Optional[Any]")
}

func (s *CodegenSuite) Test_deterministic_output() {
	a, err := GenerateCode(sampleSchema(), LangTypeScript, "ordersDb")
	s.Require().NoError(err)
	b, err := GenerateCode(sampleSchema(), LangTypeScript, "ordersDb")
	s.Require().NoError(err)
	s.Assert().Equal(a.Content, b.Content, "codegen must be deterministic for the freshness check")
}

func (s *CodegenSuite) Test_unsupported_language() {
	_, err := GenerateCode(sampleSchema(), CodegenLanguage("ruby"), "ordersDb")
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "unsupported codegen language")
}

func (s *CodegenSuite) Test_singularise_rules() {
	cases := map[string]string{
		"customers":    "customer",
		"orders":       "order",
		"events":       "event",
		"companies":    "company",
		"categories":   "category",
		"addresses":    "address",
		"boxes":        "box",
		"status":       "status", // -us: unchanged
		"series":       "series", // -is: unchanged (heuristic keeps it)
		"people":       "person", // irregular
		"class":        "class",  // -ss: unchanged
		"order_events": "order_event",
	}
	for plural, want := range cases {
		// order_events is a phrase; singularise operates on the last word, so
		// exercise the whole pipeline for it.
		if plural == "order_events" {
			s.Assert().Equal("OrderEvent", entityTypeName(plural, Table{}))
			continue
		}
		s.Assert().Equalf(want, singularise(plural), "singularise(%q)", plural)
	}
}

func (s *CodegenSuite) Test_type_name_override() {
	schema := &Schema{
		Tables: map[string]Table{
			"people": {
				TypeName: "Person",
				Columns:  map[string]Column{"id": {Type: "bigint", PrimaryKey: true}},
			},
		},
	}
	file, err := GenerateCode(schema, LangTypeScript, "hrDb")
	s.Require().NoError(err)
	s.Assert().Contains(file.Content, "export type Person = {")
	s.Assert().Contains(file.Content, "export const PersonColumns = {")
}

func (s *CodegenSuite) Test_identifier_case_helpers() {
	s.Assert().Equal("OrderEvents", pascalCase("order_events"))
	s.Assert().Equal("OrdersDb", pascalCase("ordersDb"))
	s.Assert().Equal("orders-db", kebabCase("ordersDb"))
	s.Assert().Equal("orders_db", snakeCase("ordersDb"))
	s.Assert().Equal("orders-db", kebabCase("orders_db"))
}

func (s *CodegenSuite) Test_type_mapping_edges() {
	s.Assert().Equal("string", tsType("VARCHAR(255)"))
	s.Assert().Equal("string", tsType("numeric(10,2)"))
	s.Assert().Equal("boolean", tsType("bool"))
	s.Assert().Equal("str", pyType("uuid"))
	s.Assert().Equal("float", pyType("double precision"))
}

func index(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
