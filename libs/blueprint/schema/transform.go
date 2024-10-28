package schema

import (
	"gopkg.in/yaml.v3"
)

// TransformValueWrapper holds one or more transforms
// to be applied to a specification.
// This allows for users to provide the transform field in a spec
// as a string or as a list of strings.
type TransformValueWrapper struct {
	StringList
}

func (t *TransformValueWrapper) MarshalYAML() (interface{}, error) {
	return t.StringList.MarshalYAML()
}

func (t *TransformValueWrapper) UnmarshalYAML(value *yaml.Node) error {
	return t.StringList.unmarshalYAML(value, errInvalidTransformType, "transform")
}

func (t *TransformValueWrapper) MarshalJSON() ([]byte, error) {
	return t.StringList.MarshalJSON()
}

func (t *TransformValueWrapper) UnmarshalJSON(data []byte) error {
	return t.unmarshalJSON(data, errInvalidTransformType, "transform")
}
