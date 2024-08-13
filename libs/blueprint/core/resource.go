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
