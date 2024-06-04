package schema

import (
	"github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/substitutions"
)

// Resource represents a blueprint
// resource in the specification.
type Resource struct {
	Type         string                               `yaml:"type" json:"type"`
	Description  *substitutions.StringOrSubstitutions `yaml:"description,omitempty" json:"description,omitempty"`
	Metadata     *Metadata                            `yaml:"metadata" json:"metadata"`
	LinkSelector *LinkSelector                        `yaml:"linkSelector,omitempty" json:"linkSelector,omitempty"`
	Spec         *core.MappingNode                    `yaml:"spec" json:"spec"`
}

// Metadata represents the metadata associated
// with a blueprint resource that can be used to provide labels
// and annotations that can be used to configure
// instances and used for link selections.
type Metadata struct {
	DisplayName *substitutions.StringOrSubstitutions            `yaml:"displayName" json:"displayName"`
	Annotations map[string]*substitutions.StringOrSubstitutions `yaml:"annotations" json:"annotations"`
	Labels      map[string]string                               `yaml:"labels" json:"labels"`
	Custom      *core.MappingNode                               `yaml:"custom,omitempty" json:"custom,omitempty"`
}

// LinkSelector allows a resource to select other resources
// to link to by label.
type LinkSelector struct {
	ByLabel map[string]string `yaml:"byLabel" json:"byLabel"`
}
