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
	// Options loads a set of fixed possible values available
	// for the custom variable type.
	// In the returned options, each one is keyed by a label, essentially
	// behaving as a runtime enum.
	Options(ctx context.Context, params core.BlueprintParams) (map[string]*core.ScalarValue, error)
}
