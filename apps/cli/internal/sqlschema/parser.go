package sqlschema

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// ParseSchemaFile reads and parses a SQL schema YAML file from disk.
func ParseSchemaFile(path string) (*Schema, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening schema file %s: %w", path, err)
	}
	defer f.Close()
	return ParseSchema(f)
}

// ParseSchema parses a SQL schema YAML from a reader.
func ParseSchema(r io.Reader) (*Schema, error) {
	var s Schema
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("parsing schema YAML: %w", err)
	}
	if len(s.Tables) == 0 {
		return nil, fmt.Errorf("schema must define at least one table")
	}
	return &s, nil
}
