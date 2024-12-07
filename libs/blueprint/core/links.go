package core

// LinkStatus is used to represent the current state of a link
// in a blueprint instance.
type LinkStatus int

const (
	// LinkStatusUnknown is used when we can't
	// determine an accurate status for a link.
	LinkStatusUnknown ResourceStatus = iota
	// LinkStatusCreating is used when
	// an initial link deployment is currently in progress.
	LinkStatusCreating
	// LinkStatusCreated is used when
	// a resource has been deployed
	// successfully.
	LinkStatusCreated
	// LinkStatusCreateFailed is used when
	// the first creation of a link failed.
	LinkStatusCreateFailed
	// LinkStatusDestroying is used when
	// a link is in the process of being destroyed.
	LinkStatusDestroying
	// LinkStatusDestroyed is used when
	// a link has been destroyed.
	LinkStatusDestroyed
	// LinkStatusDestroyFailed is used when
	// the destruction of a link fails.
	LinkStatusDestroyFailed
	// LinkStatusUpdating is used when
	// a link is being updated.
	LinkStatusUpdating
	// LinkStatusUpdated is used when a link
	// has been sucessfully updated.
	LinkStatusUpdated
	// LinkStatusUpdateFailed is used when a link
	// has failed to update.
	LinkStatusUpdateFailed
)

// PreciseLinkStatus is used to represent a more precise
// current state of a link in a blueprint instance.
type PreciseLinkStatus int

const (
	// PreciseLinkStatusUnknown is used when we can't
	// determine an accurate status for a link.
	PreciseLinkStatusUnknown ResourceStatus = iota
	// PreciseLinkStatusUpdatingResourceA is used when
	// the configuration for a link is being applied to resource A
	// in the link.
	PreciseLinkStatusUpdatingResourceA
	// PreciseLinkStatusResourceAUpdated is used when
	// the configuration for a link has been applied to resource A
	// in the link.
	PreciseLinkStatusResourceAUpdated
	// PreciseLinkStatusResourceAUpdateFailed is used when
	// the configuration for a link has failed to be applied to resource A
	// in the link.
	PreciseLinkStatusResourceAUpdateFailed
	// PreciseLinkStatusUpdatingResourceB is used when
	// the configuration for a link is being applied to resource B
	// in the link.
	PreciseLinkStatusUpdatingResourceB
	// PreciseLinkStatusResourceBUpdated is used when
	// the configuration for a link has been applied to resource B
	// in the link.
	PreciseLinkStatusResourceBUpdated
	// PreciseLinkStatusResourceBUpdateFailed is used when
	// the configuration for a link has failed to be applied to resource B
	// in the link.
	PreciseLinkStatusResourceBUpdateFailed
	// PreciseLinkStatusUpdatingIntermediaryResources is used when
	// intermediary resources are being created, updated or destroyed.
	// This status is a high level indication of process,
	// the status of each intermediary resource should be checked
	// to determine the exact state of each intermediary resource
	// in the link.
	PreciseLinkStatusUpdatingIntermediaryResources
	// PreciseLinkStatusIntermediaryResourcesUpdated is used when
	// all intermediary resources have been successfully updated,
	// created or destroyed.
	PreciseLinkStatusIntermediaryResourcesUpdated
	// PreciseLinkStatusIntermediaryResourceUpdateFailed is used when
	// an intermediary resource has failed to be updated, created or destroyed.
	PreciseLinkStatusIntermediaryResourceUpdateFailed
	// PreciseLinkStatusComplete is used when
	// all components of the link have been successfully updated,
	// created or destroyed.
	PreciseLinkStatusComplete
)
