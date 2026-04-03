package sqlschema

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GenerateDDL_unsupported_engine(t *testing.T) {
	s := &Schema{Tables: map[string]Table{"t": {Columns: map[string]Column{"id": {Type: "int"}}}}}
	_, err := GenerateDDL(s, "mysql")
	assert.ErrorContains(t, err, "unsupported SQL engine")
}

func Test_GenerateDDL_extensions(t *testing.T) {
	s := &Schema{
		Extensions: []string{"uuid-ossp", "pg_trgm"},
		Tables: map[string]Table{
			"t": {Columns: map[string]Column{"id": {Type: "serial", PrimaryKey: true}}},
		},
	}
	ddl, err := GenerateDDL(s, "postgres")
	require.NoError(t, err)
	assert.Contains(t, ddl.Up[0], `"uuid-ossp"`)
	assert.Contains(t, ddl.Up[1], `"pg_trgm"`)

	// Down should drop extensions in reverse order.
	lastDown := ddl.Down[len(ddl.Down)-1]
	secondLastDown := ddl.Down[len(ddl.Down)-2]
	assert.Contains(t, lastDown, `"uuid-ossp"`)
	assert.Contains(t, secondLastDown, `"pg_trgm"`)
}

func Test_GenerateDDL_basic_table(t *testing.T) {
	s := &Schema{
		Tables: map[string]Table{
			"users": {
				Columns: map[string]Column{
					"id":    {Type: "serial", PrimaryKey: true},
					"name":  {Type: "varchar(100)", Nullable: false},
					"email": {Type: "varchar(255)", Nullable: true, Unique: true},
				},
			},
		},
	}

	ddl, err := GenerateDDL(s, "postgres")
	require.NoError(t, err)
	require.Len(t, ddl.Up, 1)

	stmt := ddl.Up[0]
	assert.Contains(t, stmt, `CREATE TABLE IF NOT EXISTS "users"`)
	assert.Contains(t, stmt, `"id" serial`)
	assert.Contains(t, stmt, `"name" varchar(100) NOT NULL`)
	assert.Contains(t, stmt, `"email" varchar(255) UNIQUE`)
	assert.Contains(t, stmt, `PRIMARY KEY ("id")`)

	// Down should drop the table.
	require.Len(t, ddl.Down, 1)
	assert.Contains(t, ddl.Down[0], `DROP TABLE IF EXISTS "users" CASCADE`)
}

func Test_GenerateDDL_with_defaults(t *testing.T) {
	s := &Schema{
		Tables: map[string]Table{
			"events": {
				Columns: map[string]Column{
					"id":         {Type: "serial", PrimaryKey: true},
					"created_at": {Type: "timestamptz", Default: "NOW()"},
				},
			},
		},
	}

	ddl, err := GenerateDDL(s, "postgres")
	require.NoError(t, err)
	assert.Contains(t, ddl.Up[0], `DEFAULT NOW()`)
}

func Test_GenerateDDL_foreign_keys(t *testing.T) {
	s := &Schema{
		Tables: map[string]Table{
			"orders": {
				Columns: map[string]Column{
					"id":      {Type: "serial", PrimaryKey: true},
					"user_id": {Type: "integer", References: &ForeignKey{Table: "users", Column: "id", OnDelete: "CASCADE"}},
				},
			},
			"users": {
				Columns: map[string]Column{
					"id": {Type: "serial", PrimaryKey: true},
				},
			},
		},
	}

	ddl, err := GenerateDDL(s, "postgres")
	require.NoError(t, err)

	// Find the FK up statement.
	var fkUp string
	for _, stmt := range ddl.Up {
		if strings.Contains(stmt, "FOREIGN KEY") {
			fkUp = stmt
			break
		}
	}
	require.NotEmpty(t, fkUp, "expected a foreign key statement")
	assert.Contains(t, fkUp, `REFERENCES "users"("id")`)
	assert.Contains(t, fkUp, "ON DELETE CASCADE")

	// Find the FK down statement.
	var fkDown string
	for _, stmt := range ddl.Down {
		if strings.Contains(stmt, "DROP CONSTRAINT") {
			fkDown = stmt
			break
		}
	}
	require.NotEmpty(t, fkDown, "expected a drop constraint statement")
	assert.Contains(t, fkDown, `"fk_orders_user_id"`)
}

func Test_GenerateDDL_indexes(t *testing.T) {
	s := &Schema{
		Tables: map[string]Table{
			"audit_log": {
				Columns: map[string]Column{
					"id":          {Type: "serial", PrimaryKey: true},
					"entity_type": {Type: "varchar(50)"},
					"entity_id":   {Type: "varchar(100)"},
				},
				Indexes: []Index{
					{Name: "idx_entity", Columns: []string{"entity_type", "entity_id"}},
					{Name: "idx_unique_entity", Columns: []string{"entity_type", "entity_id"}, Unique: true},
				},
			},
		},
	}

	ddl, err := GenerateDDL(s, "postgres")
	require.NoError(t, err)

	var upIndexStmts []string
	for _, stmt := range ddl.Up {
		if strings.Contains(stmt, "INDEX") {
			upIndexStmts = append(upIndexStmts, stmt)
		}
	}
	require.Len(t, upIndexStmts, 2)
	assert.Contains(t, upIndexStmts[0], `CREATE INDEX IF NOT EXISTS "idx_entity"`)
	assert.Contains(t, upIndexStmts[1], `CREATE UNIQUE INDEX IF NOT EXISTS "idx_unique_entity"`)

	var downIndexStmts []string
	for _, stmt := range ddl.Down {
		if strings.Contains(stmt, "DROP INDEX") {
			downIndexStmts = append(downIndexStmts, stmt)
		}
	}
	require.Len(t, downIndexStmts, 2)
}

func Test_GenerateDDL_check_constraints(t *testing.T) {
	s := &Schema{
		Tables: map[string]Table{
			"products": {
				Columns: map[string]Column{
					"id":    {Type: "serial", PrimaryKey: true},
					"price": {Type: "numeric(10,2)"},
				},
				Constraints: []Constraint{
					{Name: "chk_price_positive", Type: "check", Expression: "price > 0"},
				},
			},
		},
	}

	ddl, err := GenerateDDL(s, "postgres")
	require.NoError(t, err)

	var checkUp string
	for _, stmt := range ddl.Up {
		if strings.Contains(stmt, "CHECK") {
			checkUp = stmt
			break
		}
	}
	require.NotEmpty(t, checkUp)
	assert.Contains(t, checkUp, `CHECK (price > 0)`)

	var checkDown string
	for _, stmt := range ddl.Down {
		if strings.Contains(stmt, "chk_price_positive") {
			checkDown = stmt
			break
		}
	}
	require.NotEmpty(t, checkDown)
	assert.Contains(t, checkDown, `DROP CONSTRAINT IF EXISTS "chk_price_positive"`)
}

func Test_GenerateDDL_deterministic_order(t *testing.T) {
	s := &Schema{
		Tables: map[string]Table{
			"zebra":  {Columns: map[string]Column{"id": {Type: "serial", PrimaryKey: true}}},
			"alpha":  {Columns: map[string]Column{"id": {Type: "serial", PrimaryKey: true}}},
			"middle": {Columns: map[string]Column{"id": {Type: "serial", PrimaryKey: true}}},
		},
	}

	ddl1, err := GenerateDDL(s, "postgres")
	require.NoError(t, err)
	ddl2, err := GenerateDDL(s, "postgres")
	require.NoError(t, err)
	assert.Equal(t, ddl1, ddl2)

	// Up: alphabetical order.
	assert.Contains(t, ddl1.Up[0], `"alpha"`)
	assert.Contains(t, ddl1.Up[1], `"middle"`)
	assert.Contains(t, ddl1.Up[2], `"zebra"`)

	// Down: reverse alphabetical order.
	assert.Contains(t, ddl1.Down[0], `"zebra"`)
	assert.Contains(t, ddl1.Down[1], `"middle"`)
	assert.Contains(t, ddl1.Down[2], `"alpha"`)
}

func Test_GenerateDDL_empty_table_error(t *testing.T) {
	s := &Schema{
		Tables: map[string]Table{
			"empty": {Columns: map[string]Column{}},
		},
	}
	_, err := GenerateDDL(s, "postgres")
	assert.ErrorContains(t, err, "no columns")
}
