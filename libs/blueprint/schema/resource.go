package schema

import (
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	"gopkg.in/yaml.v3"
)

// Resource represents a blueprint
// resource in the specification.
type Resource struct {
	Type         string                               `yaml:"type" json:"type"`
	Description  *substitutions.StringOrSubstitutions `yaml:"description,omitempty" json:"description,omitempty"`
	Metadata     *Metadata                            `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	LinkSelector *LinkSelector                        `yaml:"linkSelector,omitempty" json:"linkSelector,omitempty"`
	Spec         *core.MappingNode                    `yaml:"spec" json:"spec"`
	SourceMeta   *source.Meta                         `yaml:"-" json:"-"`
}

func (r *Resource) UnmarshalYAML(value *yaml.Node) error {
	r.SourceMeta = &source.Meta{
		Line:   value.Line,
		Column: value.Column,
	}

	type resourceAlias Resource
	var alias resourceAlias
	if err := value.Decode(&alias); err != nil {
		return wrapErrorWithLineInfo(err, value)
	}

	r.Type = alias.Type
	r.Description = alias.Description
	r.Metadata = alias.Metadata
	r.LinkSelector = alias.LinkSelector
	r.Spec = alias.Spec

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
		Line:   value.Line,
		Column: value.Column,
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
		Line:   value.Line,
		Column: value.Column,
	}

	type linkSelectorAlias LinkSelector
	var alias linkSelectorAlias
	if err := value.Decode(&alias); err != nil {
		return wrapErrorWithLineInfo(err, value)
	}

	s.ByLabel = alias.ByLabel

	return nil
}
