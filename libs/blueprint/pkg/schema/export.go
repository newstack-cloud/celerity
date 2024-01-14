package schema

import (
	"github.com/two-hundred/celerity/libs/blueprint/pkg/substitutions"
)

// Export represents a blueprint
// exported field in the specification.
// Exports are designed to be persisted with the state of a blueprint instance
// and to be accessible to other blueprints and external systems exposed
// via an API, an include reference or as a field in a "blueprint" resource.
// (The latter of the three options would require an implementation of blueprint resource provider)
type Export struct {
	Type        ExportType                           `yaml:"type" json:"type"`
	Field       string                               `yaml:"field" json:"field"`
	Description *substitutions.StringOrSubstitutions `yaml:"description,omitempty" json:"description,omitempty"`
}

// ExportType represents a type of exported field
// defined in a blueprint.
// Can be one of "string", "object", "integer", "float", "array" or "boolean".
type ExportType string

func (t ExportType) Equal(compareWith ExportType) bool {
	return t == compareWith
}

const (
	// ExportTypeString is for a string export
	// in a blueprint.
	ExportTypeString ExportType = "string"
	// ExportTypeObject is for an object export
	// in a blueprint.
	ExportTypeObject ExportType = "object"
	// ExportTypeInteger is for an integer export
	// in a blueprint.
	ExportTypeInteger ExportType = "integer"
	// ExportTypeFloat is for a float export
	// in a blueprint.
	ExportTypeFloat ExportType = "float"
	// ExportTypeArray is for an array export
	// in a blueprint.
	ExportTypeArray ExportType = "array"
	// ExportTypeBoolean is for a boolean export
	// in a blueprint.
	ExportTypeBoolean ExportType = "boolean"
)

var (
	// ExportTypes provides a slice of all the supported
	// export types.
	ExportTypes = []ExportType{
		ExportTypeString,
		ExportTypeObject,
		ExportTypeInteger,
		ExportTypeFloat,
		ExportTypeArray,
		ExportTypeBoolean,
	}
)
