package sqlschema

// Schema represents a parsed SQL database schema YAML file.
// Column types are engine-native (e.g. "serial", "varchar(50)", "jsonb").
type Schema struct {
	Tables     map[string]Table `yaml:"tables"`
	Extensions []string         `yaml:"extensions"`
	// Exclude lists objects that exist in the database but are deliberately not
	// modelled here, typically because a hand-written migration creates them
	// (expression or partial indexes, triggers, functions, policies). Patterns
	// use Atlas's glob syntax relative to the managed schema, e.g.
	// "audit_log.index[idx_*]" or "legacy_*". Excluded objects are never
	// created, altered or dropped by schema-YAML-driven diffs.
	Exclude []string `yaml:"exclude"`
}

// Table describes a single database table.
type Table struct {
	Description string            `yaml:"description"`
	Columns     map[string]Column `yaml:"columns"`
	Indexes     []Index           `yaml:"indexes"`
	Constraints []Constraint      `yaml:"constraints"`
	// Tags are free-form categorisation labels (e.g. "financial", "core-entity")
	// surfaced in exports. They have no effect on DDL.
	Tags []string `yaml:"tags"`
	// TypeName overrides the generated entity type name for this table (used by
	// `schema codegen`). When empty, the name is derived by singularising the
	// table name (e.g. customers → Customer). Set this for irregular plurals the
	// singulariser gets wrong.
	TypeName string `yaml:"typeName"`
}

// Column describes a single table column with engine-native type.
type Column struct {
	Type           string `yaml:"type"`
	PrimaryKey     bool   `yaml:"primaryKey"`
	Nullable       bool   `yaml:"nullable"`
	Unique         bool   `yaml:"unique"`
	Default        string `yaml:"default"`
	Description    string `yaml:"description"`
	Classification string `yaml:"classification"`
	// Tags are free-form categorisation labels for the column, surfaced in
	// exports. They have no effect on DDL.
	Tags       []string    `yaml:"tags"`
	References *ForeignKey `yaml:"references"`
}

// ForeignKey describes a foreign key reference from a column.
type ForeignKey struct {
	Table    string `yaml:"table"`
	Column   string `yaml:"column"`
	OnDelete string `yaml:"onDelete"`
}

// Index describes a table index.
type Index struct {
	Name    string   `yaml:"name"`
	Columns []string `yaml:"columns"`
	Unique  bool     `yaml:"unique"`
	// Type is the index method (Postgres: btree, gin, gist, spgist, brin, hash;
	// MySQL: btree, hash, fulltext, spatial). Empty means the engine default
	// (btree). Only the index method is covered. Expression parts, operator
	// classes and partial-index predicates are not modelled here, and those
	// indexes belong in a hand-written migration listed under Schema.Exclude.
	Type string `yaml:"type"`
}

// Constraint describes a table constraint (check, unique, composite foreign key).
type Constraint struct {
	Name       string   `yaml:"name"`
	Type       string   `yaml:"type"`
	Expression string   `yaml:"expression"`
	Columns    []string `yaml:"columns"`
}

// DatabaseResource holds the resolved configuration for a celerity/sqlDatabase
// blueprint resource. Paths are resolved to absolute paths.
type DatabaseResource struct {
	ResourceName   string
	Name           string
	Engine         string
	SchemaPath     string
	MigrationsPath string
	AuthMode       string
}
