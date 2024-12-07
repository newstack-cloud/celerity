package core

// InstanceStatus is used to represent the current state of a
// blueprint instance.
type InstanceStatus int

const (
	// InstanceStatusPreparing is used when a blueprint
	// instance is being prepared to be deployed, updated
	// or destroyed.
	InstanceStatusPreparing InstanceStatus = iota
	// InstanceStatusDeploying is used when
	// an initial blueprint deployment is currently in progress.
	InstanceStatusDeploying
	// InstanceStatusDeployed is used when
	// a blueprint instance has been deployed
	// successfully.
	InstanceStatusDeployed
	// InstanceStatusDeployFailed is used when
	// the first deployment of a blueprint instance failed.
	InstanceStatusDeployFailed
	// InstanceStatusDestroying is used when
	// all the resources defined in a blueprint
	// are in the process of being destroyed
	// for a given instance.
	InstanceStatusDestroying
	// InstanceStatusDestroyed is used when
	// all resources defined in a blueprint have been destroyed
	// for a given instance.
	InstanceStatusDestroyed
	// InstanceStatusDestroyFailed is used when
	// the destruction of all resources in a blueprint fails.
	InstanceStatusDestroyFailed
	// InstanceStatusUpdating is used when
	// a blueprint instance is being updated.
	InstanceStatusUpdating
	// InstanceStatusUpdated is used when a blueprint
	// instance has been sucessfully updated.
	InstanceStatusUpdated
	// InstanceStatusUpdateFailed is used when a blueprint
	// instance has failed to update.
	InstanceStatusUpdateFailed
)
