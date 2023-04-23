package transform

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
)

// SpecTransformer is the common interface
// used for spec transformations that takes a blueprint spec
// with a specific transform and applies it to expand
// the blueprint spec into it's final form.
// This is primarily for allowing users to define more concise specifications
// where a lot of detail can be abstracted away.
//
// Spec transformers are called straight after a schema has been successfully
// parsed and variables have been validated.
type SpecTransformer interface {
	Transform(ctx context.Context, inputBlueprint *schema.Blueprint) (*schema.Blueprint, error)
}
