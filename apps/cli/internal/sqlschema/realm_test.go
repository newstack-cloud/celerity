package sqlschema

import (
	"testing"

	"ariga.io/atlas/sql/postgres"
	atlasschema "ariga.io/atlas/sql/schema"
	"github.com/stretchr/testify/suite"
)

type RealmTestSuite struct {
	suite.Suite
}

func TestRealmTestSuite(t *testing.T) {
	suite.Run(t, new(RealmTestSuite))
}

func (s *RealmTestSuite) Test_maps_columns_and_nullability() {
	schema := &Schema{
		Tables: map[string]Table{
			"users": {
				Columns: map[string]Column{
					// Primary key: always NOT NULL even though nullable defaults false.
					"id": {Type: "bigint", PrimaryKey: true},
					// Default (nullable false) → NOT NULL.
					"email": {Type: "varchar(255)"},
					// Explicitly nullable.
					"nickname": {Type: "varchar(100)", Nullable: true},
				},
			},
		},
	}

	sch, err := desiredSchema(schema, "postgres")
	s.Require().NoError(err)

	tbl, ok := sch.Table("users")
	s.Require().True(ok)

	id, ok := tbl.Column("id")
	s.Require().True(ok)
	s.Assert().False(id.Type.Null, "primary key must be NOT NULL")

	email, ok := tbl.Column("email")
	s.Require().True(ok)
	s.Assert().False(email.Type.Null, "non-nullable column must be NOT NULL")

	nickname, ok := tbl.Column("nickname")
	s.Require().True(ok)
	s.Assert().True(nickname.Type.Null, "nullable:true column must allow NULL")

	s.Require().NotNil(tbl.PrimaryKey)
}

func (s *RealmTestSuite) Test_resolves_foreign_keys() {
	schema := &Schema{
		Tables: map[string]Table{
			"customers": {Columns: map[string]Column{"id": {Type: "bigint", PrimaryKey: true}}},
			"orders": {
				Columns: map[string]Column{
					"id":          {Type: "bigint", PrimaryKey: true},
					"customer_id": {Type: "bigint", References: &ForeignKey{Table: "customers", Column: "id", OnDelete: "cascade"}},
				},
			},
		},
	}

	sch, err := desiredSchema(schema, "postgres")
	s.Require().NoError(err)

	orders, ok := sch.Table("orders")
	s.Require().True(ok)
	s.Require().Len(orders.ForeignKeys, 1)
	s.Assert().Equal("customers", orders.ForeignKeys[0].RefTable.Name)
}

func (s *RealmTestSuite) Test_unknown_fk_target_errors() {
	schema := &Schema{
		Tables: map[string]Table{
			"orders": {
				Columns: map[string]Column{
					"customer_id": {Type: "bigint", References: &ForeignKey{Table: "missing", Column: "id"}},
				},
			},
		},
	}
	_, err := desiredSchema(schema, "postgres")
	s.Assert().ErrorContains(err, "unknown table")
}

func (s *RealmTestSuite) Test_unsupported_engine_errors() {
	schema := &Schema{Tables: map[string]Table{"users": {Columns: map[string]Column{"id": {Type: "bigint"}}}}}
	_, err := desiredSchema(schema, "oracle")
	s.Assert().ErrorContains(err, "unsupported SQL engine")
}

func (s *RealmTestSuite) Test_mysql_engine_builds_schema() {
	enableMySQLForTest(s.T())
	schema := &Schema{Tables: map[string]Table{
		"users": {Columns: map[string]Column{
			"id":   {Type: "bigint", PrimaryKey: true},
			"name": {Type: "varchar(100)"},
		}},
	}}
	sch, err := desiredSchema(schema, "mysql")
	s.Require().NoError(err)
	s.Require().Len(sch.Tables, 1)
	s.Assert().Equal("users", sch.Tables[0].Name)
}

func (s *RealmTestSuite) Test_index_type_maps_to_the_engine_attribute() {
	schema := &Schema{
		Tables: map[string]Table{
			"audit_log": {
				Columns: map[string]Column{
					"id":      {Type: "bigint", PrimaryKey: true},
					"details": {Type: "jsonb"},
				},
				Indexes: []Index{
					{Name: "idx_audit_details", Columns: []string{"details"}, Type: "gin"},
				},
			},
		},
	}

	sch, err := desiredSchema(schema, "postgres")
	s.Require().NoError(err)

	tbl, ok := sch.Table("audit_log")
	s.Require().True(ok)
	idx, ok := tbl.Index("idx_audit_details")
	s.Require().True(ok)

	typ := indexTypeAttr(idx.Attrs)
	s.Require().NotNil(typ, "index must carry an index-type attribute")
	s.Assert().Equal(postgres.IndexTypeGIN, typ.T)
}

func (s *RealmTestSuite) Test_btree_is_left_implicit() {
	// Atlas defaults an absent attribute to btree. Spelling out the default must
	// produce the same desired state as omitting it, or every diff would churn.
	explicit := &Schema{
		Tables: map[string]Table{
			"users": {
				Columns: map[string]Column{"email": {Type: "varchar(255)"}},
				Indexes: []Index{{Name: "idx_users_email", Columns: []string{"email"}, Type: "btree"}},
			},
		},
	}
	implicit := &Schema{
		Tables: map[string]Table{
			"users": {
				Columns: map[string]Column{"email": {Type: "varchar(255)"}},
				Indexes: []Index{{Name: "idx_users_email", Columns: []string{"email"}}},
			},
		},
	}

	for _, schema := range []*Schema{explicit, implicit} {
		sch, err := desiredSchema(schema, "postgres")
		s.Require().NoError(err)
		tbl, ok := sch.Table("users")
		s.Require().True(ok)
		idx, ok := tbl.Index("idx_users_email")
		s.Require().True(ok)
		s.Assert().Nil(indexTypeAttr(idx.Attrs), "btree must be left as the engine default")
	}
}

func (s *RealmTestSuite) Test_unsupported_index_type_is_rejected() {
	schema := &Schema{
		Tables: map[string]Table{
			"users": {
				Columns: map[string]Column{"email": {Type: "varchar(255)"}},
				Indexes: []Index{{Name: "idx_users_email", Columns: []string{"email"}, Type: "bloom"}},
			},
		},
	}

	_, err := desiredSchema(schema, "postgres")
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "unsupported index type")
	s.Assert().Contains(err.Error(), "gin", "the error must name what the engine accepts")
}

// indexTypeAttr returns the Postgres index-type attribute on an index, or nil
// when the index carries none (meaning the engine default, btree).
func indexTypeAttr(attrs []atlasschema.Attr) *postgres.IndexType {
	for _, a := range attrs {
		if t, ok := a.(*postgres.IndexType); ok {
			return t
		}
	}
	return nil
}
