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
	// has been successfully updated.
	ResourceStatusUpdated
	// ResourceStatusUpdateFailed is used when a resource
	// has failed to update.
	ResourceStatusUpdateFailed
	// ResourceStatusRollingBack is used when
	// another change in the same blueprint has failed
	// and the latest change involving the current resource
	// is being rolled back.
	ResourceStatusRollingBack
	// ResourceStatusRollbackFailed is used when
	// another change in the same blueprint has failed
	// and the latest change involving the current resource
	// could not be rolled back.
	ResourceStatusRollbackFailed
	// ResourceStatusRollbackComplete is used when
	// another change in the same blueprint has failed
	// and the latest change involving the current resource
	// has been rolled back.
	ResourceStatusRollbackComplete
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
	// PreciseResourceStatusCreateRollingBack is used when
	// another change in the same blueprint has failed
	// and the current resource that was successfully
	// created is being rolled back.
	PreciseResourceStatusCreateRollingBack
	// PreciseResourceStatusCreateRollbackFailed is used when
	// another change in the same blueprint has failed
	// and the current resource that was successfully
	// created could not be rolled back.
	PreciseResourceStatusCreateRollbackFailed
	// PreciseResourceStatusCreateRollbackComplete is used when
	// another change in the same blueprint has failed
	// and the current resource that was successfully
	// created has been rolled back.
	PreciseResourceStatusCreateRollbackComplete
	// ResourceStatusDestroying is used when
	// a resource is in the process of being destroyed.
	PreciseResourceStatusDestroying
	// ResourceStatusDestroyed is used when
	// a resource has been destroyed.
	PreciseResourceStatusDestroyed
	// ResourceStatusDestroyFailed is used when
	// the destruction of a resource fails.
	PreciseResourceStatusDestroyFailed
	// PreciseResourceStatusDestroyRollingBack is used when
	// another change in the same blueprint has failed
	// and the current resource that was successfully
	// removed is being rolled back.
	// This involves recreating the resource from the previous state.
	PreciseResourceStatusDestroyRollingBack
	// PreciseResourceStatusDestroyRollbackFailed is used when
	// another change in the same blueprint has failed
	// and the current resource that was successfully
	// removed could not be rolled back (recreated).
	PreciseResourceStatusDestroyRollbackFailed
	// PreciseResourceStatusDestroyRollbackConfigComplete is used when
	// another change in the same blueprint has failed
	// and the current resource that was successfully
	// removed has been rolled back (recreated)
	// but is not yet in a stable state.
	PreciseResourceStatusDestroyRollbackConfigComplete
	// PreciseResourceStatusDestroyRollbackComplete is used when
	// another change in the same blueprint has failed
	// and the current resource that was successfully
	// removed has been rolled back (recreated).
	PreciseResourceStatusDestroyRollbackComplete
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
	// PreciseResourceStatusUpdateRollingBack is used when
	// another change in the same blueprint has failed
	// and the current resource that was successfully
	// updated is being rolled back to the previous state.
	PreciseResourceStatusUpdateRollingBack
	// PreciseResourceStatusUpdateRollbackFailed is used when
	// another change in the same blueprint has failed
	// and the current resource that was successfully
	// updated could not be rolled back.
	PreciseResourceStatusUpdateRollbackFailed
	// PreciseResourceStatusUpdateRollbackConfigComplete is used when
	// another change in the same blueprint has failed
	// and the current resource that was successfully
	// updated has been rolled back to the previous state
	// but is not yet in a stable state.
	PreciseResourceStatusUpdateRollbackConfigComplete
	// PreciseResourceStatusUpdateRollbackComplete is used when
	// another change in the same blueprint has failed
	// and the current resource that was successfully
	// updated has been rolled back to the previous state.
	PreciseResourceStatusUpdateRollbackComplete
)
