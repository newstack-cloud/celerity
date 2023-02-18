package core

import (
	"encoding/json"
	"errors"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	// ErrValueMustBeScalar is an error that is returned
	// when a blueprint scalar is not a scalar value.
	ErrValueMustBeScalar = errors.New("a blueprint scalar value must be a scalar (string, int, bool or float)")
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
		return ErrValueMustBeScalar
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

	return ErrValueMustBeScalar
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

	return ErrValueMustBeScalar
}
