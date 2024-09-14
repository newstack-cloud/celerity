package schema

import (
	"encoding/json"
	"fmt"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/jsonutils"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	"gopkg.in/yaml.v3"
)

// Value provides the definition of a value
// that can be used in a blueprint.
type Value struct {
	Type        *ValueTypeWrapper                    `yaml:"type" json:"type"`
	Value       *substitutions.StringOrSubstitutions `yaml:"value" json:"value"`
	Description *substitutions.StringOrSubstitutions `yaml:"description,omitempty" json:"description,omitempty"`
	Secret      *bpcore.ScalarValue                  `yaml:"secret" json:"secret"`
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

// ValueTypeWrapper provides a struct that holds a value type.
// The reason that this exists is to allow more fine-grained control
// when serialising and deserialising values in a blueprint
// so we can check precise value types.
type ValueTypeWrapper struct {
	Value      ValueType
	SourceMeta *source.Meta
}

func (t *ValueTypeWrapper) MarshalYAML() (interface{}, error) {
	return t.Value, nil
}

func (t *ValueTypeWrapper) UnmarshalYAML(value *yaml.Node) error {
	t.SourceMeta = &source.Meta{
		Line:   value.Line,
		Column: value.Column,
	}
	valueType := ValueType(value.Value)
	t.Value = valueType
	return nil
}

func (t *ValueTypeWrapper) MarshalJSON() ([]byte, error) {
	escaped := jsonutils.EscapeJSONString(string(t.Value))
	return []byte(fmt.Sprintf("\"%s\"", escaped)), nil
}

func (t *ValueTypeWrapper) UnmarshalJSON(data []byte) error {
	var typeVal string
	err := json.Unmarshal(data, &typeVal)
	if err != nil {
		return err
	}

	valueType := ValueType(typeVal)
	t.Value = valueType

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

var (
	// ValueTypes provides a slice of all the supported
	// value types to be used for validation of
	// local value types in a blueprint.
	ValueTypes = []ValueType{
		ValueTypeString,
		ValueTypeInteger,
		ValueTypeFloat,
		ValueTypeBoolean,
		ValueTypeArray,
		ValueTypeObject,
	}
)
