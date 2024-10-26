package state

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
)

// Container provides an interface for services
// that encapsulate blueprint instance state.
// The state persistence method is entirely up to the application
// making use of this library.
type Container interface {
	// GetResource deals with retrieving the state for a given resource
	// in the provided blueprint instance.
	// This retrieves the resource for the latest revision of the given instance.
	GetResource(ctx context.Context, instanceID string, resourceID string) (*ResourceState, error)
	// GetResourceForRevision deals with retrieving the state for a given resource
	// in the provided blueprint instance revision.
	GetResourceForRevision(ctx context.Context, instanceID string, revisionID string, resourceID string) (*ResourceState, error)
	// GetLink deals with retrieving the state for a given link
	// in the provided blueprint instance.
	GetLink(ctx context.Context, instanceID string, linkID string) (*LinkState, error)
	// GetLinkForRevision deals with retrieving the state for a given link
	// in the provided blueprint instance revision.
	GetLinkForRevision(ctx context.Context, instanceID string, revisionID string, linkID string) (*LinkState, error)
	// GetInstance deals with retrieving the state for a given blueprint
	// instance ID.
	// This retrieves the latest revision of an instance.
	GetInstance(ctx context.Context, instanceID string) (*InstanceState, error)
	// GetInstanceRevision deals with retrieving the state for a specific revision
	// of a given blueprint instance.
	GetInstanceRevision(ctx context.Context, instanceID string, revisionID string) (*InstanceState, error)
	// SaveInstance deals with persisting a blueprint instance.
	// This will create a new revision.
	SaveInstance(ctx context.Context, instanceID string, instanceState InstanceState) (*InstanceState, error)
	// RemoveInstance deals with removing the state for a given blueprint instance.
	// This is not for destroying the actual deployed resources, just removing the state.
	// This deals with removing all blueprint instance revisions.
	RemoveInstance(ctx context.Context, instanceID string) error
	// RemoveInstanceRevision deals with removing the state for a specific revision
	// of a blueprint instance.
	// This is not for destroying actual deployed resources, just removing the state.
	RemoveInstanceRevision(ctx context.Context, instanceID string, revisionID string) error
	// SaveResource deals with persisting a resource in a blueprint instance.
	// This covers adding new resources and updating existing resources in the latest revision
	// in an immutable fashion.
	// This should always create a new blueprint instance revision.
	SaveResource(ctx context.Context, instanceID string, resourceID string, resourceState *ResourceState) error
	// RemoveResource deals with removing the state of a resource from
	// a given blueprint instance.
	// This removes the state for all blueprint instance revisions for the given resource.
	// There is no way to remove a resource from a specific instance revision,
	// the instance revision should be removed as a whole and recreated instead.
	RemoveResource(ctx context.Context, instanceID string, resourceID string) (*ResourceState, error)
	// CleanupRevisions deals with removing old revisions of a blueprint instance
	// based on a retention policy.
	// Applications using this library should implement functionality that facilitates
	// retention policies for blueprint instance revisions, the blueprint framework
	// only provides the interface to remove old revisions.
	CleanupRevisions(ctx context.Context, instanceID string) error
}

// ResourceState provides the current state of a resource
// in a blueprint instance.
// This includes the status, the Raw data from the upstream resouce provider
// along with reasons for failure when a resource is in a failure state.
type ResourceState struct {
	ResourceID string
	Status     core.ResourceStatus
	// ResourceData is the mapping that holds the structure of
	// the "raw" resource data from the resource provider service.
	// (e.g. AWS Lambda Function object)
	ResourceData map[string]*core.MappingNode
	// Holds the latest reasons for failures in deploying a resource,
	// this only ever holds the results of the latest deployment attempt.
	FailureReasons []string
}

// InstanceState stores the state of a blueprint instance
// including resources, metadata, exported fields and child blueprints.
type InstanceState struct {
	InstanceID string
	RevisionID string
	Status     core.InstanceStatus
	Resources  map[string]*ResourceState
	Links      map[string]*LinkState
	// Metadata is used internally to store additional non-structured information
	// that is relevant to the blueprint framework but can also be used to store
	// additional information that is relevant to the application/tool
	// making use of this library.
	Metadata        map[string]*core.MappingNode
	Exports         map[string]*core.MappingNode
	ChildBlueprints map[string]*InstanceState
}

// LinkState provides a way to store some state for links between
// resources.
// This is useful for holding state about intermediary resources
// managed by a provider's implementation of a link.
type LinkState struct {
	// IntermediaryResourceStates holds the state of intermediary resources
	// that are created by the provider's implementation of a link.
	IntermediaryResourceStates []*ResourceState
	// ResourceData is the mapping that holds the structure of
	// the "raw" link data to hold information about a link that is not
	// stored directly in the resources that are linked and is not
	// stored in intermediary resources.
	LinkData map[string]*core.MappingNode
}
