package providerv1

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/build-engine/plugin/sdk/schema"
)

// ResourceDefinition is a template to be used for defining resources
// when creating provider plugins.
// It provides a structure that allows you to define a schema and behaviour
// of a resource.
type ResourceDefinition struct {
	// The type of the resource, prefixed by the namespace of the provider.
	// Example: "aws/lambda/function" for the "aws" provider.
	Type string

	// The schema of the resource used for validation.
	Schema map[string]*schema.Schema

	// Specifies whether this resource is expected to have a common use-case
	// as a terminal resource that does not link out to other resources.
	// This is useful for providing useful warnings to users about their blueprints
	// without overloading them with warnings for all resources that don't have any outbound
	// links that could have.
	CommonTerminal bool

	// A function that can be used to dynamically determine whether a resource is likely
	// to be a common terminal resource (does not link out to other resources).
	CommonTerminalFunc func(ctx context.Context, params core.BlueprintParams) (bool, error)

	// A static list of resource types that this resource can link to.
	CanLinkTo []string

	// A function that can be used to dynamically determine what resource types this resource
	// can link to.
	CanLinkToFunc func(
		ctx context.Context,
		params core.BlueprintParams,
	) ([]string, error)

	// A function to stage the changes for a resource prior to deployment.
	// This will diff against the current resource state and the desired state
	// and indicate what is likely to change.
	// In a lot of cases, there are values that are not known until the resource is deployed
	// so it is best to given an estimate of what will change.
	StageChangesFunc func(
		ctx context.Context,
		input *provider.ResourceStageChangesInput,
	) (*provider.ResourceStageChangesOutput, error)

	// A function to retrieve the current state of the resource from the upstream system.
	// (e.g. fetch the state of an Lambda Function from AWS)
	GetExternalStateFunc func(
		ctx context.Context,
		input *provider.ResourceGetExternalStateInput,
	) (*provider.ResourceGetExternalStateOutput, error)

	// A function to deploy the resource.
	DeployFunc func(
		ctx context.Context,
		input *provider.ResourceDeployInput,
	) (*provider.ResourceDeployOutput, error)

	// A function to delete the resource in the upstream provider.
	DestroyFunc func(
		ctx context.Context,
		input *provider.ResourceDestroyInput,
	) error
}

// func ResourceFromDefinition(definition ResourceDefinition) provider.Resource {
// 	return &resourceDefWrapperImpl{
// 		definition: definition,
// 	}
// }

// type resourceDefWrapperImpl struct {
// 	definition ResourceDefinition
// }

// func (r *resourceDefWrapperImpl) GetType(
// 	ctx context.Context,
// 	input *provider.ResourceGetTypeInput,
// ) (*provider.ResourceGetTypeOutput, error) {
// 	return &provider.ResourceGetTypeOutput{
// 		Type: r.definition.Type,
// 	}, nil
// }
