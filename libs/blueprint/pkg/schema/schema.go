package schema

import "github.com/two-hundred/celerity/libs/blueprint/pkg/core"

// Blueprint provides the type for a blueprint
// specification loaded into memory.
type Blueprint struct {
	Version     string                 `yaml:"version" json:"version"`
	Transform   *TransformValueWrapper `yaml:"transform,omitempty" json:"transform,omitempty"`
	Variables   map[string]*Variable   `yaml:"variables,omitempty" json:"variables,omitempty"`
	Include     map[string]*Include    `yaml:"include,omitempty" json:"include,omitempty"`
	Resources   map[string]*Resource   `yaml:"resources" json:"resources"`
	DataSources map[string]*DataSource `yaml:"datasources,omitempty" json:"datasources,omitempty"`
	Exports     map[string]*Export     `yaml:"exports,omitempty" json:"exports,omitempty"`
	Metadata    *core.MappingNode      `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}
