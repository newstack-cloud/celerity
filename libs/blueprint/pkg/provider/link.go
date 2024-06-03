package provider

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/state"
)

type Link interface {
	// StageChanges must detail the changes that will be made when a deployment of the loaded blueprint
	// for the link between two resources and blueprint instance provided in resourceInfo.
	StageChanges(
		ctx context.Context,
		resourceAInfo *ResourceInfo,
		resourceBInfo *ResourceInfo,
		params core.BlueprintParams,
	) (LinkChanges, error)
	// Deploy deals with deploying a link between two resources in the upstream provider.
	// The behaviour of deploy is completely down to the implementation of a link provider and how long
	// a link is likely to take to deploy. The state will be synchronised periodically and will reflect the current
	// state for long running deployments that we won't be waiting around for.
	// Parameters are passed into Deploy for extra context, blueprint variables will have already
	// been substituted at this stage and must be used instead of the passed in params argument
	// to ensure consistency between the staged changes that are reviewed and the deployment itself.
	Deploy(
		ctx context.Context,
		changes LinkChanges,
		resourceAInfo *ResourceInfo,
		resourceBInfo *ResourceInfo,
		params core.BlueprintParams,
	) (state.ResourceState, error)
	// PriorityResourceType retrieves the resource type in the relationship
	// that must be deployed first. This will be empty for links where one resource type does not
	// need to be deployed before the other.
	PriorityResourceType() string
	// Type tells us whether the link is "hard" or "soft" link.
	// A hard link is where the priority resource type must be created first.
	// A soft link is where it does not matter which resource type in the relationship
	// is created first.
	Type() LinkType
	// HandleResourceTypeAError deals with handling errors in
	// the deployment of the first of the two linked resources.
	HandleResourceTypeAError(ctx context.Context, resourceInfo *ResourceInfo) error
	// HandleResourceTypeBError deals with handling errors
	// in the second of the two linked resources.
	HandleResourceTypeBError(ctx context.Context, resourceInfo *ResourceInfo) error
}

// LinkType provides a way to categorise links to help determine the order
// in which resources need to be deployed when a blueprint instance is being deployed.
type LinkType string

const (
	// LinkTypeHard is the type of link where the priority resource type
	// must be created before the other resource type in the relationship.
	LinkTypeHard LinkType = "hard"
	// LinkTypeSoft is the type of link where it does not matter
	// which of the two resource types in the relationship is created
	// first.
	LinkTypeSoft LinkType = "soft"
)

// Changes provides a set of modified fields along with a version
// of the resource schema (includes metadata labels and annotations) and spec
// that has already had all it's variables substituted.
type LinkChanges struct {
	ResourceTypeAModifiedFields  []string
	ResourceTypeANewFields       []string
	ResourceTypeARemovedFields   []string
	ResourceTypeAUnchangedFields []string
	ResourceTypeBModifiedFields  []string
	ResourceTypeBNewFields       []string
	ResourceTypeBRemovedFields   []string
	ResourceTypeBUnchangedFields []string
}
