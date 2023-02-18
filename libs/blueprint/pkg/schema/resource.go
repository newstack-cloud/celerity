package schema

import (
	"github.com/two-hundred/celerity/libs/blueprint/pkg/core"
)

// Resource represents a blueprint
// resource in the specification.
type Resource struct {
	Type         string        `yaml:"type" json:"type"`
	Metadata     *Metadata     `yaml:"metadata" json:"metadata"`
	LinkSelector *LinkSelector `yaml:"linkSelector,omitempty" json:"linkSelector,omitempty"`
	// This is an initial form of the spec that is fed into
	// a resource provider that will convert this into the precise
	// spec for each resource type.
	Spec map[string]interface{} `yaml:"spec" json:"spec"`
}

// Metadata represents the metadata associated
// with a blueprint resource that can be used to provide labels
// and annotations that can be used to configure
// instances and used for link selections.
type Metadata struct {
	DisplayName string                      `yaml:"displayName" json:"displayName"`
	Annotations map[string]core.ScalarValue `yaml:"annotations" json:"annotations"`
	Labels      map[string]string           `yaml:"labels" json:"labels"`
	Custom      map[string]interface{}      `yaml:"custom" json:"custom"`
}

// LinkSelector allows a resource to select other resources
// to link to by label.
type LinkSelector struct {
	ByLabel map[string]string `yaml:"byLabel" json:"byLabel"`
}
