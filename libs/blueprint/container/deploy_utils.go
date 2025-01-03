package container

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

// CreateDeployChannels creates a new DeployChannels struct that contains
// all the channels required to process deploy/destroy events.
func CreateDeployChannels() *DeployChannels {
	resourceUpdateChan := make(chan ResourceDeployUpdateMessage)
	childUpdateChan := make(chan ChildDeployUpdateMessage)
	linkUpdateChan := make(chan LinkDeployUpdateMessage)
	deploymentUpdateChan := make(chan DeploymentUpdateMessage)
	finishChan := make(chan DeploymentFinishedMessage)
	errChan := make(chan error)

	return &DeployChannels{
		ResourceUpdateChan:   resourceUpdateChan,
		ChildUpdateChan:      childUpdateChan,
		LinkUpdateChan:       linkUpdateChan,
		DeploymentUpdateChan: deploymentUpdateChan,
		FinishChan:           finishChan,
		ErrChan:              errChan,
	}
}

func determineResourceDestroyingStatus(rollingBack bool) core.ResourceStatus {
	if rollingBack {
		return core.ResourceStatusRollingBack
	}

	return core.ResourceStatusDestroying
}

func determinePreciseResourceDestroyingStatus(rollingBack bool) core.PreciseResourceStatus {
	if rollingBack {
		// In the context of rolling back, destroying a resource is to roll back
		// the creation of the resource.
		return core.PreciseResourceStatusCreateRollingBack
	}

	return core.PreciseResourceStatusDestroying
}

func determineResourceDestroyFailedStatus(rollingBack bool) core.ResourceStatus {
	if rollingBack {
		return core.ResourceStatusRollbackFailed
	}

	return core.ResourceStatusDestroyFailed
}

func determinePreciseResourceDestroyFailedStatus(rollingBack bool) core.PreciseResourceStatus {
	if rollingBack {
		// In the context of rolling back, destroying a resource is to roll back
		// the creation of the resource.
		return core.PreciseResourceStatusCreateRollbackFailed
	}

	return core.PreciseResourceStatusDestroyFailed
}

func determineResourceDestroyedStatus(rollingBack bool) core.ResourceStatus {
	if rollingBack {
		return core.ResourceStatusRollbackComplete
	}

	return core.ResourceStatusDestroyed
}

func determinePreciseResourceDestroyedStatus(rollingBack bool) core.PreciseResourceStatus {
	if rollingBack {
		// In the context of rolling back, destroying a resource is to roll back
		// the creation of the resource.
		return core.PreciseResourceStatusCreateRollbackComplete
	}

	return core.PreciseResourceStatusDestroyed
}

func determineInstanceDestroyFailedStatus(rollingBack bool) core.InstanceStatus {
	if rollingBack {
		return core.InstanceStatusDeployRollbackFailed
	}

	return core.InstanceStatusDestroyFailed
}

func emptyChangesDeployFailedMessage(rollingBack bool) string {
	if rollingBack {
		return "an empty set of changes was provided for " +
			"re-deploying the blueprint when rolling back a destroy operation"
	}

	return "an empty set of changes was provided for deployment"
}

func emptyChangesDestroyFailedMessage(rollingBack bool) string {
	if rollingBack {
		return "an empty set of changes was provided for " +
			"destroying the blueprint when rolling back a deploy operation"
	}

	return "an empty set of changes was provided " +
		"for the blueprint instance to be destroyed"
}

func determineInstanceDestroyedStatus(rollingBack bool) core.InstanceStatus {
	if rollingBack {
		return core.InstanceStatusDeployRollbackComplete
	}

	return core.InstanceStatusDestroyed
}

func determineInstanceDeployFailedStatus(rollingBack bool, newInstance bool) core.InstanceStatus {
	if rollingBack && newInstance {
		return core.InstanceStatusDestroyRollbackFailed
	}

	if !newInstance {
		return core.InstanceStatusUpdateFailed
	}

	return core.InstanceStatusDeployFailed
}

func determineInstanceDestroyingStatus(rollingBack bool) core.InstanceStatus {
	if rollingBack {
		// If the context is destroying an instance as a part of the rollback
		// process, the status should be rolling back the original deployment.
		return core.InstanceStatusDeployRollingBack
	}

	return core.InstanceStatusDestroying
}

func determineInstanceDeployingStatus(rollingBack bool, newInstance bool) core.InstanceStatus {
	if rollingBack && newInstance {
		// If the context is deploying an instance as a part of the rollback
		// process, the status should be rolling back the destruction of the
		// instance.
		return core.InstanceStatusDestroyRollingBack
	}

	if rollingBack && !newInstance {
		// If the context is re-deploying an existing instance as a part of the rollback
		// process, the status should be rolling back the original deployment.
		return core.InstanceStatusUpdateRollingBack
	}

	if !newInstance {
		return core.InstanceStatusUpdating
	}

	return core.InstanceStatusDeploying
}

func determineInstanceDeployedStatus(rollingBack bool, newInstance bool) core.InstanceStatus {
	if rollingBack && newInstance {
		// If the context is deploying an instance as a part of the rollback
		// process, the status should be rolling back the destruction of the
		// instance.
		return core.InstanceStatusDestroyRollbackComplete
	}

	if rollingBack && !newInstance {
		// If the context is re-deploying an existing instance as a part of the rollback
		// process, the status should be rolling back the original deployment.
		return core.InstanceStatusUpdateRollbackComplete
	}

	if !newInstance {
		return core.InstanceStatusUpdated
	}

	return core.InstanceStatusDeployed
}

func determineResourceDeployingStatus(rollingBack bool, newResource bool) core.ResourceStatus {
	if rollingBack {
		// If the context is deploying a resource as a part of the rollback
		// process, the general status should be to roll back.
		return core.ResourceStatusRollingBack
	}

	if !newResource {
		return core.ResourceStatusUpdating
	}

	return core.ResourceStatusCreating
}

func determinePreciseResourceDeployingStatus(rollingBack bool, newResource bool) core.PreciseResourceStatus {
	if rollingBack && newResource {
		// If the context is deploying a new resource as a part of the rollback
		// process, the status should be rolling back the destruction of the
		// resource.
		return core.PreciseResourceStatusDestroyRollingBack
	}

	if rollingBack && !newResource {
		// If the context is re-deploying an existing resource as a part of the rollback
		// process, the status should be rolling back the original deployment.
		return core.PreciseResourceStatusUpdateRollingBack
	}

	if !newResource {
		return core.PreciseResourceStatusUpdating
	}

	return core.PreciseResourceStatusCreating
}

func determineResourceDeployFailedStatus(rollingBack bool, newResource bool) core.ResourceStatus {
	if rollingBack {
		// If the context is deploying a resource as a part of the rollback
		// process, the status should be a failure to rollback the original
		// resource deployment.
		return core.ResourceStatusRollbackFailed
	}

	if !newResource {
		return core.ResourceStatusUpdateFailed
	}

	return core.ResourceStatusCreateFailed
}

func determinePreciseResourceDeployFailedStatus(rollingBack bool, newResource bool) core.PreciseResourceStatus {
	if rollingBack && newResource {
		// If the context is deploying a new resource as a part of the rollback
		// process, the status should be a failure to rollback the original
		// resource deployment.
		return core.PreciseResourceStatusDestroyRollbackFailed
	}

	if rollingBack && !newResource {
		// If the context is re-deploying an existing resource as a part of the rollback
		// process, the status should be a failure to rollback the original resource
		// deployment.
		return core.PreciseResourceStatusUpdateRollbackFailed
	}

	if !newResource {
		return core.PreciseResourceStatusUpdateFailed
	}

	return core.PreciseResourceStatusCreateFailed
}

func determineResourceConfigCompleteStatus(rollingBack bool, newResource bool) core.ResourceStatus {
	if rollingBack {
		// If the context is deploying a resource as a part of the rollback
		// process, the status should be rolling back as config completion
		// is still in progress, waiting for the resource to be stabilised.
		return core.ResourceStatusRollingBack
	}

	if !newResource {
		return core.ResourceStatusUpdating
	}

	return core.ResourceStatusCreating
}

func determinePreciseResourceConfigCompleteStatus(rollingBack bool, newResource bool) core.PreciseResourceStatus {
	if rollingBack && newResource {
		// If the context is deploying a new resource as a part of the rollback
		// process, the status should be the config complete state for a destruction
		// rollback.
		return core.PreciseResourceStatusDestroyRollbackConfigComplete
	}

	if rollingBack && !newResource {
		// If the context is re-deploying an existing resource as a part of the rollback
		// process, the status should be the config complete state for an update rollback.
		return core.PreciseResourceStatusUpdateRollbackConfigComplete
	}

	return core.PreciseResourceStatusConfigComplete
}

func determineLinkUpdatingStatus(
	rollingBack bool,
	linkUpdateType provider.LinkUpdateType,
) core.LinkStatus {
	if rollingBack {
		return determineLinkUpdatingRollbackStatus(linkUpdateType)
	}

	if linkUpdateType == provider.LinkUpdateTypeCreate {
		return core.LinkStatusCreating
	}

	if linkUpdateType == provider.LinkUpdateTypeUpdate {
		return core.LinkStatusUpdating
	}

	return core.LinkStatusDestroying
}

func determineLinkOperationSuccessfullyFinishedStatus(
	rollingBack bool,
	linkUpdateType provider.LinkUpdateType,
) core.LinkStatus {
	if rollingBack {
		return determineLinkRollbackFinishedStatus(linkUpdateType)
	}

	if linkUpdateType == provider.LinkUpdateTypeCreate {
		return core.LinkStatusCreated
	}

	if linkUpdateType == provider.LinkUpdateTypeUpdate {
		return core.LinkStatusUpdated
	}

	return core.LinkStatusDestroyed
}

func determineLinkRollbackFinishedStatus(linkUpdateType provider.LinkUpdateType) core.LinkStatus {
	if linkUpdateType == provider.LinkUpdateTypeCreate {
		// A create update type in the context of rolling back
		// is to reverse the removal of a link.
		return core.LinkStatusDestroyRollbackComplete
	}

	if linkUpdateType == provider.LinkUpdateTypeUpdate {
		// An update type in the context of rolling back
		// is to reverse the changes made to a link.
		return core.LinkStatusUpdateRollbackComplete
	}

	// A destroy update type in the context of rolling back
	// is to reverse the creation of a link.
	return core.LinkStatusCreateRollbackComplete
}

func determineLinkUpdatingRollbackStatus(linkUpdateType provider.LinkUpdateType) core.LinkStatus {
	if linkUpdateType == provider.LinkUpdateTypeCreate {
		// A create update type in the context of rolling back
		// is to reverse the removal of a link.
		return core.LinkStatusDestroyRollingBack
	}

	if linkUpdateType == provider.LinkUpdateTypeUpdate {
		// An update type in the context of rolling back
		// is to reverse the changes made to a link.
		return core.LinkStatusUpdateRollingBack
	}

	// A destroy update type in the context of rolling back
	// is to reverse the creation of a link.
	return core.LinkStatusCreateRollingBack
}

func determineLinkUpdateFailedStatus(
	rollingBack bool,
	linkUpdateType provider.LinkUpdateType,
) core.LinkStatus {
	if rollingBack {
		return determineLinkUpdateRollbackFailedStatus(linkUpdateType)
	}

	if linkUpdateType == provider.LinkUpdateTypeCreate {
		return core.LinkStatusCreateFailed
	}

	if linkUpdateType == provider.LinkUpdateTypeUpdate {
		return core.LinkStatusUpdateFailed
	}

	return core.LinkStatusDestroyFailed
}

func determineLinkUpdateRollbackFailedStatus(
	linkUpdateType provider.LinkUpdateType,
) core.LinkStatus {
	if linkUpdateType == provider.LinkUpdateTypeCreate {
		// A create update type in the context of rolling back
		// is to reverse the removal of a link.
		return core.LinkStatusDestroyRollbackFailed
	}

	if linkUpdateType == provider.LinkUpdateTypeUpdate {
		// An update type in the context of rolling back
		// is to reverse the changes made to a link.
		return core.LinkStatusUpdateRollbackFailed
	}

	// A destroy update type in the context of rolling back
	// is to reverse the creation of a link.
	return core.LinkStatusCreateRollbackFailed
}

func determinePreciseLinkUpdatingResourceAStatus(rollingBack bool) core.PreciseLinkStatus {
	if rollingBack {
		// Updating resource A in the context of rolling back is to roll back
		// the latest changes made to resource A specific to the current link.
		return core.PreciseLinkStatusResourceAUpdateRollingBack
	}

	return core.PreciseLinkStatusUpdatingResourceA
}

func determinePreciseLinkUpdatingResourceBStatus(rollingBack bool) core.PreciseLinkStatus {
	if rollingBack {
		// Updating resource B in the context of rolling back is to roll back
		// the latest changes made to resource B specific to the current link.
		return core.PreciseLinkStatusResourceBUpdateRollingBack
	}

	return core.PreciseLinkStatusUpdatingResourceB
}

func determinePreciseLinkResourceAUpdateFailedStatus(rollingBack bool) core.PreciseLinkStatus {
	if rollingBack {
		// Updating resource A in the context of rolling back is to roll back
		// the latest changes made to resource A specific to the current link.
		return core.PreciseLinkStatusResourceAUpdateRollbackFailed
	}

	return core.PreciseLinkStatusResourceAUpdateFailed
}

func determinePreciseLinkResourceAUpdatedStatus(rollingBack bool) core.PreciseLinkStatus {
	if rollingBack {
		// Updating resource A in the context of rolling back is to roll back
		// the latest changes made to resource A specific to the current link.
		return core.PreciseLinkStatusResourceAUpdateRollbackComplete
	}

	return core.PreciseLinkStatusResourceAUpdated
}

func determinePreciseLinkResourceBUpdatedStatus(rollingBack bool) core.PreciseLinkStatus {
	if rollingBack {
		// Updating resource B in the context of rolling back is to roll back
		// the latest changes made to resource B specific to the current link.
		return core.PreciseLinkStatusResourceBUpdateRollbackComplete
	}

	return core.PreciseLinkStatusResourceBUpdated
}

func determinePreciseLinkResourceBUpdateFailedStatus(rollingBack bool) core.PreciseLinkStatus {
	if rollingBack {
		// Updating resource B in the context of rolling back is to roll back
		// the latest changes made to resource B specific to the current link.
		return core.PreciseLinkStatusResourceBUpdateRollbackFailed
	}

	return core.PreciseLinkStatusResourceBUpdateFailed
}

func determinePreciseLinkUpdatingIntermediariesStatus(rollingBack bool) core.PreciseLinkStatus {
	if rollingBack {
		// Updating intermediary resources in the context of rolling back is to roll back
		// the latest changes made to intermediary resources specific to the current link.
		return core.PreciseLinkStatusIntermediaryResourceUpdateRollingBack
	}

	return core.PreciseLinkStatusUpdatingIntermediaryResources
}

func determinePreciseLinkIntermediariesUpdateFailedStatus(rollingBack bool) core.PreciseLinkStatus {
	if rollingBack {
		// Updating intermediary resources in the context of rolling back is to roll back
		// the latest changes made to intermediary resources specific to the current link.
		return core.PreciseLinkStatusIntermediaryResourceUpdateRollbackFailed
	}

	return core.PreciseLinkStatusIntermediaryResourceUpdateFailed
}

func determinePreciseLinkIntermediariesUpdatedStatus(rollingBack bool) core.PreciseLinkStatus {
	if rollingBack {
		// Updating intermediary resources in the context of rolling back is to roll back
		// the latest changes made to intermediary resources specific to the current link.
		return core.PreciseLinkStatusIntermediaryResourceUpdateRollbackComplete
	}

	return core.PreciseLinkStatusIntermediaryResourcesUpdated
}

func determineLinkUpdateIntermediariesRetryFailureDurations(
	currentRetryInfo *retryInfo,
) *state.LinkCompletionDurations {
	if currentRetryInfo.exceededMaxRetries {
		totalDuration := core.Sum(currentRetryInfo.attemptDurations)
		return &state.LinkCompletionDurations{
			IntermediaryResources: &state.LinkComponentCompletionDurations{
				TotalDuration:    &totalDuration,
				AttemptDurations: currentRetryInfo.attemptDurations,
			},
		}
	}

	return &state.LinkCompletionDurations{
		IntermediaryResources: &state.LinkComponentCompletionDurations{
			AttemptDurations: currentRetryInfo.attemptDurations,
		},
	}
}

func determineFinishedFailureStatus(destroyingInstance bool, rollingBack bool) core.InstanceStatus {
	if destroyingInstance {
		if rollingBack {
			// If the context is destroying an instance as a part of the rollback
			// process, the status should be a failure to rollback the original
			// deployment.
			return core.InstanceStatusDeployRollbackFailed
		}

		return core.InstanceStatusDestroyFailed
	}

	if rollingBack {
		// If the context is deploying an instance as a part of the rollback
		// process, the status should be a failure to rollback destroying the
		// instance.
		return core.InstanceStatusDestroyRollbackFailed
	}

	return core.InstanceStatusDeployFailed
}

func finishedFailureMessages(deployCtx *deployContext, failedElements []string) []string {
	operation := determineOperation(deployCtx)
	messages := []string{
		fmt.Sprintf(
			"failed to %s the blueprint instance due to %d %s",
			operation,
			len(failedElements),
			pluralise("failure", "failures", len(failedElements)),
		),
	}
	messageTemplate := "failed to %s %q"
	for _, elementName := range failedElements {
		messages = append(messages, fmt.Sprintf(messageTemplate, operation, elementName))
	}

	return messages
}

func determineOperation(deployCtx *deployContext) string {
	if deployCtx.destroying && !deployCtx.rollback {
		return "destroy"
	}

	if deployCtx.destroying && deployCtx.rollback {
		return "roll back deployment of"
	}

	if !deployCtx.destroying && deployCtx.rollback {
		return "roll back destruction of"
	}

	return "deploy"
}

func checkDeploymentForNewInstance(input *DeployInput) (bool, error) {
	if input.Changes == nil {
		return input.InstanceID == "", nil
	}

	hasExistingResourceChanges := len(input.Changes.ResourceChanges) > 0 ||
		len(input.Changes.RemovedResources) > 0

	hasExistingChildChanges := len(input.Changes.ChildChanges) > 0 ||
		len(input.Changes.RemovedChildren) > 0 ||
		len(input.Changes.RecreateChildren) > 0

	if input.InstanceID == "" && (hasExistingResourceChanges || hasExistingChildChanges) {
		return false, errInstanceIDRequiredForChanges()
	}

	isForNewInstance := input.InstanceID == "" &&
		!hasExistingResourceChanges &&
		!hasExistingChildChanges

	return isForNewInstance, nil
}

func isResourceDestroyEvent(preciseStatus core.PreciseResourceStatus, rollingBack bool) bool {
	if rollingBack {
		// In the context of rolling back, destroying a resource is to roll back
		// the creation of the resource.
		return preciseStatus == core.PreciseResourceStatusCreateRollingBack ||
			preciseStatus == core.PreciseResourceStatusCreateRollbackComplete ||
			preciseStatus == core.PreciseResourceStatusCreateRollbackFailed
	}

	return preciseStatus == core.PreciseResourceStatusDestroying ||
		preciseStatus == core.PreciseResourceStatusDestroyed ||
		preciseStatus == core.PreciseResourceStatusDestroyFailed
}

func isChildDestroyEvent(instanceStatus core.InstanceStatus, rollingBack bool) bool {
	if rollingBack {
		// In the context of rolling back, destroying a child instance is to roll back
		// the deployment of the child instance.
		return instanceStatus == core.InstanceStatusDeployRollingBack ||
			instanceStatus == core.InstanceStatusDeployRollbackComplete ||
			instanceStatus == core.InstanceStatusDeployRollbackFailed
	}

	return instanceStatus == core.InstanceStatusDestroying ||
		instanceStatus == core.InstanceStatusDestroyed ||
		instanceStatus == core.InstanceStatusDestroyFailed
}

func isLinkDestroyEvent(linkStatus core.LinkStatus, rollingBack bool) bool {
	if rollingBack {
		// In the context of rolling back, destroying a link is to roll back
		// the creation of the link.
		return linkStatus == core.LinkStatusCreateRollingBack ||
			linkStatus == core.LinkStatusCreateRollbackComplete ||
			linkStatus == core.LinkStatusCreateRollbackFailed
	}

	return linkStatus == core.LinkStatusDestroying ||
		linkStatus == core.LinkStatusDestroyed ||
		linkStatus == core.LinkStatusDestroyFailed
}

func isResourceUpdateEvent(preciseStatus core.PreciseResourceStatus, rollingBack bool) bool {
	if rollingBack {
		return preciseStatus == core.PreciseResourceStatusUpdateRollingBack ||
			preciseStatus == core.PreciseResourceStatusUpdateRollbackComplete ||
			preciseStatus == core.PreciseResourceStatusUpdateRollbackFailed ||
			preciseStatus == core.PreciseResourceStatusUpdateRollbackConfigComplete
	}

	return preciseStatus == core.PreciseResourceStatusUpdating ||
		preciseStatus == core.PreciseResourceStatusUpdated ||
		preciseStatus == core.PreciseResourceStatusUpdateFailed ||
		preciseStatus == core.PreciseResourceStatusUpdateConfigComplete
}

func isResourceCreationEvent(preciseStatus core.PreciseResourceStatus, rollingBack bool) bool {
	if rollingBack {
		// In the context of rolling back, creating a resource is to roll back
		// the destruction of the resource.
		return preciseStatus == core.PreciseResourceStatusDestroyRollingBack ||
			preciseStatus == core.PreciseResourceStatusDestroyRollbackComplete ||
			preciseStatus == core.PreciseResourceStatusDestroyRollbackFailed ||
			preciseStatus == core.PreciseResourceStatusDestroyRollbackConfigComplete
	}

	return preciseStatus == core.PreciseResourceStatusCreating ||
		preciseStatus == core.PreciseResourceStatusCreated ||
		preciseStatus == core.PreciseResourceStatusCreateFailed ||
		preciseStatus == core.PreciseResourceStatusConfigComplete
}

func isChildUpdateEvent(status core.InstanceStatus, rollingBack bool) bool {
	if rollingBack {
		return status == core.InstanceStatusUpdateRollingBack ||
			status == core.InstanceStatusUpdateRollbackComplete ||
			status == core.InstanceStatusUpdateRollbackFailed
	}

	return status == core.InstanceStatusUpdating ||
		status == core.InstanceStatusUpdated ||
		status == core.InstanceStatusUpdateFailed
}

func isChildDeployEvent(status core.InstanceStatus, rollingBack bool) bool {
	if rollingBack {
		// In the context of rolling back, deploying a child instance is to roll back
		// the destruction of the child instance.
		return status == core.InstanceStatusDestroyRollingBack ||
			status == core.InstanceStatusDestroyRollbackComplete ||
			status == core.InstanceStatusDestroyRollbackFailed
	}

	return status == core.InstanceStatusDeploying ||
		status == core.InstanceStatusDeployed ||
		status == core.InstanceStatusDeployFailed
}

func isLinkUpdateEvent(status core.LinkStatus, rollingBack bool) bool {
	if rollingBack {
		return status == core.LinkStatusUpdateRollingBack ||
			status == core.LinkStatusUpdateRollbackComplete ||
			status == core.LinkStatusUpdateRollbackFailed
	}

	return status == core.LinkStatusUpdating ||
		status == core.LinkStatusUpdated ||
		status == core.LinkStatusUpdateFailed
}

func isLinkCreationEvent(status core.LinkStatus, rollingBack bool) bool {
	if rollingBack {
		// In the context of rolling back, creating a link is to roll back
		// the destruction of the link.
		return status == core.LinkStatusDestroyRollingBack ||
			status == core.LinkStatusDestroyRollbackComplete ||
			status == core.LinkStatusDestroyRollbackFailed
	}

	return status == core.LinkStatusCreating ||
		status == core.LinkStatusCreated ||
		status == core.LinkStatusCreateFailed
}

func startedDestroyingResource(
	preciseStatus core.PreciseResourceStatus,
	rollingBack bool,
) bool {
	if rollingBack {
		// In the context of rolling back, destroying a resource is to roll back
		// the creation of the resource.
		return preciseStatus == core.PreciseResourceStatusCreateRollingBack
	}

	return preciseStatus == core.PreciseResourceStatusDestroying
}

func startedDestroyingChild(
	status core.InstanceStatus,
	rollingBack bool,
) bool {
	if rollingBack {
		// In the context of rolling back, destroying a child instance is to roll back
		// the deployment of the child instance.
		return status == core.InstanceStatusDeployRollingBack
	}

	return status == core.InstanceStatusDestroying
}

func startedDestroyingLink(
	status core.LinkStatus,
	rollingBack bool,
) bool {
	if rollingBack {
		// In the context of rolling back, destroying a link is to roll back
		// the creation of the link.
		return status == core.LinkStatusCreateRollingBack
	}

	return status == core.LinkStatusDestroying
}

func finishedDestroyingResource(
	msg ResourceDeployUpdateMessage,
	rollingBack bool,
) bool {

	if rollingBack {
		// In the context of rolling back, destroying a resource is to roll back
		// the creation of the resource.
		rollbackFinished := msg.PreciseStatus == core.PreciseResourceStatusCreateRollbackComplete ||
			msg.PreciseStatus == core.PreciseResourceStatusCreateRollbackFailed
		return rollbackFinished && !msg.CanRetry
	}

	destroyFinished := msg.PreciseStatus == core.PreciseResourceStatusDestroyed ||
		msg.PreciseStatus == core.PreciseResourceStatusDestroyFailed

	return destroyFinished && !msg.CanRetry
}

func finishedDestroyingChild(
	msg ChildDeployUpdateMessage,
	rollingBack bool,
) bool {

	if rollingBack {
		// In the context of rolling back, destroying a child is to roll back
		// the deployment of the child instance.
		return msg.Status == core.InstanceStatusDeployRollbackComplete ||
			msg.Status == core.InstanceStatusDeployRollbackFailed
	}

	return msg.Status == core.InstanceStatusDestroyed ||
		msg.Status == core.InstanceStatusDestroyFailed
}

func finishedDestroyingLink(
	msg LinkDeployUpdateMessage,
	rollingBack bool,
) bool {

	if rollingBack {
		// In the context of rolling back, destroying a link is to roll back
		// the creation of the link.
		rollbackFinished := msg.Status == core.LinkStatusCreateRollbackComplete ||
			msg.Status == core.LinkStatusCreateRollbackFailed
		return rollbackFinished && !msg.CanRetryCurrentStage
	}

	destroyFinshed := msg.Status == core.LinkStatusDestroyed ||
		msg.Status == core.LinkStatusDestroyFailed

	return destroyFinshed && !msg.CanRetryCurrentStage
}

func wasResourceDestroyedSuccessfully(
	preciseStatus core.PreciseResourceStatus,
	rollingBack bool,
) bool {
	if rollingBack {
		// In the context of rolling back, destroying a resource is to roll back
		// the creation of the resource.
		return preciseStatus == core.PreciseResourceStatusCreateRollbackComplete
	}

	return preciseStatus == core.PreciseResourceStatusDestroyed
}

func wasChildDestroyedSuccessfully(
	status core.InstanceStatus,
	rollingBack bool,
) bool {
	if rollingBack {
		// In the context of rolling back, destroying a child is to roll back
		// the deployment of the child instance.
		return status == core.InstanceStatusDeployRollbackComplete
	}

	return status == core.InstanceStatusDestroyed
}

func wasLinkDestroyedSuccessfully(
	status core.LinkStatus,
	rollingBack bool,
) bool {
	if rollingBack {
		// In the context of rolling back, destroying a link is to roll back
		// the creation of the link.
		return status == core.LinkStatusCreateRollbackComplete
	}

	return status == core.LinkStatusDestroyed
}

func determineResourceRetryFailureDurations(
	currentRetryInfo *retryInfo,
) *state.ResourceCompletionDurations {
	if currentRetryInfo.exceededMaxRetries {
		totalDuration := core.Sum(currentRetryInfo.attemptDurations)
		return &state.ResourceCompletionDurations{
			TotalDuration:    &totalDuration,
			AttemptDurations: currentRetryInfo.attemptDurations,
		}
	}

	return &state.ResourceCompletionDurations{
		AttemptDurations: currentRetryInfo.attemptDurations,
	}
}

func determineResourceDeployConfigCompleteDurations(
	currentRetryInfo *retryInfo,
	currentConfigCompleteDuration time.Duration,
) *state.ResourceCompletionDurations {
	configCompleteMS := core.FractionalMilliseconds(currentConfigCompleteDuration)
	configCompleteDurationPtr := &configCompleteMS
	return &state.ResourceCompletionDurations{
		ConfigCompleteDuration: configCompleteDurationPtr,
		AttemptDurations:       currentRetryInfo.attemptDurations,
	}
}

func determineResourceDeployFinishedDurations(
	currentRetryInfo *retryInfo,
	currentAttemptDuration time.Duration,
	configCompleteDuration *time.Duration,
) *state.ResourceCompletionDurations {
	updatedAttemptDurations := append(
		currentRetryInfo.attemptDurations,
		core.FractionalMilliseconds(currentAttemptDuration),
	)

	configCompleteDurationPtr := (*float64)(nil)
	if configCompleteDuration != nil {
		configCompleteMS := core.FractionalMilliseconds(*configCompleteDuration)
		configCompleteDurationPtr = &configCompleteMS
	}

	totalDuration := core.Sum(updatedAttemptDurations)
	return &state.ResourceCompletionDurations{
		ConfigCompleteDuration: configCompleteDurationPtr,
		TotalDuration:          &totalDuration,
		AttemptDurations:       updatedAttemptDurations,
	}
}

func determineLinkUpdateResourceARetryFailureDurations(
	currentRetryInfo *retryInfo,
) *state.LinkCompletionDurations {
	if currentRetryInfo.exceededMaxRetries {
		totalDuration := core.Sum(currentRetryInfo.attemptDurations)
		return &state.LinkCompletionDurations{
			ResourceAUpdate: &state.LinkComponentCompletionDurations{
				TotalDuration:    &totalDuration,
				AttemptDurations: currentRetryInfo.attemptDurations,
			},
		}
	}

	return &state.LinkCompletionDurations{
		ResourceAUpdate: &state.LinkComponentCompletionDurations{
			AttemptDurations: currentRetryInfo.attemptDurations,
		},
	}
}

func determineLinkUpdateResourceAFinishedDurations(
	currentRetryInfo *retryInfo,
	currentAttemptDuration time.Duration,
) *state.LinkCompletionDurations {
	updatedAttemptDurations := append(
		currentRetryInfo.attemptDurations,
		core.FractionalMilliseconds(currentAttemptDuration),
	)
	totalDuration := core.Sum(updatedAttemptDurations)
	return &state.LinkCompletionDurations{
		ResourceAUpdate: &state.LinkComponentCompletionDurations{
			TotalDuration:    &totalDuration,
			AttemptDurations: updatedAttemptDurations,
		},
	}
}

func determineLinkUpdateResourceBRetryFailureDurations(
	currentRetryInfo *retryInfo,
) *state.LinkCompletionDurations {
	if currentRetryInfo.exceededMaxRetries {
		totalDuration := core.Sum(currentRetryInfo.attemptDurations)
		return &state.LinkCompletionDurations{
			ResourceBUpdate: &state.LinkComponentCompletionDurations{
				TotalDuration:    &totalDuration,
				AttemptDurations: currentRetryInfo.attemptDurations,
			},
		}
	}

	return &state.LinkCompletionDurations{
		ResourceBUpdate: &state.LinkComponentCompletionDurations{
			AttemptDurations: currentRetryInfo.attemptDurations,
		},
	}
}

func determineLinkUpdateResourceBFinishedDurations(
	currentRetryInfo *retryInfo,
	currentAttemptDuration time.Duration,
	accumDurationInfo *state.LinkCompletionDurations,
) *state.LinkCompletionDurations {
	updatedAttemptDurations := append(
		currentRetryInfo.attemptDurations,
		core.FractionalMilliseconds(currentAttemptDuration),
	)
	totalDuration := core.Sum(updatedAttemptDurations)
	durationInfo := accumDurationInfo
	if durationInfo == nil {
		durationInfo = &state.LinkCompletionDurations{}
	}
	durationInfo.ResourceBUpdate = &state.LinkComponentCompletionDurations{
		TotalDuration:    &totalDuration,
		AttemptDurations: updatedAttemptDurations,
	}
	return durationInfo
}

func determineLinkUpdateIntermediariesFinishedDurations(
	currentRetryInfo *retryInfo,
	currentAttemptDuration time.Duration,
	accumDurationInfo *state.LinkCompletionDurations,
) *state.LinkCompletionDurations {
	updatedAttemptDurations := append(
		currentRetryInfo.attemptDurations,
		core.FractionalMilliseconds(currentAttemptDuration),
	)
	stageTotalDuration := core.Sum(updatedAttemptDurations)
	durationInfo := accumDurationInfo
	if durationInfo == nil {
		durationInfo = &state.LinkCompletionDurations{}
	}
	durationInfo.IntermediaryResources = &state.LinkComponentCompletionDurations{
		TotalDuration:    &stageTotalDuration,
		AttemptDurations: updatedAttemptDurations,
	}
	totalDuration := sumLinkComponentCompletionDurations(durationInfo)
	durationInfo.TotalDuration = &totalDuration
	return durationInfo
}

func sumLinkComponentCompletionDurations(
	durations *state.LinkCompletionDurations,
) float64 {
	return *durations.ResourceAUpdate.TotalDuration +
		*durations.ResourceBUpdate.TotalDuration +
		*durations.IntermediaryResources.TotalDuration
}

func addRetryAttempt(retryInfoToUpdate *retryInfo, currentAttemptDuration time.Duration) *retryInfo {
	nextAttempt := retryInfoToUpdate.attempt + 1
	return &retryInfo{
		policy:  retryInfoToUpdate.policy,
		attempt: nextAttempt,
		attemptDurations: append(
			retryInfoToUpdate.attemptDurations,
			core.FractionalMilliseconds(currentAttemptDuration),
		),
		exceededMaxRetries: nextAttempt > retryInfoToUpdate.policy.MaxRetries,
	}
}

func createDestroyChangesFromChildState(
	childState *state.InstanceState,
) *BlueprintChanges {
	changes := &BlueprintChanges{
		RemovedResources: []string{},
		RemovedLinks:     []string{},
		RemovedChildren:  []string{},
		RemovedExports:   []string{},
	}

	for _, resource := range childState.Resources {
		changes.RemovedResources = append(changes.RemovedResources, resource.ResourceName)
	}

	for _, link := range childState.Links {
		changes.RemovedLinks = append(changes.RemovedLinks, link.LinkName)
	}

	for childName := range childState.ChildBlueprints {
		changes.RemovedChildren = append(changes.RemovedChildren, childName)
	}

	for exportName := range childState.Exports {
		changes.RemovedExports = append(changes.RemovedExports, exportName)
	}

	return changes
}

func getFailedRemovalsAndUpdateState(
	finished map[string]*deployUpdateMessageWrapper,
	group []state.Element,
	state DeploymentState,
	rollback bool,
) []string {
	failed := []string{}

	for _, element := range group {
		elementName := getNamespacedLogicalName(element)
		if msgWrapper, ok := finished[elementName]; ok {
			if removalWasSuccessful(msgWrapper, rollback) {
				state.SetDestroyedElement(element)
			} else {
				failed = append(failed, elementName)
			}
		} else {
			failed = append(failed, elementName)
		}
	}

	return failed
}

func getFailedElementDeploymentsAndUpdateState(
	finished map[string]*deployUpdateMessageWrapper,
	changes *BlueprintChanges,
	deployCtx *deployContext,
) []string {
	failed := []string{}

	failedResources := getFailedResourceDeploymentsAndUpdateState(
		finished,
		changes,
		deployCtx,
	)
	failed = append(failed, failedResources...)

	failedLinks := getFailedLinkDeploymentsAndUpdateState(
		finished,
		changes,
		deployCtx,
	)
	failed = append(failed, failedLinks...)

	failedChildren := getFailedChildDeploymentsAndUpdateState(
		finished,
		changes,
		deployCtx,
	)
	failed = append(failed, failedChildren...)

	return failed
}

// A lock must be held on the deployment state before calling this function.
func getFailedResourceDeploymentsAndUpdateState(
	finished map[string]*deployUpdateMessageWrapper,
	changes *BlueprintChanges,
	deployCtx *deployContext,
) []string {
	failed := []string{}

	for updateResourceName := range changes.ResourceChanges {
		resourceElementName := core.ResourceElementID(updateResourceName)
		resourceUpdateFailed := checkResourceUpdateFailedAndUpdateState(
			resourceElementName,
			finished,
			deployCtx,
			updateWasSuccessful,
		)
		if resourceUpdateFailed {
			failed = append(failed, resourceElementName)
		}
	}

	for createdResourceName := range changes.NewResources {
		resourceElementName := core.ResourceElementID(createdResourceName)
		resourceCreationFailed := checkResourceUpdateFailedAndUpdateState(
			resourceElementName,
			finished,
			deployCtx,
			creationWasSuccessful,
		)
		if resourceCreationFailed {
			failed = append(failed, resourceElementName)
		}
	}

	return failed
}

func checkResourceUpdateFailedAndUpdateState(
	resourceElementName string,
	finished map[string]*deployUpdateMessageWrapper,
	deployCtx *deployContext,
	wasSuccessful func(*deployUpdateMessageWrapper, bool) bool,
) bool {
	if msgWrapper, ok := finished[resourceElementName]; ok {
		if wasSuccessful(msgWrapper, deployCtx.rollback) {
			deployCtx.state.SetUpdatedElement(&ResourceIDInfo{
				ResourceID:   msgWrapper.resourceUpdateMessage.ResourceID,
				ResourceName: msgWrapper.resourceUpdateMessage.ResourceName,
			})
			return false
		}
	}

	return true
}

// A lock must be held on the deployment state before calling this function.
func getFailedLinkDeploymentsAndUpdateState(
	finished map[string]*deployUpdateMessageWrapper,
	changes *BlueprintChanges,
	deployCtx *deployContext,
) []string {
	failed := []string{}

	for _, updateResourceChanges := range changes.ResourceChanges {
		linkCreationFailed := checkLinkDeploymentsFailedAndUpdateState(
			updateResourceChanges.NewOutboundLinks,
			finished,
			deployCtx,
			creationWasSuccessful,
		)
		failed = append(failed, linkCreationFailed...)

		linkUpdateFailed := checkLinkDeploymentsFailedAndUpdateState(
			updateResourceChanges.OutboundLinkChanges,
			finished,
			deployCtx,
			updateWasSuccessful,
		)
		failed = append(failed, linkUpdateFailed...)
	}

	return failed
}

func checkLinkDeploymentsFailedAndUpdateState(
	linkChanges map[string]provider.LinkChanges,
	finished map[string]*deployUpdateMessageWrapper,
	deployCtx *deployContext,
	wasSuccessful func(*deployUpdateMessageWrapper, bool) bool,
) []string {
	failedLinks := []string{}

	for linkName := range linkChanges {
		linkElementName := linkElementID(linkName)
		if msgWrapper, ok := finished[linkElementName]; ok {
			if wasSuccessful(msgWrapper, deployCtx.rollback) {
				deployCtx.state.SetCreatedElement(&LinkIDInfo{
					LinkID:   msgWrapper.linkUpdateMessage.LinkID,
					LinkName: linkName,
				})
			} else {
				failedLinks = append(failedLinks, linkElementName)
			}
		} else {
			failedLinks = append(failedLinks, linkElementName)
		}
	}

	return failedLinks
}

func getFailedChildDeploymentsAndUpdateState(
	finished map[string]*deployUpdateMessageWrapper,
	changes *BlueprintChanges,
	deployCtx *deployContext,
) []string {
	failed := []string{}

	// for childName := range changes.ChildChanges {
	// 	childDeployFailed := checkChildDeploymentFailedAndUpdateState(
	// 		childName,
	// 		finished,
	// 		deployCtx,
	// 		updateWasSuccessful,
	// 	)
	// 	if childDeployFailed {
	// 		childElementName := core.ChildElementID(childName)
	// 		failed = append(failed, childElementName)
	// 	}
	// }

	// for childName := range changes.NewChildren {
	// 	childDeployFailed := checkChildDeploymentFailedAndUpdateState(
	// 		childName,
	// 		finished,
	// 		deployCtx,
	// 		creationWasSuccessful,
	// 	)
	// 	if childDeployFailed {
	// 		childElementName := core.ChildElementID(childName)
	// 		failed = append(failed, childElementName)
	// 	}
	// }

	// for _, childName := range changes.RecreateChildren {
	// 	childDeployFailed := checkChildDeploymentFailedAndUpdateState(
	// 		childName,
	// 		finished,
	// 		deployCtx,
	// 		creationWasSuccessful,
	// 	)
	// 	if childDeployFailed {
	// 		childElementName := core.ChildElementID(childName)
	// 		failed = append(failed, childElementName)
	// 	}
	// }

	return failed
}

func removalWasSuccessful(
	msgWrapper *deployUpdateMessageWrapper,
	rollback bool,
) bool {
	if msgWrapper == nil {
		return false
	}

	if msgWrapper.resourceUpdateMessage != nil {
		if rollback {
			return msgWrapper.resourceUpdateMessage.PreciseStatus == core.PreciseResourceStatusCreateRollbackComplete
		}

		return msgWrapper.resourceUpdateMessage.Status == core.ResourceStatusDestroyed
	}

	if msgWrapper.childUpdateMessage != nil {
		if rollback {
			return msgWrapper.childUpdateMessage.Status == core.InstanceStatusDeployRollbackComplete
		}

		return msgWrapper.childUpdateMessage.Status == core.InstanceStatusDestroyed
	}

	if msgWrapper.linkUpdateMessage != nil {
		if rollback {
			return msgWrapper.linkUpdateMessage.Status == core.LinkStatusCreateRollbackComplete
		}

		return msgWrapper.linkUpdateMessage.Status == core.LinkStatusDestroyed
	}

	return false
}

func updateWasSuccessful(
	msgWrapper *deployUpdateMessageWrapper,
	rollback bool,
) bool {
	if msgWrapper == nil {
		return false
	}

	if msgWrapper.resourceUpdateMessage != nil {
		if rollback {
			return msgWrapper.resourceUpdateMessage.PreciseStatus == core.PreciseResourceStatusUpdateRollbackComplete
		}

		return msgWrapper.resourceUpdateMessage.Status == core.ResourceStatusUpdated
	}

	if msgWrapper.childUpdateMessage != nil {
		if rollback {
			return msgWrapper.childUpdateMessage.Status == core.InstanceStatusUpdateRollbackComplete
		}

		return msgWrapper.childUpdateMessage.Status == core.InstanceStatusUpdated
	}

	if msgWrapper.linkUpdateMessage != nil {
		if rollback {
			return msgWrapper.linkUpdateMessage.Status == core.LinkStatusUpdateRollbackComplete
		}

		return msgWrapper.linkUpdateMessage.Status == core.LinkStatusUpdated
	}

	return false
}

func creationWasSuccessful(
	msgWrapper *deployUpdateMessageWrapper,
	rollback bool,
) bool {
	if msgWrapper == nil {
		return false
	}

	if msgWrapper.resourceUpdateMessage != nil {
		if rollback {
			return msgWrapper.resourceUpdateMessage.PreciseStatus == core.PreciseResourceStatusDestroyRollbackComplete
		}

		return msgWrapper.resourceUpdateMessage.Status == core.ResourceStatusCreated
	}

	if msgWrapper.childUpdateMessage != nil {
		if rollback {
			return msgWrapper.childUpdateMessage.Status == core.InstanceStatusDestroyRollbackComplete
		}

		return msgWrapper.childUpdateMessage.Status == core.InstanceStatusDeployed
	}

	if msgWrapper.linkUpdateMessage != nil {
		if rollback {
			return msgWrapper.linkUpdateMessage.Status == core.LinkStatusDestroyRollbackComplete
		}

		return msgWrapper.linkUpdateMessage.Status == core.LinkStatusCreated
	}

	return false
}

func updateToChildUpdateMessage(
	msg *DeploymentUpdateMessage,
	parentInstanceID string,
	element state.Element,
	groupIndex int,
) ChildDeployUpdateMessage {
	return ChildDeployUpdateMessage{
		ParentInstanceID: parentInstanceID,
		ChildInstanceID:  element.ID(),
		ChildName:        element.LogicalName(),
		Group:            groupIndex,
		Status:           msg.Status,
		UpdateTimestamp:  msg.UpdateTimestamp,
	}
}

func finishedToChildUpdateMessage(
	msg *DeploymentFinishedMessage,
	parentInstanceID string,
	element state.Element,
	groupIndex int,
) ChildDeployUpdateMessage {
	return ChildDeployUpdateMessage{
		ParentInstanceID: parentInstanceID,
		ChildInstanceID:  element.ID(),
		ChildName:        element.LogicalName(),
		Group:            groupIndex,
		Status:           msg.Status,
		FailureReasons:   msg.FailureReasons,
		UpdateTimestamp:  msg.UpdateTimestamp,
		Durations:        msg.Durations,
	}
}

func getResourceChangeInfo(
	resourceName string,
	changes *BlueprintChanges,
) *resourceChangeDeployInfo {
	for changeResourceName, resourceChanges := range changes.ResourceChanges {
		if changeResourceName == resourceName {
			return &resourceChangeDeployInfo{
				isNew:   false,
				changes: &resourceChanges,
			}
		}
	}

	for newResourceName, resourceChanges := range changes.NewResources {
		if newResourceName == resourceName {
			return &resourceChangeDeployInfo{
				isNew:   true,
				changes: &resourceChanges,
			}
		}
	}

	return nil
}

func getResolvedResourceFromChanges(
	changes *provider.Changes,
) *provider.ResolvedResource {
	if changes == nil {
		return nil
	}

	return changes.AppliedResourceInfo.ResourceWithResolvedSubs
}

type resourceChangeDeployInfo struct {
	isNew   bool
	changes *provider.Changes
}

func resourceHasFieldsToResolve(
	resourceName string,
	resolvePaths []string,
) bool {
	resourceElementIDDotPrefix := fmt.Sprintf("%s.", core.ResourceElementID(resourceName))
	resourceElementIDBracketPrefix := fmt.Sprintf("%s[", core.ResourceElementID(resourceName))

	return slices.ContainsFunc(resolvePaths, func(path string) bool {
		return strings.HasPrefix(path, resourceElementIDDotPrefix) ||
			strings.HasPrefix(path, resourceElementIDBracketPrefix)
	})
}

func prepareResourceChangesForDeployment(
	changes *provider.Changes,
	resolvedResource *provider.ResolvedResource,
	resourceState *state.ResourceState,
	resourceID string,
	instanceID string,
) *provider.Changes {
	return &provider.Changes{
		AppliedResourceInfo: provider.ResourceInfo{
			ResourceID:               resourceID,
			ResourceName:             changes.AppliedResourceInfo.ResourceName,
			InstanceID:               instanceID,
			CurrentResourceState:     resourceState,
			ResourceWithResolvedSubs: resolvedResource,
		},
		MustRecreate:              changes.MustRecreate,
		ModifiedFields:            changes.ModifiedFields,
		NewFields:                 changes.NewFields,
		RemovedFields:             changes.RemovedFields,
		UnchangedFields:           changes.UnchangedFields,
		ComputedFields:            changes.ComputedFields,
		FieldChangesKnownOnDeploy: changes.FieldChangesKnownOnDeploy,
		ConditionKnownOnDeploy:    changes.ConditionKnownOnDeploy,
		NewOutboundLinks:          changes.NewOutboundLinks,
		OutboundLinkChanges:       changes.OutboundLinkChanges,
		RemovedOutboundLinks:      changes.RemovedOutboundLinks,
	}
}

func countElementsToDeploy(changes *BlueprintChanges) int {
	linksToDeployCount := 0
	for _, newResourceChanges := range changes.NewResources {
		linksToDeployCount += len(newResourceChanges.NewOutboundLinks)
	}

	for _, resourceChanges := range changes.ResourceChanges {
		linksToDeployCount += len(resourceChanges.NewOutboundLinks) +
			len(resourceChanges.OutboundLinkChanges)
	}

	return len(changes.NewResources) +
		len(changes.ResourceChanges) +
		len(changes.NewChildren) +
		len(changes.RecreateChildren) +
		len(changes.ChildChanges) +
		linksToDeployCount
}

func createRetryInfo(policy *provider.RetryPolicy) *retryInfo {
	return &retryInfo{
		policy: policy,
		// Start at 0 for first attempt as retries are counted from 1.
		attempt:            0,
		attemptDurations:   []float64{},
		exceededMaxRetries: false,
	}
}

// This is used during the deploy and destroy process to provide an equivalent
// namespaced logical identifier for links that distinguishes link names
// from resource and child blueprint names.
func linkElementID(linkName string) string {
	return fmt.Sprintf("link(%s)", linkName)
}

func pluralise(singular, plural string, count int) string {
	if count == 1 {
		return singular
	}

	return plural
}
