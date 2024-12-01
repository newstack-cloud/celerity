package core

// ResourceStatus is used to represent the current state of a resource
// in a blueprint instance.
type ResourceStatus int

const (
	// ResourceStatusUnknown is used when we can't
	// determine an accurate status for a resource.
	ResourceStatusUnknown ResourceStatus = iota
	// ResourceStatusCreating is used when
	// an initial resource deployment is currently in progress.
	ResourceStatusCreating
	// ResourceStatusCreated is used when
	// a resource has been deployed
	// successfully.
	ResourceStatusCreated
	// ResourceStatusCreateFailed is used when
	// the first creation of a resource failed.
	ResourceStatusCreateFailed
	// ResourceStatusDestroying is used when
	// a resource is in the process of being destroyed.
	ResourceStatusDestroying
	// ResourceStatusDestroyed is used when
	// a resource has been destroyed.
	ResourceStatusDestroyed
	// ResourceStatusDestroyFailed is used when
	// the destruction of a resource fails.
	ResourceStatusDestroyFailed
	// ResourceStatusUpdating is used when
	// a resource is being updated.
	ResourceStatusUpdating
	// ResourceStatusUpdated is used when a resource
	// has been sucessfully updated.
	ResourceStatusUpdated
	// ResourceStatusUpdateFailed is used when a resource
	// has failed to update.
	ResourceStatusUpdateFailed
)

// PreciseResourceStatus is used to represent a more precise
// current state of a resource in a blueprint instance.
// This is used to allow the container "engine" to be more efficient
// in deploying a blueprint, by avoiding blocking on resource finalisation
// that isn't always needed to be able to successfully deploy the resources
// that are dependent on the resource in question.
type PreciseResourceStatus int

const (
	// PreciseResourceStatusUnknown is used when we can't
	// determine an accurate status for a resource.
	PreciseResourceStatusUnknown PreciseResourceStatus = iota
	// PreciseResourceStatusCreating is used when
	// an initial resource deployment is currently in progress.
	PreciseResourceStatusCreating
	// PreciseResourceStatusConfigComplete is used when
	// a resource has been configured successfully.
	// What this means is that the resource has been created
	// but is not yet in a stable state.
	// For example, an application in a container orchestration service
	// has been created but is not yet up and running.
	PreciseResourceStatusConfigComplete
	// ResourceStatusCreated is used when
	// a resource has been deployed
	// successfully.
	// This is used when a resource is in a stable state.
	PreciseResourceStatusCreated
	// ResourceStatusCreateFailed is used when
	// the first creation of a resource failed.
	PreciseResourceStatusCreateFailed
	// ResourceStatusDestroying is used when
	// a resource is in the process of being destroyed.
	PreciseResourceStatusDestroying
	// ResourceStatusDestroyed is used when
	// a resource has been destroyed.
	PreciseResourceStatusDestroyed
	// ResourceStatusDestroyFailed is used when
	// the destruction of a resource fails.
	PreciseResourceStatusDestroyFailed
	// ResourceStatusUpdating is used when
	// a resource is being updated.
	PreciseResourceStatusUpdating
	// PreciseResourceStatusUpdateConfigComplete is used when
	// a resource being updated has been configured successfully.
	// What this means is that the resource has been updated
	// but is not yet in a stable state.
	// For example, an application in a container orchestration service
	// has been updated but the new version is not yet up and running.
	PreciseResourceStatusUpdateConfigComplete
	// ResourceStatusUpdated is used when a resource
	// has been sucessfully updated.
	PreciseResourceStatusUpdated
	// ResourceStatusUpdateFailed is used when a resource
	// has failed to update.
	PreciseResourceStatusUpdateFailed
)
