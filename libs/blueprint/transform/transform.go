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
	// CustomValidate a schema for an abstract resource that will be transformed.
	CustomValidate(ctx context.Context, input *AbstractResourceValidateInput) (*AbstractResourceValidateOutput, error)
	// GetSpecDefinition retrieves the spec definition for an abstract resource,
	// this is the first line of validation for a resource in a blueprint and is also
	// useful for validating references to an abstract resource instance
	// in a blueprint and for providing definitions for docs and tooling.
	GetSpecDefinition(
		ctx context.Context,
		input *AbstractResourceGetSpecDefinitionInput,
	) (*AbstractResourceGetSpecDefinitionOutput, error)
	// GetStateDefinition retrieves the output state definition for an abstract resource,
	// This exposes a collection of alias attributes that map to the underlying concrete
	// resource state attributes.
	GetStateDefinition(
		ctx context.Context,
		input *AbstractResourceGetStateDefinitionInput,
	) (*AbstractResourceGetStateDefinitionOutput, error)
	// CanLinkTo specifices the list of resource types the current resource type
	// can link to.
	CanLinkTo(ctx context.Context, input *AbstractResourceCanLinkToInput) (*AbstractResourceCanLinkToOutput, error)
	// IsCommonTerminal specifies whether this resource is expected to have a common use-case
	// as a terminal resource that does not link out to other resources.
	// This is useful for providing useful warnings to users about their blueprints
	// without overloading them with warnings for all resources that don't have any outbound
	// links that could have.
	IsCommonTerminal(
		ctx context.Context,
		input *AbstractResourceIsCommonTerminalInput,
	) (*AbstractResourceIsCommonTerminalOutput, error)
	// GetType deals with retrieving the namespaced type for an abstract
	// resource in a blueprint spec.
	GetType(ctx context.Context, input *AbstractResourceGetTypeInput) (*AbstractResourceGetTypeOutput, error)
	// GetTypeDescription deals with retrieving the description for a resource type in a blueprint spec
	// that can be used for documentation and tooling.
	// Markdown and plain text formats are supported.
	GetTypeDescription(
		ctx context.Context,
		input *AbstractResourceGetTypeDescriptionInput,
	) (*AbstractResourceGetTypeDescriptionOutput, error)
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

// AbstractResourceGetSpecDefinitionOutput provides the output from providing an
// output state definition for an abstract resource.
type AbstractResourceGetStateDefinitionOutput struct {
	StateDefinition *provider.ResourceStateDefinition
}

// AbstractResourceGetStateDefinitionInput provides the input from providing an output state
// definition for an abstract resource.
type AbstractResourceGetStateDefinitionInput struct {
	Params core.BlueprintParams
}

// AbstractResourceGetSpecDefinitionOutput provides the output from providing a spec definition
// for an abstract resource.
type AbstractResourceGetSpecDefinitionOutput struct {
	SpecDefinition *provider.ResourceSpecDefinition
}

// AbstractResourceCanLinkToInput provides the input data needed for a resource to
// determine what types of resources it can link to.
type AbstractResourceCanLinkToInput struct {
	Params core.BlueprintParams
}

// AbstractResourceCanLinkToOutput provides the output data from determining what types of resources
// a given resource can link to.
type AbstractResourceCanLinkToOutput struct {
	CanLinkTo []string
}

// AbstractResourceIsCommonTerminalInput provides the input data needed for a resource to
// determine if it is a common terminal resource.
type AbstractResourceIsCommonTerminalInput struct {
	Params core.BlueprintParams
}

// AbstractResourceIsCommonTerminalOutput provides the output data from determining if a resource
// is a common terminal resource.
type AbstractResourceIsCommonTerminalOutput struct {
	IsCommonTerminal bool
}

// AbstractResourceGetTypeInput provides the input data needed for an abstract resource to
// determine the type of a resource in a blueprint spec.
type AbstractResourceGetTypeInput struct {
	Params core.BlueprintParams
}

// AbstractResourceGetTypeOutput provides the output data from determining the type of an
// abstract resource in a blueprint spec.
type AbstractResourceGetTypeOutput struct {
	Type string
}

// AbstractResourceGetTypeDescriptionInput provides the input data needed for a resource to
// retrieve a description of the type of an abstract resource in a blueprint spec.
type AbstractResourceGetTypeDescriptionInput struct {
	Params core.BlueprintParams
}

// AbstractResourceGetTypeDescriptionOutput provides the output data from retrieving a description
// of the type of am abstract resource in a blueprint spec.
type AbstractResourceGetTypeDescriptionOutput struct {
	MarkdownDescription  string
	PlainTextDescription string
}
