package core

import (
	"encoding/json"
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/source"
	"gopkg.in/yaml.v3"
)

// ScalarValue represents a scalar value in
// a blueprint specification.
// Pointers are used as empty values such as "", 0 and false
// are valid default values.
// When marshalling, only one value is expected to be set,
// if multiple values are provided for some reason the priority
// is as follows:
// 1. int
// 2. bool
// 3. float64
// 4. string
//
// The reason strings are the lowest priority is because
// every other scalar type can be parsed as a string.
type ScalarValue struct {
	IntValue    *int
	BoolValue   *bool
	FloatValue  *float64
	StringValue *string
	SourceMeta  *source.Meta
}

// MarshalYAML fulfils the yaml.Marshaler interface
// to marshal a blueprint value into one of the
// supported scalar types.
func (v *ScalarValue) MarshalYAML() (interface{}, error) {
	if v.StringValue != nil {
		return *v.StringValue, nil
	}
	if v.IntValue != nil {
		return *v.IntValue, nil
	}
	if v.BoolValue != nil {
		return *v.BoolValue, nil
	}
	return *v.FloatValue, nil
}

// UnmarshalYAML fulfils the yaml.Unmarshaler interface
// to unmarshal a parsed blueprint value into one of the
// supported scalar types.
func (v *ScalarValue) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return errMustBeScalar(value)
	}

	v.SourceMeta = &source.Meta{
		Position: source.Position{
			Line:   value.Line,
			Column: value.Column,
		},
		EndPosition: source.EndSourcePositionFromYAMLScalarNode(value),
	}

	// Decode will read floating point numbers as integers
	// and truncate. There probably is a cleaner solution
	// for this but checking for decimal point is simple.
	if !strings.Contains(value.Value, ".") {
		var intVal int
		if err := value.Decode(&intVal); err == nil {
			v.IntValue = &intVal
			return nil
		}
	}

	var boolVal bool
	if err := value.Decode(&boolVal); err == nil {
		v.BoolValue = &boolVal
		return nil
	}

	var floatVal float64
	if err := value.Decode(&floatVal); err == nil {
		v.FloatValue = &floatVal
		return nil
	}

	// String is a superset of all other value types so must
	// be tried last.
	var stringVal string
	if err := value.Decode(&stringVal); err == nil {
		v.StringValue = &stringVal
		return nil
	}

	return errMustBeScalar(value)
}

// MarshalJSON fulfils the json.Marshaler interface
// to marshal a blueprint value into one of the
// supported scalar types.
func (v *ScalarValue) MarshalJSON() ([]byte, error) {
	if v.StringValue != nil {
		return json.Marshal(*v.StringValue)
	}

	if v.IntValue != nil {
		return json.Marshal(*v.IntValue)
	}

	if v.BoolValue != nil {
		return json.Marshal(*v.BoolValue)
	}

	return json.Marshal(*v.FloatValue)
}

// UnmarshalJSON fulfils the json.Unmarshaler interface
// to unmarshal a parsed blueprint value into one of the
// supported scalar types.
func (v *ScalarValue) UnmarshalJSON(data []byte) error {

	// Decode will read floating point numbers as integers
	// and truncate. There probably is a cleaner solution
	// for this but checking for decimal point is simple.
	if !strings.Contains(string(data), ".") {
		var intVal int
		if err := json.Unmarshal(data, &intVal); err == nil {
			v.IntValue = &intVal
			return nil
		}
	}

	var boolVal bool
	if err := json.Unmarshal(data, &boolVal); err == nil {
		v.BoolValue = &boolVal
		return nil
	}

	var floatVal float64
	if err := json.Unmarshal(data, &floatVal); err == nil {
		v.FloatValue = &floatVal
		return nil
	}

	// String is a superset of all other value types so must
	// be tried last.
	var stringVal string
	if err := json.Unmarshal(data, &stringVal); err == nil {
		v.StringValue = &stringVal
		return nil
	}

	return errMustBeScalar(nil)
}

func (l *ScalarValue) Equal(otherScalar *ScalarValue) bool {
	if l == nil || otherScalar == nil {
		return false
	}

	if l.StringValue != nil && otherScalar.StringValue != nil {
		return *l.StringValue == *otherScalar.StringValue
	}

	if l.IntValue != nil && otherScalar.IntValue != nil {
		return *l.IntValue == *otherScalar.IntValue
	}

	if l.BoolValue != nil && otherScalar.BoolValue != nil {
		return *l.BoolValue == *otherScalar.BoolValue
	}

	if l.FloatValue != nil && otherScalar.FloatValue != nil {
		return *l.FloatValue == *otherScalar.FloatValue
	}

	return false
}

// StringValueFromScalar extracts a Go string from a string
// scalar value. If the scalar is nil, an empty string is returned.
func StringValueFromScalar(scalar *ScalarValue) string {
	if scalar == nil {
		return ""
	}

	if scalar.StringValue != nil {
		return *scalar.StringValue
	}

	return ""
}

// IntValueFromScalar extracts a Go int from an int
// scalar value. If the scalar is nil, 0 is returned.
func IntValueFromScalar(scalar *ScalarValue) int {
	if scalar == nil {
		return 0
	}

	if scalar.IntValue != nil {
		return *scalar.IntValue
	}

	return 0
}

// BoolValueFromScalar extracts a Go bool from a bool
// scalar value. If the scalar is nil, false is returned.
func BoolValueFromScalar(scalar *ScalarValue) bool {
	if scalar == nil {
		return false
	}

	if scalar.BoolValue != nil {
		return *scalar.BoolValue
	}

	return false
}

// FloatValueFromScalar extracts a Go float64 from a float64
// scalar value. If the scalar is nil, 0.0 is returned.
func FloatValueFromScalar(scalar *ScalarValue) float64 {
	if scalar == nil {
		return 0.0
	}

	if scalar.FloatValue != nil {
		return *scalar.FloatValue
	}

	return 0.0
}

// ScalarFromString creates a scalar value from a string.
func ScalarFromString(value string) *ScalarValue {
	return &ScalarValue{
		StringValue: &value,
	}
}

// ScalarFromBool creates a scalar value from a boolean.
func ScalarFromBool(value bool) *ScalarValue {
	return &ScalarValue{
		BoolValue: &value,
	}
}

// ScalarFromInt creates a scalar value from an integer.
func ScalarFromInt(value int) *ScalarValue {
	return &ScalarValue{
		IntValue: &value,
	}
}

// ScalarFromFloat creates a scalar value from a float.
func ScalarFromFloat(value float64) *ScalarValue {
	return &ScalarValue{
		FloatValue: &value,
	}
}

// ScalarType represents the type of a scalar value that can be
// used in annotation and configuration definitions.
type ScalarType string

const (
	// ScalarTypeString is the type of an element in a spec that is a string.
	ScalarTypeString ScalarType = "string"
	// ScalarTypeInteger is the type of an element in a spec that is an integer.
	ScalarTypeInteger ScalarType = "integer"
	// ScalarTypeFloat is the type of an element in a spec that is a float.
	ScalarTypeFloat ScalarType = "float"
	// ScalarTypeBool is the type of an element in a spec that is a boolean.
	ScalarTypeBool ScalarType = "boolean"
)
