package state

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/core"
)

// Container provides an interface for services
// that encapsulate blueprint instance state.
// The state persistence method is entirely up to the application
// making use of this library.
type Container interface {
	// GetResource deals with retrieving the state for a given resource
	// in the provided blueprint instance.
	// This retrieves the resource for the latest revision of the given instance.
	GetResource(ctx context.Context, instanceID string, resourceID string) (ResourceState, error)
	// GetResourceForRevision deals with retrieving the state for a given resource
	// in the provided blueprint instance revision.
	GetResourceForRevision(ctx context.Context, instanceID string, revisionId string, resourceID string) (ResourceState, error)
	// GetInstance deals with retrieving the state for a given blueprint
	// instance ID.
	// This retrieves the latest revision of an instance.
	GetInstance(ctx context.Context, instanceID string) (InstanceState, error)
	// GetInstanceRevision deals with retrieving the state for a specific revision
	// of a given blueprint instance.
	GetInstanceRevision(ctx context.Context, instanceID string, revisionID string) (InstanceState, error)
	// SaveInstance deals with persisting a blueprint instance.
	// This will create a new revision.
	SaveInstance(ctx context.Context, instanceID string, instanceState InstanceState) (InstanceState, error)
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
	SaveResource(ctx context.Context, instanceID string, resourceID string, resourceState ResourceState) error
	// RemoveResource deals with removing the state of a resource from
	// a given blueprint instance.
	// This removes the state for all blueprint instance revisions for the given resource.
	// There is no way to remove a resource from a specific instance revision,
	// the instance revision should be removed as a whole and recreated instead.
	RemoveResource(ctx context.Context, instanceID string, resourceID string) (ResourceState, error)
}

// ResourceState provides the current state of a resource
// in a blueprint instance.
// This includes the status, the Raw data from the downstream resouce provider
// along with reasons for failure when a resource is in a failure state.
type ResourceState struct {
	Status core.ResourceStatus
	// ResourceData is the mapping that holds the structure of
	// the "raw" resource data from the resource provider service.
	// (e.g. AWS Lambda Function object)
	ResourceData   map[string]interface{}
	FailureReasons []string
}

// InstanceState stores the state of a blueprint instance
// which is a mapping of resource IDs to resource state.
type InstanceState map[string]*ResourceState
