package schema

import (
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
)

// ValidateResourceSchema is a helper function that allows plugins
// to validate a resource spec provided in a blueprint against
// a pre-determined schema.
func ValidateResourceSchema(
	resourceTypeSchema map[string]*Schema,
	blueprintResource *schema.Resource,
	params core.BlueprintParams,
) ([]*core.Diagnostic, error) {
	return nil, nil
}
