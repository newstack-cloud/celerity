package sqlschema

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ParseSchema_valid(t *testing.T) {
	yaml := `
tables:
  audit_log:
    description: "Audit trail"
    columns:
      id: { type: "serial", primaryKey: true }
      entity_type: { type: "varchar(50)", nullable: false }
      details: { type: "jsonb" }
      created_at: { type: "timestamptz", default: "NOW()" }
    indexes:
      - name: "idx_audit_entity"
        columns: ["entity_type"]
extensions:
  - "uuid-ossp"
`
	s, err := ParseSchema(strings.NewReader(yaml))
	require.NoError(t, err)
	require.Len(t, s.Tables, 1)
	require.Contains(t, s.Tables, "audit_log")

	table := s.Tables["audit_log"]
	assert.Equal(t, "Audit trail", table.Description)
	assert.Len(t, table.Columns, 4)
	assert.True(t, table.Columns["id"].PrimaryKey)
	assert.Equal(t, "serial", table.Columns["id"].Type)
	assert.False(t, table.Columns["entity_type"].Nullable)
	assert.Equal(t, "NOW()", table.Columns["created_at"].Default)
	assert.Len(t, table.Indexes, 1)
	assert.Equal(t, "idx_audit_entity", table.Indexes[0].Name)
	assert.Equal(t, []string{"uuid-ossp"}, s.Extensions)
}

func Test_ParseSchema_with_foreign_key(t *testing.T) {
	yaml := `
tables:
  orders:
    columns:
      id: { type: "serial", primaryKey: true }
      user_id:
        type: "integer"
        references:
          table: "users"
          column: "id"
          onDelete: "CASCADE"
`
	s, err := ParseSchema(strings.NewReader(yaml))
	require.NoError(t, err)

	col := s.Tables["orders"].Columns["user_id"]
	require.NotNil(t, col.References)
	assert.Equal(t, "users", col.References.Table)
	assert.Equal(t, "id", col.References.Column)
	assert.Equal(t, "CASCADE", col.References.OnDelete)
}

func Test_ParseSchema_with_constraints(t *testing.T) {
	yaml := `
tables:
  products:
    columns:
      id: { type: "serial", primaryKey: true }
      price: { type: "numeric(10,2)" }
    constraints:
      - name: "chk_price_positive"
        type: "check"
        expression: "price > 0"
`
	s, err := ParseSchema(strings.NewReader(yaml))
	require.NoError(t, err)

	table := s.Tables["products"]
	require.Len(t, table.Constraints, 1)
	assert.Equal(t, "chk_price_positive", table.Constraints[0].Name)
	assert.Equal(t, "check", table.Constraints[0].Type)
	assert.Equal(t, "price > 0", table.Constraints[0].Expression)
}

func Test_ParseSchema_no_tables_error(t *testing.T) {
	yaml := `extensions: ["uuid-ossp"]`
	_, err := ParseSchema(strings.NewReader(yaml))
	assert.ErrorContains(t, err, "at least one table")
}

func Test_ParseSchema_invalid_yaml(t *testing.T) {
	_, err := ParseSchema(strings.NewReader(`{invalid`))
	assert.Error(t, err)
}

func Test_ParseSchema_metadata_fields(t *testing.T) {
	yaml := `
tables:
  users:
    columns:
      email:
        type: "varchar(255)"
        description: "User email address"
        classification: "pii"
        unique: true
        nullable: false
`
	s, err := ParseSchema(strings.NewReader(yaml))
	require.NoError(t, err)

	col := s.Tables["users"].Columns["email"]
	assert.Equal(t, "User email address", col.Description)
	assert.Equal(t, "pii", col.Classification)
	assert.True(t, col.Unique)
	assert.False(t, col.Nullable)
}
