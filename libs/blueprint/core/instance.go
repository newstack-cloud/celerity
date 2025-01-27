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
	// InstanceStatusDeployRollingBack is used when
	// a blueprint instance deployment has failed
	// and is being rolled back to a previous state.
	InstanceStatusDeployRollingBack
	// InstanceStatusDeployRollbackFailed is used when
	// a blueprint instance deployment has failed
	// and the rollback process has also failed.
	InstanceStatusDeployRollbackFailed
	// InstanceStatusDeployRollbackComplete is used when
	// a blueprint instance deployment has been successfully rolled back
	// to a previous state.
	InstanceStatusDeployRollbackComplete
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
	// InstanceStatusDestroyRollingBack is used when
	// a blueprint instance removal has failed
	// and is being rolled back to a previous state.
	InstanceStatusDestroyRollingBack
	// InstanceStatusDestroyRollbackFailed is used when
	// a blueprint instance removal has failed
	// and the rollback process has also failed.
	InstanceStatusDestroyRollbackFailed
	// InstanceStatusDeployRollbackComplete is used when
	// a blueprint instance removal has been successfully rolled back
	// to a previous state.
	InstanceStatusDestroyRollbackComplete
	// InstanceStatusUpdating is used when
	// a blueprint instance is being updated.
	InstanceStatusUpdating
	// InstanceStatusUpdated is used when a blueprint
	// instance has been sucessfully updated.
	InstanceStatusUpdated
	// InstanceStatusUpdateFailed is used when a blueprint
	// instance has failed to update.
	InstanceStatusUpdateFailed
	// InstanceStatusUpdateRollingBack is used when
	// a blueprint instance update has failed
	// and is being rolled back to a previous state.
	InstanceStatusUpdateRollingBack
	// InstanceStatusUpdateRollbackFailed is used when
	// a blueprint instance update has failed
	// and the rollback process has also failed.
	InstanceStatusUpdateRollbackFailed
	// InstanceStatusUpdateRollbackComplete is used when
	// a blueprint instance update has been successfully rolled back
	// to a previous state.
	InstanceStatusUpdateRollbackComplete
	// InstanceStatusNotDeployed is used when
	// a blueprint instance has not had its first deployment.
	// This is useful for persisting a skeleton for an instance
	// before the first deployment of a new blueprint instance.
	InstanceStatusNotDeployed
)
