package schema

import (
	"encoding/json"
	"fmt"

	"github.com/two-hundred/celerity/libs/common/pkg/core"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"gopkg.in/yaml.v3"
)

// Variable provides the definition of a variable
// that can be used in a blueprint.
type Variable struct {
	Type        *VariableTypeWrapper `yaml:"type" json:"type"`
	Description string               `yaml:"description,omitempty" json:"description,omitempty"`
	Secret      bool                 `yaml:"secret" json:"secret"`
	Default     *bpcore.ScalarValue  `yaml:"default,omitempty" json:"default,omitempty"`
}

// VariableType represents a type of variable
// defined in a blueprint.
// Can be one of "string", "integer", "float" or "boolean".
type VariableType string

func (t VariableType) Equal(compareWith VariableType) bool {
	return t == compareWith
}

// VariableTypeWrapper provides a struct that holds a variable type
// value.
// The reason that this exists is to allow more fine-grained control
// when serialising and deserialising variables in a blueprint
// so we can check precise values.
type VariableTypeWrapper struct {
	Value VariableType
}

func (t *VariableTypeWrapper) MarshalYAML() (interface{}, error) {
	if !core.SliceContains(VariableTypes, t.Value) {
		return nil, errInvalidVariableType(t.Value)
	}

	return t.Value, nil
}

func (t *VariableTypeWrapper) UnmarshalYAML(value *yaml.Node) error {
	valueVarType := VariableType(value.Value)
	if !core.SliceContains(VariableTypes, valueVarType) {
		return errInvalidVariableType(valueVarType)
	}

	t.Value = valueVarType
	return nil
}

func (t *VariableTypeWrapper) MarshalJSON() ([]byte, error) {
	if !core.SliceContains(VariableTypes, t.Value) {
		return nil, errInvalidVariableType(t.Value)
	}
	return []byte(fmt.Sprintf("\"%s\"", t.Value)), nil
}

func (t *VariableTypeWrapper) UnmarshalJSON(data []byte) error {
	var typeVal string
	err := json.Unmarshal(data, &typeVal)
	if err != nil {
		return err
	}

	typeValVarType := VariableType(typeVal)
	if !core.SliceContains(VariableTypes, typeValVarType) {
		return errInvalidVariableType(typeValVarType)
	}
	t.Value = VariableType(typeVal)

	return nil
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
	// VariableTypes provides a slice of all the supported
	// variable types to be used for clean validation of fields
	// with a field with VariableType.
	VariableTypes = []VariableType{
		VariableTypeString,
		VariableTypeInteger,
		VariableTypeFloat,
		VariableTypeBoolean,
	}
)
