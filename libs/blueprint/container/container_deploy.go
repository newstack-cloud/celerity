package container

import (
	"context"
	"time"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

const (
	prepareFailureMessage = "failed to load instance state while preparing to deploy"
)

func (c *defaultBlueprintContainer) Deploy(
	ctx context.Context,
	input *DeployInput,
	channels *DeployChannels,
	paramOverrides core.BlueprintParams,
) {
	startTime := c.clock.Now()
	channels.DeploymentUpdateChan <- DeploymentUpdateMessage{
		InstanceID: input.InstanceID,
		Status:     core.InstanceStatusPreparing,
	}

	_, _, err := c.collectElementsToRemove(
		ctx,
		input.InstanceID,
		input.Changes,
		startTime,
		channels,
	)
	if err != nil {
		channels.ErrChan <- err
		return
	}

	// if finished {
	// 	return
	// }

	// Get all components that should be removed. (including recreated children)
	// Order component removal based on dependencies in state.
	// Group component removals so those that are not connected can be removed in parallel.
	// Remove components in order.
	// Get all components to be deployed or updated.
	// Order components based on dependencies in the blueprint.
	// Group components so those that are not connected can be deployed in parallel.
	// Unlike with change staging, groups are not executed as a unit, they are used as
	// pools to look for components that can be deployed based on the current state of deployment.
	// For each component to be created or updated (including recreated children):
	//  - call Deploy method component (resource, link, child blueprint)
	//      - handle specialised provider errors (retryable, resource deploy errors etc.)
	//  - If component is resource and is in config complete status, check if any of its dependents
	//    require the resource to be stable before they can be deployed.
	//    - If so, wait for the resource to be stable before deploying the dependent.
	//    - If not, begin deploying the dependent.
}

func (c *defaultBlueprintContainer) collectElementsToRemove(
	ctx context.Context,
	instanceID string,
	changes *BlueprintChanges,
	startTime time.Time,
	channels *DeployChannels,
) ([]*collectedElements, bool, error) {
	if len(changes.RemovedChildren) == 0 &&
		len(changes.RemovedResources) == 0 &&
		len(changes.RemovedLinks) == 0 {
		return []*collectedElements{}, false, nil
	}

	currentInstanceState, err := c.stateContainer.GetInstance(ctx, instanceID)
	if err != nil {
		channels.FinishChan <- c.createDeploymentFinishedMessage(
			instanceID,
			core.InstanceStatusDeployFailed,
			[]string{prepareFailureMessage},
			time.Since(startTime),
		)
		return []*collectedElements{}, true, nil
	}

	_, err = c.collectResourcesToRemove(&currentInstanceState, changes.RemovedResources)
	if err != nil {
		// todo: if error due to dependents still being in the blueprint, provide a more detailed failure reason message.
		channels.FinishChan <- c.createDeploymentFinishedMessage(
			instanceID,
			core.InstanceStatusDeployFailed,
			[]string{prepareFailureMessage},
			time.Since(startTime),
		)
		return []*collectedElements{}, true, nil
	}

	return []*collectedElements{}, false, nil
}

func (c *defaultBlueprintContainer) collectResourcesToRemove(
	currentState *state.InstanceState,
	removedResources []string,
) ([]*resourceIDInfo, error) {
	return []*resourceIDInfo{}, nil
}

func (c *defaultBlueprintContainer) createDeploymentFinishedMessage(
	instanceID string,
	status core.InstanceStatus,
	failureReasons []string,
	elapsedTime time.Duration,
) DeploymentFinishedMessage {
	elapsedMilliseconds := core.FractionalMilliseconds(elapsedTime)
	return DeploymentFinishedMessage{
		InstanceID:      instanceID,
		Status:          status,
		FailureReasons:  failureReasons,
		FinishTimestamp: c.clock.Now().Unix(),
		Durations: &state.InstanceCompletionDuration{
			TotalDuration: &elapsedMilliseconds,
		},
	}
}

type collectedElements struct {
	resources []*resourceIDInfo
	children  []*childBlueprintIDInfo
	total     int
}

// DeployChannels contains all the channels required to stream
// deployment events.
type DeployChannels struct {
	// ResourceUpdateChan receives messages about the status of deployment for resources.
	ResourceUpdateChan chan ResourceDeployUpdateMessage
	// LinkUpdateChan receives messages about the status of deployment for links.
	LinkUpdateChan chan LinkDeployUpdateMessage
	// ChildUpdateChan receives messages about the status of deployment for child blueprints.
	ChildUpdateChan chan ChildDeployUpdateMessage
	// DeploymentUpdateChan receives messages about the status of the blueprint instance deployment.
	DeploymentUpdateChan chan DeploymentUpdateMessage
	// FinishChan is used to signal that the blueprint instance deployment has finished,
	// the message will contain the final status of the deployment.
	FinishChan chan DeploymentFinishedMessage
	// ErrChan is used to signal that an unexpected error occurred during deployment of changes.
	ErrChan chan error
}

// ResourceDeployUpdateMessage provides a message containing status updates
// for resources being deployed.
// Deployment messages report on status changes for resources,
// the state of a resource will need to be fetched from the state container
// to get further information about the state of the resource.
type ResourceDeployUpdateMessage struct {
	// InstanceID is the ID of the blueprint instance
	// the message is associated with.
	// As updates are sent for parent and child blueprints,
	// this ID is used to differentiate between them.
	InstanceID string `json:"instanceId"`
	// ResourceID is the globally unique ID of the resource.
	ResourceID string `json:"resourceId"`
	// ResourceName is the logical name of the resource
	// as defined in the source blueprint.
	ResourceName string `json:"resourceName"`
	// Group is the group number the resource belongs to relative to the ordering
	// for components in the current blueprint associated with the instance ID.
	// A group is a collection of items that can be deployed at the same time.
	Group int `json:"group"`
	// Status holds the high-level status of the resource.
	Status core.ResourceStatus `json:"status"`
	// PreciseStatus holds the detailed status of the resource.
	PreciseStatus core.PreciseResourceStatus `json:"preciseStatus"`
	// FailureReasons holds a list of reasons why the resource failed to deploy
	// if the status update is for a failure.
	FailureReasons []string `json:"failureReasons,omitempty"`
	// Attempt is the current attempt number for deploying the resource.
	Attempt int `json:"attempt"`
	// UpdateTimestamp is the unix timestamp in seconds for
	// when the status update occurred.
	UpdateTimestamp int64 `json:"updateTimestamp"`
	// Durations holds duration information for a resource deployment.
	// Duration information is attached on one of the following precise status updates:
	// - PreciseResourceStatusConfigComplete
	// - PreciseResourceStatusCreated
	// - PreciseResourceStatusCreateFailed
	// - PreciseResourceStatusDestroyed
	// - PreciseResourceStatusDestroyFailed
	// - PreciseResourceStatusUpdateConfigComplete
	// - PreciseResourceStatusUpdated
	// - PreciseResourceStatusUpdateFailed
	Durations *state.ResourceCompletionDurations `json:"durations,omitempty"`
}

// ResourceChangesMessage provides a message containing status updates
// for resources being deployed.
// Deployment messages report on status changes for resources,
// the state of a resource will need to be fetched from the state container
// to get further information about the state of the resource.
type LinkDeployUpdateMessage struct {
	// InstanceID is the ID of the blueprint instance
	// the message is associated with.
	// As updates are sent for parent and child blueprints,
	// this ID is used to differentiate between them.
	InstanceID string `json:"instanceId"`
	// LinkID is the globally unique ID of the link.
	LinkID string `json:"linkId"`
	// LinkName is the logic name of the link in the blueprint.
	// This is a combination of the 2 resources that are linked.
	// For example, if a link is between a VPC and a subnet,
	// the link name would be "vpc::subnet".
	LinkName string `json:"linkName"`
	// Status holds the high-level status of the link.
	Status core.LinkStatus `json:"status"`
	// PreciseStatus holds the detailed status of the link.
	PreciseStatus core.PreciseLinkStatus `json:"preciseStatus"`
	// FailureReasons holds a list of reasons why the link failed to deploy
	// if the status update is for a failure.
	FailureReasons []string `json:"failureReasons,omitempty"`
	// Attempt is the current attempt number for deploying the link.
	Attempt int `json:"attempt"`
	// UpdateTimestamp is the unix timestamp in seconds for
	// when the status update occurred.
	UpdateTimestamp int64 `json:"updateTimestamp"`
	// Durations holds duration information for a link deployment.
	// Duration information is attached on one of the following precise status updates:
	// - PreciseLinkStatusResourceAUpdated
	// - PreciseLinkStatusResourceAUpdateFailed
	// - PreciseLinkStatusResourceBUpdated
	// - PreciseLinkStatusResourceBUpdateFailed
	// - PreciseLinkStatusIntermediaryResourcesUpdated
	// - PreciseLinkStatusIntermediaryResourceUpdateFailed
	// - PreciseLinkStatusComplete
	Durations *state.LinkCompletionDurations `json:"durations,omitempty"`
}

// ChildDeployUpdateMessage provides a message containing status updates
// for child blueprints being deployed.
// Deployment messages report on status changes for child blueprints,
// the state of a child blueprint will need to be fetched from the state container
// to get further information about the state of the child blueprint.
type ChildDeployUpdateMessage struct {
	// ParentInstanceID is the ID of the parent blueprint instance
	// the message is associated with.
	ParentInstanceID string `json:"parentInstanceId"`
	// ChildInstanceID is the ID of the child blueprint instance
	// the message is associated with.
	ChildInstanceID string `json:"instanceId"`
	// ChildName is the logical name of the child blueprint
	// as defined in the source blueprint as an include.
	ChildName string `json:"childName"`
	// Group is the group number the child blueprint belongs to relative to the ordering
	// for components in the current blueprint associated with the parent instance ID.
	Group int `json:"group"`
	// Status holds the status of the child blueprint.
	Status core.InstanceStatus `json:"status"`
	// FailureReasons holds a list of reasons why the child blueprint failed to deploy
	// if the status update is for a failure.
	FailureReasons []string `json:"failureReasons,omitempty"`
	// UpdateTimestamp is the unix timestamp in seconds for
	// when the status update occurred.
	UpdateTimestamp int64 `json:"updateTimestamp"`
	// Durations holds duration information for a child blueprint deployment.
	// Duration information is attached on one of the following status updates:
	// - InstanceStatusDeployed
	// - InstanceStatusDeployFailed
	// - InstanceStatusDestroyed
	// - InstanceStatusUpdated
	// - InstanceStatusUpdateFailed
	Durations *state.InstanceCompletionDuration `json:"durations,omitempty"`
}

// DeploymentUpdateMessage provides a message containing a blueprint-wide status update
// for the deployment of a blueprint instance.
type DeploymentUpdateMessage struct {
	// InstanceID is the ID of the blueprint instance
	// the message is associated with.
	InstanceID string `json:"instanceId"`
	// Status holds the status of the instance deployment.
	Status core.InstanceStatus `json:"status"`
}

// DeploymentFinishedMessage provides a message containing the final status
// of the blueprint instance deployment.
type DeploymentFinishedMessage struct {
	// InstanceID is the ID of the blueprint instance
	// the message is associated with.
	InstanceID string `json:"instanceId"`
	// Status holds the status of the instance deployment.
	Status core.InstanceStatus `json:"status"`
	// FailureReasons holds a list of reasons why the instance failed to deploy
	// if the final status is a failure.
	FailureReasons []string `json:"failureReasons,omitempty"`
	// FinishTimestamp is the unix timestamp in seconds for
	// when the deployment finished.
	FinishTimestamp int64 `json:"finishTimestamp"`
	// Durations holds duration information for the blueprint deployment.
	// Duration information is attached on one of the following status updates:
	// - InstanceStatusDeployed
	// - InstanceStatusDeployFailed
	// - InstanceStatusDestroyed
	// - InstanceStatusUpdated
	// - InstanceStatusUpdateFailed
	Durations *state.InstanceCompletionDuration `json:"durations,omitempty"`
}
