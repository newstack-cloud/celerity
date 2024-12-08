package provider

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

// Link provides the interface for the implementation of a link between two resources.
// This provides error handling methods in order to roll back changes to resource A and resource B,
// however, as intermediary resources always come after resource A and B updates,
// any errors that could cause inconsistencies between multiple intermediary resources
// should be handled by the link implementation.
type Link interface {
	// StageChanges must detail the changes that will be made when a deployment of the loaded blueprint
	// for the link between two resources and blueprint instance provided in resourceInfo.
	// Unlike resources, links do not map to a specification for a single deployable unit,
	// so link implementations must specify the changes that will be made across multiple resources.
	StageChanges(
		ctx context.Context,
		input *LinkStageChangesInput,
	) (*LinkStageChangesOutput, error)
	// UpdateResourceA deals with applying the changes to the first of the two linked resources
	// for the creation or removal of a link between two resources.
	// Parameters are passed into UpdateResourceA for extra context, blueprint variables will have already
	// been substituted at this stage and must be used instead of the passed in params argument
	// to ensure consistency between the staged changes that are reviewed and the deployment itself.
	UpdateResourceA(ctx context.Context, input *LinkUpdateResourceInput) (*LinkUpdateResourceOutput, error)
	// UpdateResourceB deals with applying the changes to the second of the two linked resources
	// for the creation or removal of a link between two resources.
	// Parameters are passed into UpdateResourceA for extra context, blueprint variables will have already
	// been substituted at this stage and must be used instead of the passed in params argument
	// to ensure consistency between the staged changes that are reviewed and the deployment itself.
	UpdateResourceB(ctx context.Context, input *LinkUpdateResourceInput) (*LinkUpdateResourceOutput, error)
	// UpdateIntermediaryResources deals with creating, updating or deleting intermediary resources
	// that are required for the link between two resources.
	// This is called for both the creation and removal of a link between two resources.
	// Parameters are passed into UpdateResourceA for extra context, blueprint variables will have already
	// been substituted at this stage and must be used instead of the passed in params argument
	// to ensure consistency between the staged changes that are reviewed and the deployment itself.
	UpdateIntermediaryResources(
		ctx context.Context,
		input *LinkUpdateIntermediaryResourcesInput,
	) (*LinkUpdateIntermediaryResourcesOutput, error)
	// GetPriorityResourceType retrieves the resource type in the relationship
	// that must be deployed first. This will be empty for links where one resource type does not
	// need to be deployed before the other.
	GetPriorityResourceType(ctx context.Context, input *LinkGetPriorityResourceTypeInput) (*LinkGetPriorityResourceTypeOutput, error)
	// GetType deals with retrieving the type of the link in relation to the two resource
	// types it provides a relationship between.
	GetType(ctx context.Context, input *LinkGetTypeInput) (*LinkGetTypeOutput, error)
	// GetKind tells us whether the link is a "hard" or "soft" link.
	// A hard link is where the priority resource type must be created first.
	// A soft link is where it does not matter which resource type in the relationship
	// is created first.
	GetKind(ctx context.Context, input *LinkGetKindInput) (*LinkGetKindOutput, error)
	// HandleResourceAError deals with handling errors in
	// the deployment of the first of the two linked resources.
	// This is useful to roll back changes made to resource B in the case
	// resource B was updated first.
	// This will also be called on failure to update intermediary resources.
	HandleResourceAError(ctx context.Context, input *LinkHandleResourceErrorInput) error
	// HandleResourceTypeBError deals with handling errors
	// in the second of the two linked resources.
	// This is useful to roll back changes made to resource A in the case
	// resource A was updated first.
	// This will also be called on failure to update intermediary resources.
	HandleResourceBError(ctx context.Context, input *LinkHandleResourceErrorInput) error
}

// LinkStageChangesInput provides the input required to
// stage changes for a link between two resources.
type LinkStageChangesInput struct {
	ResourceAChanges *Changes
	ResourceBChanges *Changes
	CurrentLinkState *state.LinkState
	Params           core.BlueprintParams
}

// LinkStageChangesOutput provides the output from staging changes
// for a link between two resources.
type LinkStageChangesOutput struct {
	Changes *LinkChanges
}

// LinkUpdateResourceInput provides the input required to
// update a resource in a link relationship
// with data that will contribute to "activating" the link.
type LinkUpdateResourceInput struct {
	Changes      *LinkChanges
	ResourceInfo *ResourceInfo
	Params       core.BlueprintParams
}

// LinkUpdateResourceOutput provides the output from updating
// a resource in a link relationship.
type LinkUpdateResourceOutput struct {
	LinkData *core.MappingNode
}

// LinkUpdateIntermediaryResourcesInput provides the input required to
// update intermediary resources in a link relationship.
type LinkUpdateIntermediaryResourcesInput struct {
	ResourceAInfo *ResourceInfo
	ResourceBInfo *ResourceInfo
	Changes       *LinkChanges
	Params        core.BlueprintParams
}

type LinkUpdateIntermediaryResourcesOutput struct {
	IntermediaryResourceStates []*state.LinkIntermediaryResourceState
	LinkData                   *core.MappingNode
}

// LinkPriorityResourceTypeOutput provides the input for retrieving
// the priority resource type in a link relationship.
type LinkGetPriorityResourceTypeInput struct {
	Params core.BlueprintParams
}

// LinkPriorityResourceTypeOutput provides the output for retrieving
// the priority resource type in a link relationship.
type LinkGetPriorityResourceTypeOutput struct {
	PriorityResourceType string
}

// LinkGetKindInput provides the input for retrieving the kind of link.
type LinkGetKindInput struct {
	Params core.BlueprintParams
}

// LinkGetKindOutput provides the output for retrieving the kind of link.
type LinkGetKindOutput struct {
	Kind LinkKind
}

// LinkGetTypeOutput provides the output for retrieving the type of link
// with respect to the two resource types it provides a relationship between.
type LinkGetTypeInput struct {
	Params core.BlueprintParams
}

// LinkGetTypeOutput provides the output for retrieving the type of link
// with respect to the two resource types it provides a relationship between.
type LinkGetTypeOutput struct {
	Type string
}

// LinkHandleResourceErrorInput provides the input for handling errors
// related to the deployment of a resource type in a link relationship.
type LinkHandleResourceErrorInput struct {
	ResourceInfo *ResourceInfo
	Params       core.BlueprintParams
}

// LinkKind provides a way to categorise links to help determine the order
// in which resources need to be deployed when a blueprint instance is being deployed.
type LinkKind string

const (
	// LinkKindHard is the type of link where the priority resource type
	// must be created before the other resource type in the relationship.
	LinkKindHard LinkKind = "hard"
	// LinkKindSoft is the type of link where it does not matter
	// which of the two resource types in the relationship is created
	// first.
	LinkKindSoft LinkKind = "soft"
)

// LinkChanges provides a set of modified fields for a link between two resources.
// The link field changes represent a set of changes that will be made to the
// resources in the link relationship, these changes should be modelled as per the
// structure of the linkData that is persisted in the state of a blueprint instance.
// The linkData model should be organised by the resource type with a structure
// that is a close approximation of the actual changes that will be made to each
// resource during deployment in the upstream provider.
type LinkChanges struct {
	ModifiedFields  []*FieldChange `json:"modifiedFields"`
	NewFields       []*FieldChange `json:"newFields"`
	RemovedFields   []string       `json:"removedFields"`
	UnchangedFields []string       `json:"unchangedFields"`
	// FieldChangesKnownOnDeploy holds a list of field names
	// for which changes will be known when the host blueprint is deployed.
	FieldChangesKnownOnDeploy []string `json:"fieldChangesKnownOnDeploy"`
}
