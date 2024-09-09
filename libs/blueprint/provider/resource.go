package provider

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

// ResourceInfo provides all the information needed for a resource
// including the blueprint schema data with annotations, labels
// and the spec as a core mapping node.
type ResourceInfo struct {
	// ResourceID holds the ID of a resource when in the context
	// of a blueprint instance when deploying or staging changes.
	// Sometimes staging changes is independent of an instance and is used to compare
	// two vesions of a blueprint in which
	// case the resource ID will be empty.
	ResourceID string
	// InstanceID holds the ID of the blueprint instance
	// that the current resource belongs to.
	InstanceID string
	// RevisionID holds the ID of the blueprint instance revision
	// that the current resource deployment belongs to.
	RevisionID     string
	SchemaResource *schema.Resource
}

// Resource provides the interface for a resource
// that a provider can contain which includes logic for validating,
// transforming, linking and deploying a resource.
type Resource interface {
	// CustomValidate provides support for custom validation that goes beyond
	// the spec schema validation provided by the resource's spec definition.
	CustomValidate(ctx context.Context, input *ResourceValidateInput) (*ResourceValidateOutput, error)
	// GetSpecDefinition retrieves the spec definition for a resource,
	// this is the first line of validation for a resource in a blueprint and is also
	// useful for validating references to a resource instance
	// in a blueprint and for providing definitions for docs and tooling.
	GetSpecDefinition(ctx context.Context, input *ResourceGetSpecDefinitionInput) (*ResourceGetSpecDefinitionOutput, error)
	// CanLinkTo specifices the list of resource types the current resource type
	// can link to.
	CanLinkTo(ctx context.Context, input *ResourceCanLinkToInput) (*ResourceCanLinkToOutput, error)
	// IsCommonTerminal specifies whether this resource is expected to have a common use-case
	// as a terminal resource that does not link out to other resources.
	// This is useful for providing useful warnings to users about their blueprints
	// without overloading them with warnings for all resources that don't have any outbound
	// links that could have.
	IsCommonTerminal(ctx context.Context, input *ResourceIsCommonTerminalInput) (*ResourceIsCommonTerminalOutput, error)
	// StageChanges must detail the changes that will be made when a deployment of the loaded blueprint
	// for the resource and blueprint instance provided in resourceInfo.
	StageChanges(ctx context.Context, input *ResourceStageChangesInput) (*ResourceStageChangesOutput, error)
	// GetType deals with retrieving the namespaced type for a resource in a blueprint spec.
	GetType(ctx context.Context, input *ResourceGetTypeInput) (*ResourceGetTypeOutput, error)
	// Deploy deals with deploying a resource with the upstream resource provider.
	// The behaviour of deploy is completely down to the implementation of a resource provider and how long
	// a resource is likely to take to deploy. The state will be synchronised periodically and will reflect the current
	// state for long running deployments that we won't be waiting around for.
	// Parameters are passed into Deploy for extra context, blueprint variables will have already
	// been substituted at this stage and must be used instead of the passed in params argument
	// to ensure consistency between the staged changes that are reviewed and the deployment itself.
	Deploy(ctx context.Context, input *ResourceDeployInput) (*ResourceDeployOutput, error)
	// GetExternalState deals with getting a revision of the state of the resource from the resource provider.
	// (e.g. AWS or Google Cloud)
	// The blueprint instance, resource and in same case the revision IDs should be
	// attached to the resource in the external provider
	// in order to fetch it's status and sync up.
	GetExternalState(ctx context.Context, input *ResourceGetExternalStateInput) (*ResourceGetExternalStateOutput, error)
	// Destroy deals with destroying a resource instance if its current
	// state is successfully deployed or cleaning up a corrupt or partially deployed
	// resource instance.
	Destroy(ctx context.Context, input *ResourceDestroyInput) error
}

// ResourceValidateParams provides the input data needed for a resource to
// be validated.
type ResourceValidateInput struct {
	SchemaResource *schema.Resource
	Params         core.BlueprintParams
}

// ResourceValidateOutput provides the output data from validating a resource
// which includes a list of diagnostics that detail issues with the resource.
type ResourceValidateOutput struct {
	Diagnostics []*core.Diagnostic
}

// ResourceGetSpecDefinitionInput provides the input data needed for a resource to
// provide a spec definition.
type ResourceGetSpecDefinitionInput struct {
	Params core.BlueprintParams
}

// ResourceGetSpecDefinitionOutput provides the output data from providing a spec definition
// for a resource.
type ResourceGetSpecDefinitionOutput struct {
	SpecDefinition *ResourceSpecDefinition
}

// ResourceCanLinkToInput provides the input data needed for a resource to
// determine what types of resources it can link to.
type ResourceCanLinkToInput struct {
	Params core.BlueprintParams
}

// ResourceCanLinkToOutput provides the output data from determining what types of resources
// a given resource can link to.
type ResourceCanLinkToOutput struct {
	CanLinkTo []string
}

// ResourceIsCommonTerminalInput provides the input data needed for a resource to
// determine if it is a common terminal resource.
type ResourceIsCommonTerminalInput struct {
	Params core.BlueprintParams
}

// ResourceIsCommonTerminalOutput provides the output data from determining if a resource
// is a common terminal resource.
type ResourceIsCommonTerminalOutput struct {
	IsCommonTerminal bool
}

// ResourceDeployInput provides the input data needed for a resource to
// be deployed.
type ResourceDeployInput struct {
	Changes *Changes
	Params  core.BlueprintParams
}

// ResourceStageChangesInput provides the input data needed for a resource to
// stage changes.
type ResourceStageChangesInput struct {
	ResourceInfo *ResourceInfo
	Params       core.BlueprintParams
}

// ResourceStageChangesOutput provides the output data from staging changes
// for a resource.
type ResourceStageChangesOutput struct {
	Changes *Changes
}

// ResourceGetTypeInput provides the input data needed for a resource to
// determine the type of a resource in a blueprint spec.
type ResourceGetTypeInput struct {
	Params core.BlueprintParams
}

// ResourceGetTypeOutput provides the output data from determining the type of a resource
// in a blueprint spec.
type ResourceGetTypeOutput struct {
	Type string
}

// ResourceDeployOutput provides the output data from deploying a resource.
type ResourceDeployOutput struct {
	State *state.ResourceState
}

// ResourceGetExternalStateInput provides the input data needed for a resource to
// get the external state of a resource.
type ResourceGetExternalStateInput struct {
	InstanceID string
	RevisionID string
	ResourceID string
}

// ResourceGetExternalStateOutput provides the output data from
// retrieving the external state of a resource.
type ResourceGetExternalStateOutput struct {
	State *state.ResourceState
}

// ResourceDestroyInput provides the input data needed to delete
// a resource.
type ResourceDestroyInput struct {
	InstanceID string
	RevisionID string
	ResourceID string
}

// Changes provides a set of modified fields along with a version
// of the resource schema (includes metadata labels and annotations) and spec
// that has already had all it's variables substituted.
type Changes struct {
	// AppliedResourceInfo provides a new version of the spec
	// and schema for which variable substitution has been applied
	// so the deploy phase has everything it needs to deploy the resource.
	AppliedResourceInfo *ResourceInfo
	MustRecreate        bool
	ModifiedFields      []string
	NewFields           []string
	RemovedFields       []string
	UnchangedFields     []string
	// OutboundLinkChanges holds a mapping
	// of the linked to resource name to any changes
	// that will be made to the link.
	OutboundLinkChanges map[string]*LinkChanges
}

// ResourceSpecDefinition provides a definition for a resource spec
// that can be used for validation, docs and tooling.
type ResourceSpecDefinition struct {
	// Schema holds the schema for the resource
	// specification.
	Schema *ResourceSpecSchema
}

// ResourceSpecSchema provides a schema that can be used to validate
// a resource spec.
type ResourceSpecSchema struct {
	// Type holds the type of the resource spec.
	Type ResourceSpecSchemaType
	// Label holds a human-readable label for the resource spec.
	Label string
	// Description holds a human-readable description for the resource spec
	// without any formatting.
	Description string
	// FormattedDescription holds a human-readable description for the resource spec
	// that is formatted with markdown.
	FormattedDescription string
	// Attributes holds a mapping of the attribute types for a resource spec
	// schema object.
	Attributes map[string]*ResourceSpecSchema
	// Items holds the schema for the items in a resource spec schema array.
	Items *ResourceSpecSchema
	// MapValues holds the schema for the values in a resource spec schema map.
	// Keys are always strings.
	MapValues *ResourceSpecSchema
	// OneOf holds a list of possible schemas for a resource spec item.
	// This is to be used with the "union" type.
	OneOf []*ResourceSpecSchema
	// Required holds a list of required attributes for a resource spec schema object.
	Required []string
	// Nullable specifies whether the resource spec schema can be null.
	Nullable bool
	// Default holds the default value for a resource spec schema,
	// this will be populated in the `Resource.Spec` mapping node
	// if the resource spec is missing a value
	// for a specific attribute or item in the spec.
	// The default value will not be used if the attribute is nil
	// and the schema is nullable, a nil pointer should not be used
	// for an empty value, pointers should be set when you want to explicitly
	// set a value to nil.
	Default interface{}
}

// ResourceSpecSchemaType holds the type of a resource schema.
type ResourceSpecSchemaType string

const (
	// ResourceSpecSchemaTypeString is for a schema string.
	ResourceSpecSchemaTypeString ResourceSpecSchemaType = "string"
	// ResourceSpecSchemaTypeInteger is for a schema integer.
	ResourceSpecSchemaTypeInteger ResourceSpecSchemaType = "integer"
	// ResourceSpecSchemaTypeFloat is for a schema float.
	ResourceSpecSchemaTypeFloat ResourceSpecSchemaType = "float"
	// ResourceSpecSchemaTypeBoolean is for a schema boolean.
	ResourceSpecSchemaTypeBoolean ResourceSpecSchemaType = "boolean"
	// ResourceSpecSchemaTypeMap is for a schema map.
	ResourceSpecSchemaTypeMap ResourceSpecSchemaType = "map"
	// ResourceSpecSchemaTypeObject is for a schema object.
	ResourceSpecSchemaTypeObject ResourceSpecSchemaType = "object"
	// ResourceSpecSchemaTypeArray is for a schema array.
	ResourceSpecSchemaTypeArray ResourceSpecSchemaType = "array"
	// ResourceSpecSchemaTypeUnion is for an element that can be one of
	// multiple schemas.
	ResourceSpecSchemaTypeUnion ResourceSpecSchemaType = "union"
)
