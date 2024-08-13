package transform

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
)

// SpecTransformer is the common interface
// used for spec transformations that takes a blueprint spec
// with a specific transform and applies it to expand
// the blueprint spec into it's final form.
// This is primarily for allowing users to define more concise specifications
// where a lot of detail can be abstracted away.
//
// A spec transformer is responsible for providing a way to validate
// abstract resources against a schema prior to transforming the blueprint.
//
// Spec transformers are called straight after a schema has been successfully
// parsed and variables have been validated.
type SpecTransformer interface {
	Transform(ctx context.Context, inputBlueprint *schema.Blueprint) (*schema.Blueprint, error)
	AbstractResource(resourceType string) AbstractResource
}

// AbstractResource is the interface for an abstract resource
// that a spec transformer can contain which includes logic for validating
// an abstract resource before transformation.
type AbstractResource interface {
	// Validate a schema for an abstract resource that will be transformed.
	Validate(ctx context.Context, schemaResource *schema.Resource, params core.BlueprintParams) error
}
