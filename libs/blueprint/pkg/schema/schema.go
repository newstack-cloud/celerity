package schema

import (
	"encoding/json"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/source"
	"gopkg.in/yaml.v3"
)

// Blueprint provides the type for a blueprint
// specification loaded into memory.
type Blueprint struct {
	Version     string                 `yaml:"version" json:"version"`
	Transform   *TransformValueWrapper `yaml:"transform,omitempty" json:"transform,omitempty"`
	Variables   *VariableMap           `yaml:"variables,omitempty" json:"variables,omitempty"`
	Include     *IncludeMap            `yaml:"include,omitempty" json:"include,omitempty"`
	Resources   *ResourceMap           `yaml:"resources" json:"resources"`
	DataSources map[string]*DataSource `yaml:"datasources,omitempty" json:"datasources,omitempty"`
	Exports     map[string]*Export     `yaml:"exports,omitempty" json:"exports,omitempty"`
	Metadata    *core.MappingNode      `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// VariableMap provides a mapping of names to variable values
// in a blueprint.
// This includes extra information about the locations of
// the keys in the original source being unmarshalled.
// This information will not always be present, it is populated
// when unmarshalling from YAML source documents.
type VariableMap struct {
	Values map[string]*Variable
	// Mapping of variable names to their source locations.
	SourceMeta map[string]*source.Meta
}

func (m *VariableMap) MarshalYAML() (interface{}, error) {
	return m.Values, nil
}

func (m *VariableMap) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return errInvalidMap(value, "variables")
	}

	m.Values = make(map[string]*Variable)
	m.SourceMeta = make(map[string]*source.Meta)
	for i := 0; i < len(value.Content); i += 2 {
		key := value.Content[i]
		val := value.Content[i+1]

		m.SourceMeta[key.Value] = &source.Meta{
			Line:   key.Line,
			Column: key.Column,
		}

		var variable Variable
		err := val.Decode(&variable)
		if err != nil {
			return err
		}

		m.Values[key.Value] = &variable
	}

	return nil
}

func (m *VariableMap) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Values)
}

func (m *VariableMap) UnmarshalJSON(data []byte) error {
	values := make(map[string]*Variable)
	err := json.Unmarshal(data, &values)
	if err != nil {
		return err
	}

	m.Values = values
	return nil
}

// IncludeMap provides a mapping of names to child
// blueprint includes.
// This includes extra information about the locations of
// the keys in the original source being unmarshalled.
// This information will not always be present, it is populated
// when unmarshalling from YAML source documents.
type IncludeMap struct {
	Values map[string]*Include
	// Mapping of variable names to their source locations.
	SourceMeta map[string]*source.Meta
}

func (m *IncludeMap) MarshalYAML() (interface{}, error) {
	return m.Values, nil
}

func (m *IncludeMap) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return errInvalidMap(value, "include")
	}

	m.Values = make(map[string]*Include)
	m.SourceMeta = make(map[string]*source.Meta)
	for i := 0; i < len(value.Content); i += 2 {
		key := value.Content[i]
		val := value.Content[i+1]

		m.SourceMeta[key.Value] = &source.Meta{
			Line:   key.Line,
			Column: key.Column,
		}

		var include Include
		err := val.Decode(&include)
		if err != nil {
			return err
		}

		m.Values[key.Value] = &include
	}

	return nil
}

func (m *IncludeMap) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Values)
}

func (m *IncludeMap) UnmarshalJSON(data []byte) error {
	values := make(map[string]*Include)
	err := json.Unmarshal(data, &values)
	if err != nil {
		return err
	}

	m.Values = values
	return nil
}

// ResourceMap provides a mapping of names to resources.
// This includes extra information about the locations of
// the keys in the original source being unmarshalled.
// This information will not always be present, it is populated
// when unmarshalling from YAML source documents.
type ResourceMap struct {
	Values map[string]*Resource
	// Mapping of variable names to their source locations.
	SourceMeta map[string]*source.Meta
}

func (m *ResourceMap) MarshalYAML() (interface{}, error) {
	return m.Values, nil
}

func (m *ResourceMap) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return errInvalidMap(value, "resources")
	}

	m.Values = make(map[string]*Resource)
	m.SourceMeta = make(map[string]*source.Meta)
	for i := 0; i < len(value.Content); i += 2 {
		key := value.Content[i]
		val := value.Content[i+1]

		m.SourceMeta[key.Value] = &source.Meta{
			Line:   key.Line,
			Column: key.Column,
		}

		var resource Resource
		err := val.Decode(&resource)
		if err != nil {
			return err
		}

		m.Values[key.Value] = &resource
	}

	return nil
}

func (m *ResourceMap) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Values)
}

func (m *ResourceMap) UnmarshalJSON(data []byte) error {
	values := make(map[string]*Resource)
	err := json.Unmarshal(data, &values)
	if err != nil {
		return err
	}

	m.Values = values
	return nil
}
