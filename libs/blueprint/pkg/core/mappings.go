package core

import (
	"github.com/two-hundred/celerity/libs/blueprint/pkg/substitutions"
	"gopkg.in/yaml.v3"
)

// MappingNode provides a tree structure for user-defined
// resource specs or metadata mappings.
//
// This is used to allow creators of resource providers to define
// custom specifications while supporting ${..} placeholder substitution
// as a first class member of the primary representation of the spec.
// The initial intention was to allow the definition of custom native structs
// for resource specs, but this was abandoned in favour of a structure that will
// make placeholder substitution easier to deal with for the framework.
//
// This is also used to provide a tree structure for metadata mappings
// to facilitate substitutions at all levels of nesting in user-provided
// metadata.
type MappingNode struct {
	// Literal represents a literal value in a mapping node.
	Literal *ScalarValue
	// Fields represents a map of field names to child mapping nodes.
	Fields map[string]*MappingNode
	// Items represents a slice of child mapping nodes.
	Items []*MappingNode
	// StringWithSubstitutions is a slice of strings and substitutions
	// where substitutions are a representation of placeholders for variables,
	// resource properties, data source properties and child blueprint properties
	// or function calls wrapped contained within ${..}.
	StringWithSubstitutions *substitutions.StringOrSubstitutions
}

// MarshalYAML fulfils the yaml.Marshaler interface
// to marshal a mapping node into a YAML representation.
func (m *MappingNode) MarshalYAML() (interface{}, error) {
	if m.Literal != nil {
		return m.Literal, nil
	}

	if m.StringWithSubstitutions != nil {
		return m.StringWithSubstitutions, nil
	}

	if m.Fields != nil {
		return m.Fields, nil
	}

	if m.Items != nil {
		return m.Items, nil
	}

	return nil, nil
}

// UnmarshalYAML fulfils the yaml.Unmarshaler interface
// to unmarshal a YAML representation into a mapping node.
func (m *MappingNode) UnmarshalYAML(node *yaml.Node) error {
	// todo: implement unmarshalling a mapping node.
	return nil
}
