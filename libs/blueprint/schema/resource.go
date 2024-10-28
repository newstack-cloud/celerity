package schema

import (
	"encoding/json"
	"fmt"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/jsonutils"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	"gopkg.in/yaml.v3"
)

// Resource represents a blueprint
// resource in the specification.
type Resource struct {
	Type         *ResourceTypeWrapper                 `yaml:"type" json:"type"`
	Description  *substitutions.StringOrSubstitutions `yaml:"description,omitempty" json:"description,omitempty"`
	Metadata     *Metadata                            `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	DependsOn    *DependsOnList                       `yaml:"dependsOn,omitempty" json:"dependsOn,omitempty"`
	Condition    *Condition                           `yaml:"condition,omitempty" json:"condition,omitempty"`
	Each         *substitutions.StringOrSubstitutions `yaml:"each,omitempty" json:"each,omitempty"`
	LinkSelector *LinkSelector                        `yaml:"linkSelector,omitempty" json:"linkSelector,omitempty"`
	Spec         *core.MappingNode                    `yaml:"spec" json:"spec"`
	SourceMeta   *source.Meta                         `yaml:"-" json:"-"`
}

func (r *Resource) UnmarshalYAML(value *yaml.Node) error {
	r.SourceMeta = &source.Meta{
		Position: source.Position{
			Line:   value.Line,
			Column: value.Column,
		},
	}

	type resourceAlias Resource
	var alias resourceAlias
	if err := value.Decode(&alias); err != nil {
		return wrapErrorWithLineInfo(err, value)
	}

	r.Type = alias.Type
	r.Description = alias.Description
	r.Metadata = alias.Metadata
	r.DependsOn = alias.DependsOn
	r.Condition = alias.Condition
	r.Each = alias.Each
	r.LinkSelector = alias.LinkSelector
	r.Spec = alias.Spec

	return nil
}

// DependsOnList provides a list of resource names
// that a resource depends on.
// This can include extra information about the locations of
// elements in the list in the original source,
// depending on the source format.
type DependsOnList struct {
	StringList
}

func (t *DependsOnList) MarshalYAML() (interface{}, error) {
	return t.StringList.MarshalYAML()
}

func (t *DependsOnList) UnmarshalYAML(value *yaml.Node) error {
	return t.StringList.unmarshalYAML(value, errInvalidDependencyType, "dependency")
}

func (t *DependsOnList) MarshalJSON() ([]byte, error) {
	return t.StringList.MarshalJSON()
}

func (t *DependsOnList) UnmarshalJSON(data []byte) error {
	return t.unmarshalJSON(data, errInvalidDependencyType, "dependency")
}

// ResourceTypeWrapper provides a struct that holds a resource type
// value.
type ResourceTypeWrapper struct {
	Value      string
	SourceMeta *source.Meta
}

func (t *ResourceTypeWrapper) MarshalYAML() (interface{}, error) {
	return t.Value, nil
}

func (t *ResourceTypeWrapper) UnmarshalYAML(value *yaml.Node) error {
	t.SourceMeta = &source.Meta{
		Position: source.Position{
			Line:   value.Line,
			Column: value.Column,
		},
		EndPosition: source.EndSourcePositionFromYAMLScalarNode(value),
	}

	t.Value = value.Value
	return nil
}

func (t *ResourceTypeWrapper) MarshalJSON() ([]byte, error) {
	escaped := jsonutils.EscapeJSONString(string(t.Value))
	return []byte(fmt.Sprintf("\"%s\"", escaped)), nil
}

func (t *ResourceTypeWrapper) UnmarshalJSON(data []byte) error {
	var typeVal string
	err := json.Unmarshal(data, &typeVal)
	if err != nil {
		return err
	}

	t.Value = typeVal

	return nil
}

// Metadata represents the metadata associated
// with a blueprint resource that can be used to provide labels
// and annotations that can be used to configure
// instances and used for link selections.
type Metadata struct {
	DisplayName *substitutions.StringOrSubstitutions `yaml:"displayName" json:"displayName"`
	Annotations *StringOrSubstitutionsMap            `yaml:"annotations,omitempty" json:"annotations,omitempty"`
	Labels      *StringMap                           `yaml:"labels,omitempty" json:"labels,omitempty"`
	Custom      *core.MappingNode                    `yaml:"custom,omitempty" json:"custom,omitempty"`
	SourceMeta  *source.Meta                         `yaml:"-" json:"-"`
}

func (m *Metadata) UnmarshalYAML(value *yaml.Node) error {
	m.SourceMeta = &source.Meta{
		Position: source.Position{
			Line:   value.Line,
			Column: value.Column,
		},
	}

	type metadataAlias Metadata
	var alias metadataAlias
	if err := value.Decode(&alias); err != nil {
		return wrapErrorWithLineInfo(err, value)
	}

	m.DisplayName = alias.DisplayName
	m.Annotations = alias.Annotations
	m.Labels = alias.Labels
	m.Custom = alias.Custom

	return nil
}

// LinkSelector allows a resource to select other resources
// to link to by label.
type LinkSelector struct {
	ByLabel    *StringMap   `yaml:"byLabel" json:"byLabel"`
	SourceMeta *source.Meta `yaml:"-" json:"-"`
}

func (s *LinkSelector) UnmarshalYAML(value *yaml.Node) error {
	s.SourceMeta = &source.Meta{
		Position: source.Position{
			Line:   value.Line,
			Column: value.Column,
		},
	}

	type linkSelectorAlias LinkSelector
	var alias linkSelectorAlias
	if err := value.Decode(&alias); err != nil {
		return wrapErrorWithLineInfo(err, value)
	}

	s.ByLabel = alias.ByLabel

	return nil
}
