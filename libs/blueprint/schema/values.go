package schema

import (
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	"gopkg.in/yaml.v3"
)

// Value provides the definition of a value
// that can be used in a blueprint.
type Value struct {
	Type        ValueType                            `yaml:"type" json:"type"`
	Value       *substitutions.StringOrSubstitutions `yaml:"value" json:"value"`
	Description *substitutions.StringOrSubstitutions `yaml:"description,omitempty" json:"description,omitempty"`
	Secret      bool                                 `yaml:"secret" json:"secret"`
	SourceMeta  *source.Meta                         `yaml:"-" json:"-"`
}

func (t *Value) UnmarshalYAML(value *yaml.Node) error {
	t.SourceMeta = &source.Meta{
		Line:   value.Line,
		Column: value.Column,
	}

	type valueAlias Value
	var alias valueAlias
	if err := value.Decode(&alias); err != nil {
		return wrapErrorWithLineInfo(err, value)
	}

	t.Type = alias.Type
	t.Description = alias.Description
	t.Secret = alias.Secret
	t.Value = alias.Value

	return nil
}

// ValueType represents a type of value
// defined in a blueprint.
// Can be one of "string", "integer", "float", "boolean", "array" or "object".
type ValueType string

func (t ValueType) Equal(compareWith ValueType) bool {
	return t == compareWith
}

const (
	// ValueTypeString is for a string value
	// in a blueprint.
	ValueTypeString ValueType = "string"
	// ValueTypeInteger is for an integer value
	// in a blueprint.
	ValueTypeInteger ValueType = "integer"
	// ValueTypeFloat is for a float value
	// in a blueprint.
	ValueTypeFloat ValueType = "float"
	// ValueTypeBoolean is for a boolean value
	// in a blueprint.
	ValueTypeBoolean ValueType = "boolean"
	// ValueTypeArray is for an array value
	// in a blueprint.
	ValueTypeArray ValueType = "array"
	// ValueTypeObject is for an object value
	// in a blueprint.
	ValueTypeObject ValueType = "object"
)
