package providerv1

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

// LinkDefinition is a template to be used for defining links
// between resources when creating provider plugins.
// It provides a structure that allows you to define a schema and behaviour
// of a link.
// This implements the `provider.Link` interface and can be used in the same way
// as any other link implementation used in a provider plugin.
type LinkDefinition struct {
	// The type of the link that should consist of the types of the resources
	// in the relationship, prefixed by the namespace of the provider.
	// Example: "aws/lambda/function::aws/dynamodb/table" for a link between
	// an AWS Lambda function and an AWS DynamoDB table.
	Type string

	// The kind of link that contributes to the ordering of resources during deployment.
	// This can either be "hard" or "soft".
	// Hard links require the priority resource must exist before the dependent resource
	// can be created.
	// Soft links do not require either of the resources to exist before the other.
	Kind provider.LinkKind

	// The priority resource in the relationship based on the ordering of the resource
	// types.
	// For example in the link type "aws/lambda/function::aws/dynamodb/table",
	// if the priority resource should be "aws/lambda/function", then this field
	// should be set to `provider.LinkPriorityResourceA`.
	// If there is no priority resource, this field should be set to
	// `provider.LinkPriorityResourceNone`.
	// This will not be used if PriorityResourceFunc is provided.
	PriorityResource provider.LinkPriorityResource

	// A function that can be used to dynamically determine the priority resource
	// in the relationship based on the ordering of the resource types.
	// This will not be used if PriorityResourceFunc is provided.
	PriorityResourceType string

	// A function that can be used to dynamically determine the priority resource
	// in the link relationship.
	PriorityResourceFunc func(
		ctx context.Context,
		input *provider.LinkGetPriorityResourceInput,
	) (*provider.LinkGetPriorityResourceOutput, error)

	// A function that details the changes that will be made when a deployment of the loaded blueprint
	// for the link between two resources.
	// Unlike resources, links do not map to a specification for a single deployable unit,
	// so link implementations must specify the changes that will be made across multiple resources.
	StageChangesFunc func(
		ctx context.Context,
		input *provider.LinkStageChangesInput,
	) (*provider.LinkStageChangesOutput, error)

	// A function that deals with applying the changes to the first of the two linked resources
	// for the creation or removal of a link between two resources.
	// The value of the `LinkData` field returned in the output will be combined
	// with the LinkData output from updating resource B and intermediary resources
	// to form the final LinkData that will be persisted in the state of the blueprint instance.
	// Parameters are passed into UpdateResourceA for extra context, blueprint variables will have already
	// been substituted at this stage and must be used instead of the passed in params argument
	// to ensure consistency between the staged changes that are reviewed and the deployment itself.
	UpdateResourceAFunc func(
		ctx context.Context,
		input *provider.LinkUpdateResourceInput,
	) (*provider.LinkUpdateResourceOutput, error)

	// A function that deals with applying the changes to the second of the two linked resources
	// for the creation or removal of a link between two resources.
	// The value of the `LinkData` field returned in the output will be combined
	// with the LinkData output from updating resource A and intermediary resources
	// to form the final LinkData that will be persisted in the state of the blueprint instance.
	// Parameters are passed into UpdateResourceB for extra context, blueprint variables will have already
	// been substituted at this stage and must be used instead of the passed in params argument
	// to ensure consistency between the staged changes that are reviewed and the deployment itself.
	UpdateResourceBFunc func(
		ctx context.Context,
		input *provider.LinkUpdateResourceInput,
	) (*provider.LinkUpdateResourceOutput, error)

	// A function that deals with creating, updating or deleting intermediary resources
	// that are required for the link between two resources.
	// This is called for both the creation and removal of a link between two resources.
	// The value of the `LinkData` field returned in the output will be combined
	// with the LinkData output from updating resource A and B
	// to form the final LinkData that will be persisted in the state of the blueprint instance.
	// Parameters are passed into UpdateIntermediaryResources for extra context, blueprint variables will have already
	// been substituted at this stage and must be used instead of the passed in params argument
	// to ensure consistency between the staged changes that are reviewed and the deployment itself.
	UpdateIntermediaryResourceFunc func(
		ctx context.Context,
		input *provider.LinkUpdateResourceInput,
	) (*provider.LinkUpdateResourceOutput, error)
}

func (l *LinkDefinition) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{
		Type: l.Type,
	}, nil
}

func (l *LinkDefinition) GetKind(
	ctx context.Context,
	input *provider.LinkGetKindInput,
) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		Kind: l.Kind,
	}, nil
}

func (l *LinkDefinition) GetPriorityResource(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceInput,
) (*provider.LinkGetPriorityResourceOutput, error) {
	if l.PriorityResourceFunc != nil {
		return l.PriorityResourceFunc(ctx, input)
	}

	return &provider.LinkGetPriorityResourceOutput{
		PriorityResource:     l.PriorityResource,
		PriorityResourceType: l.PriorityResourceType,
	}, nil
}

func (l *LinkDefinition) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return l.StageChangesFunc(ctx, input)
}

func (l *LinkDefinition) UpdateResourceA(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return l.UpdateResourceAFunc(ctx, input)
}

func (l *LinkDefinition) UpdateResourceB(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return l.UpdateResourceBFunc(ctx, input)
}

func (l *LinkDefinition) UpdateIntermediaryResource(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return l.UpdateIntermediaryResourceFunc(ctx, input)
}
