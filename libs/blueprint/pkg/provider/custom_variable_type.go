package provider

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
)

// CustomVariableType provides the interface for a custom variable type
// that provides convenience variable types with a (usually large) fixed set of possible values.
// A custom variable type should not be used for dynamically sourced values
// external to a blueprint, data sources exist for that purpose.
// All custom variable type values must be of the same primitive type.
type CustomVariableType interface {
	// Validate deals with ensuring that a variable value with a custom variable type
	// is in the fixed set of possibilities.
	Validate(ctx context.Context, schemaDataSource *schema.Variable, params core.BlueprintParams) error
	// Options deals with loading a set of fixed options available
	// for the custom variable type.
	// In the returned options, each one is keyed by a label, essentially
	// behaving as a runtime enum.
	Options(ctx context.Context, params core.BlueprintParams) (map[string]*core.ScalarValue, error)
}
