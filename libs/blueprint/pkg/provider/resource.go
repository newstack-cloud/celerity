package provider

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/state"
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
	// Validate a resource's specification.
	Validate(ctx context.Context, schemaResource *schema.Resource, params core.BlueprintParams) ([]*core.Diagnostic, error)
	// CanLinkTo specifices the list of resource types the current resource type
	// can link to.
	CanLinkTo() []string
	// IsCommonTerminal specifies whether this resource is expected to have a common use-case
	// as a terminal resource that does not link out to other resources.
	// This is useful for providing useful warnings to users about their blueprints
	// without overloading them with warnings for all resources that don't have any outbound
	// links that could have.
	IsCommonTerminal() bool
	// StageChanges must detail the changes that will be made when a deployment of the loaded blueprint
	// for the resource and blueprint instance provided in resourceInfo.
	StageChanges(
		ctx context.Context,
		resourceInfo *ResourceInfo,
		params core.BlueprintParams,
	) (Changes, error)
	// GetType deals with retrieving the namespaced type for a resource in a blueprint spec.
	GetType() string
	// Deploy deals with deploying a resource with the upstream resource provider.
	// The behaviour of deploy is completely down to the implementation of a resource provider and how long
	// a resource is likely to take to deploy. The state will be synchronised periodically and will reflect the current
	// state for long running deployments that we won't be waiting around for.
	// Parameters are passed into Deploy for extra context, blueprint variables will have already
	// been substituted at this stage and must be used instead of the passed in params argument
	// to ensure consistency between the staged changes that are reviewed and the deployment itself.
	Deploy(ctx context.Context, changes Changes, params core.BlueprintParams) (state.ResourceState, error)
	// GetExternalState deals with getting a revision of the state of the resource from the resource provider.
	// (e.g. AWS or Google Cloud)
	// The blueprint instance, resource and in same case the revision IDs should be
	// attached to the resource in the external provider
	// in order to fetch it's status and sync up.
	GetExternalState(ctx context.Context, instanceID string, revisionID string, resourceID string) (state.ResourceState, error)
	// Destroy deals with destroying a resource instance if it's current
	// state is successfully deployed or cleaning up a corrupt or partially deployed
	// resource instance.
	Destroy(ctx context.Context, instanceID string, revisionID string, resourceID string) error
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
