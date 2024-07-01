package schema

import (
	bpcore "github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/source"
	"gopkg.in/yaml.v3"
)

// Variable provides the definition of a variable
// that can be used in a blueprint.
type Variable struct {
	Type          VariableType          `yaml:"type" json:"type"`
	Description   string                `yaml:"description,omitempty" json:"description,omitempty"`
	Secret        bool                  `yaml:"secret" json:"secret"`
	Default       *bpcore.ScalarValue   `yaml:"default,omitempty" json:"default,omitempty"`
	AllowedValues []*bpcore.ScalarValue `yaml:"allowedValues,omitempty" json:"allowedValues,omitempty"`
	SourceMeta    *source.Meta          `yaml:"-" json:"-"`
}

func (t *Variable) UnmarshalYAML(value *yaml.Node) error {
	t.SourceMeta = &source.Meta{
		Line:   value.Line,
		Column: value.Column,
	}

	type variableAlias Variable
	var alias variableAlias
	if err := value.Decode(&alias); err != nil {
		return wrapErrorWithLineInfo(err, value)
	}

	t.Type = alias.Type
	t.Description = alias.Description
	t.Secret = alias.Secret
	t.Default = alias.Default
	t.AllowedValues = alias.AllowedValues

	return nil
}

// VariableType represents a type of variable
// defined in a blueprint.
// Can be one of "string", "integer", "float" or "boolean" or a custom type
// defined by a resource provider.
type VariableType string

func (t VariableType) Equal(compareWith VariableType) bool {
	return t == compareWith
}

const (
	// VariableTypeString is for a string variable
	// in a blueprint.
	VariableTypeString VariableType = "string"
	// VariableTypeInteger is for an integer value
	// in a blueprint.
	VariableTypeInteger VariableType = "integer"
	// VariableTypeFloat is for a float value
	// in a blueprint.
	VariableTypeFloat VariableType = "float"
	// VariableTypeBoolean is for a boolean value
	// in a blueprint.
	VariableTypeBoolean VariableType = "boolean"
)

var (
	// CoreVariableTypes provides a slice of all the core supported
	// variable types to be used for clean validation of fields
	// with a field with VariableType.
	// This does not represent all possible variable types,
	// as provider custom variable types are also supported.
	CoreVariableTypes = []VariableType{
		VariableTypeString,
		VariableTypeInteger,
		VariableTypeFloat,
		VariableTypeBoolean,
	}
)
