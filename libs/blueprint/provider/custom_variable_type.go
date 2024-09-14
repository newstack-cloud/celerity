package provider

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
)

// CustomVariableType provides the interface for a custom variable type
// that provides convenience variable types with a (usually large) fixed set of possible values.
// A custom variable type should not be used for dynamically sourced values
// external to a blueprint, data sources exist for that purpose.
// All custom variable type values must be of the same primitive type.
type CustomVariableType interface {
	// GetType deals with retrieving the namespaced type for a custom variable type.
	GetType(ctx context.Context, input *CustomVariableTypeGetTypeInput) (*CustomVariableTypeGetTypeOutput, error)
	// GetDescription deals with retrieving the description for a custom variable type in a blueprint spec
	// that can be used for documentation and tooling.
	// Markdown and plain text formats are supported.
	GetDescription(ctx context.Context, input *CustomVariableTypeGetDescriptionInput) (*CustomVariableTypeGetDescriptionOutput, error)
	// Options loads a set of fixed possible values available
	// for the custom variable type.
	// In the returned options, each one is keyed by a label, essentially
	// behaving as a runtime enum.
	Options(ctx context.Context, input *CustomVariableTypeOptionsInput) (*CustomVariableTypeOptionsOutput, error)
}

// CustomVariableTypeOptionsInput provides the input required to load
// the fixed set of possible values for a custom variable type.
type CustomVariableTypeOptionsInput struct {
	Params core.BlueprintParams
}

// CustomVariableTypeOptionsOutput provides
// the output from loading the fixed set of possible values
// for a custom variable type.
type CustomVariableTypeOptionsOutput struct {
	Options map[string]*core.ScalarValue
}

// CustomVariableTypeGetTypeInput provides the input required to
// retrieve the namespaced type for a custom variable type.
type CustomVariableTypeGetTypeInput struct {
	Params core.BlueprintParams
}

// CustomVariableTypeGetTypeOutput provides the output from retrieving the namespaced type
// for a custom variable type.
type CustomVariableTypeGetTypeOutput struct {
	Type string
}

// CustomVariableTypeGetDescriptionInput provides the input required to
// retrieve a description for a custom variable type.
type CustomVariableTypeGetDescriptionInput struct {
	Params core.BlueprintParams
}

// CustomVariableTypeGetDescriptionOutput provides the output from retrieving the description
// for a custom variable type.
type CustomVariableTypeGetDescriptionOutput struct {
	MarkdownDescription  string
	PlainTextDescription string
}
