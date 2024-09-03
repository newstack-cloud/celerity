package transform

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
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
	// Transform a blueprint by expanding abstract resources
	// into their final form along with any other transformations
	// that are required.
	Transform(ctx context.Context, input *SpecTransformerTransformInput) (*SpecTransformerTransformOutput, error)
	// AbstractResources returns the abstract resource implementation
	// for a given resource type.
	AbstractResource(ctx context.Context, resourceType string) (AbstractResource, error)
}

// AbstractResource is the interface for an abstract resource
// that a spec transformer can contain which includes logic for validating
// an abstract resource before transformation.
type AbstractResource interface {
	// Validate a schema for an abstract resource that will be transformed.
	Validate(ctx context.Context, input *AbstractResourceValidateInput) (*AbstractResourceValidateOutput, error)
	// GetSpecDefinition retrieves the spec definition for an abstract resource,
	// this is the first line of validation for a resource in a blueprint and is also
	// useful for validating references to an abstract resource instance
	// in a blueprint and for providing definitions for docs and tooling.
	GetSpecDefinition(
		ctx context.Context,
		input *AbstractResourceGetSpecDefinitionInput,
	) (*AbstractResourceGetSpecDefinitionOutput, error)
}

// SpecTransformerTransformInput provides the input required to transform
// a blueprint.
type SpecTransformerTransformInput struct {
	InputBlueprint *schema.Blueprint
}

// SpecTransformerTransformOutput provides the output from transforming a blueprint
// which includes the expanded blueprint.
type SpecTransformerTransformOutput struct {
	TransformedBlueprint *schema.Blueprint
}

// AbstractResourceValidateInput provides the input required to validate
// an abstract resource before transformation.
type AbstractResourceValidateInput struct {
	SchemaResource *schema.Resource
	Params         core.BlueprintParams
}

// AbstractResourceValidateOutput provides the output from validating an abstract resource
// which includes a list of diagnostics that detail issues with the abstract resource.
type AbstractResourceValidateOutput struct {
	Diagnostics []*core.Diagnostic
}

// AbstractResourceGetSpecDefinitionInput provides the input from providing a spec definition
// for an abstract resource.
type AbstractResourceGetSpecDefinitionInput struct {
	Params core.BlueprintParams
}

// AbstractResourceGetSpecDefinitionOutput provides the output from providing a spec definition
// for an abstract resource.
type AbstractResourceGetSpecDefinitionOutput struct {
	SpecDefinition *provider.ResourceSpecDefinition
}
