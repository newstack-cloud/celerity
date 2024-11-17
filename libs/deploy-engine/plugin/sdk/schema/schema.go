package schema

import "github.com/two-hundred/celerity/libs/blueprint/core"

// Schema is a convenience type that allows plugins
// to declaratively define a schema for a resource spec
// or data source object to be validated against.
type Schema struct {
	// Type is the type of the item in the schema.
	Type ItemType
	// Required must be set of the item is required.
	Required bool
	// ValidateFunc is a function that can be used to validate the item.
	ValidateFunc func(val any, key string) ([]*core.Diagnostic, error)
}

// ItemType is an enum for the supported item types
// that can be used in a schema.
type ItemType string

const (
	// TypeString is the type to be used for a string.
	TypeString ItemType = "string"

	// TypeInt is the type to be used for an integer.
	TypeInt ItemType = "int"

	// TypeBool is the type to be used for a boolean.
	TypeBool ItemType = "bool"

	// TypeFloat is the type to be used for a float.
	TypeFloat ItemType = "float"

	// TypeMap is the type to be used for a mapping.
	TypeMap ItemType = "map"

	// TypeList is the type to be used for a list.
	TypeList ItemType = "list"
)
