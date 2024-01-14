package substitutions

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// StringOrSubstitutions represents a value
// that represents a string interpolated with substitutions.
type StringOrSubstitutions struct {
	Values []*StringOrSubstitution
}

// MarshalYAML fulfils the yaml.Marshaler interface
// to marshal a blueprint string or substitutions value
// into a string representation.
func (s *StringOrSubstitutions) MarshalYAML() (interface{}, error) {
	// During serialisation, there is no way of knowing the context
	// (i.e. the key or field name) in which the substitutions are being used.
	// This is why an empty string is passed as the substitution context.
	return SubstitutionsToString("", s)
}

// UnmarshalYAML fulfils the yaml.Unmarshaler interface
// to unmarshal a string that could contain interpolated
// references.
func (s *StringOrSubstitutions) UnmarshalYAML(node *yaml.Node) error {
	// During deserialisation, there is no way of knowing the context
	// (i.e. the key or field name) in which the substitutions are being used.
	// This is why an empty string is passed as the substitution context.
	parsedValues, err := ParseSubstitutionValues("", node.Value)
	if err != nil {
		return err
	}
	s.Values = parsedValues
	return nil
}

// MarshalJSON fulfils the json.Marshaler interface
// to marshal a blueprint string or substitutions value
// into a string representation.
func (v *StringOrSubstitutions) MarshalJSON() ([]byte, error) {
	str, err := SubstitutionsToString("", v)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("\"%s\"", str)), nil
}

// UnmarshalJSON fulfils the json.Unmarshaler interface
// to unmarshal a string that could contain interpolated
// references.
func (s *StringOrSubstitutions) UnmarshalJSON(data []byte) error {
	// During deserialisation, there is no way of knowing the context
	// (i.e. the key or field name) in which the substitutions are being used.
	// This is why an empty string is passed as the substitution context.
	parsedValues, err := ParseSubstitutionValues("", string(data))
	if err != nil {
		return err
	}
	s.Values = parsedValues
	return nil
}
