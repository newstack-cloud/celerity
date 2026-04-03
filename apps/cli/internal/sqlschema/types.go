package sqlschema

// Schema represents a parsed SQL database schema YAML file.
// Column types are engine-native (e.g. "serial", "varchar(50)", "jsonb").
type Schema struct {
	Tables     map[string]Table `yaml:"tables"`
	Extensions []string         `yaml:"extensions"`
}

// Table describes a single database table.
type Table struct {
	Description string            `yaml:"description"`
	Columns     map[string]Column `yaml:"columns"`
	Indexes     []Index           `yaml:"indexes"`
	Constraints []Constraint      `yaml:"constraints"`
}

// Column describes a single table column with engine-native type.
type Column struct {
	Type           string      `yaml:"type"`
	PrimaryKey     bool        `yaml:"primaryKey"`
	Nullable       bool        `yaml:"nullable"`
	Unique         bool        `yaml:"unique"`
	Default        string      `yaml:"default"`
	Description    string      `yaml:"description"`
	Classification string      `yaml:"classification"`
	References     *ForeignKey `yaml:"references"`
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
}

// Constraint describes a table constraint (check, unique, composite foreign key).
type Constraint struct {
	Name       string   `yaml:"name"`
	Type       string   `yaml:"type"`
	Expression string   `yaml:"expression"`
	Columns    []string `yaml:"columns"`
}

// MigrationScript represents a versioned SQL migration with up/down files
// (V<number>__<description>.up.sql / V<number>__<description>.down.sql).
type MigrationScript struct {
	Version     int
	Description string
	UpPath      string
	DownPath    string
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
