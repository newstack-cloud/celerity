package container

import (
	"context"
	"sync"
	"time"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
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
	instanceTreePath := getInstanceTreePath(paramOverrides, input.InstanceID)
	if exceedsMaxDepth(instanceTreePath, MaxBlueprintDepth) {
		channels.ErrChan <- errMaxBlueprintDepthExceeded(
			instanceTreePath,
			MaxBlueprintDepth,
		)
		return
	}

	ctxWithInstanceID := context.WithValue(ctx, core.BlueprintInstanceIDKey, input.InstanceID)
	state := &deploymentState{
		pendingLinks:               map[string]*linkPendingCompletion{},
		resourceNamePendingLinkMap: map[string][]string{},
		destroyed:                  map[string]state.Element{},
		created:                    map[string]state.Element{},
		updated:                    map[string]state.Element{},
		linkDurationInfo:           map[string]*state.LinkCompletionDurations{},
	}

	c.deploy(
		ctxWithInstanceID,
		input,
		channels,
		state,
		paramOverrides,
	)
}

func (c *defaultBlueprintContainer) deploy(
	ctx context.Context,
	input *DeployInput,
	channels *DeployChannels,
	state *deploymentState,
	paramOverrides core.BlueprintParams,
) {
	if input.Changes == nil {
		channels.FinishChan <- c.createDeploymentFinishedMessage(
			input.InstanceID,
			determineInstanceDeployFailedStatus(input.Rollback),
			[]string{emptyChangesDeployFailedMessage(input.Rollback)},
			/* elapsedTime */ 0,
			/* prepareElapsedTime */ nil,
		)
		return
	}

	startTime := c.clock.Now()
	channels.DeploymentUpdateChan <- DeploymentUpdateMessage{
		InstanceID:      input.InstanceID,
		Status:          core.InstanceStatusPreparing,
		UpdateTimestamp: startTime.Unix(),
	}

	instances := c.stateContainer.Instances()
	currentInstanceState, err := instances.Get(ctx, input.InstanceID)
	if err != nil {
		channels.FinishChan <- c.createDeploymentFinishedMessage(
			input.InstanceID,
			determineInstanceDeployFailedStatus(input.Rollback),
			[]string{prepareFailureMessage},
			c.clock.Since(startTime),
			/* prepareElapsedTime */ nil,
		)
		return
	}

	// Use the same behaviour as change staging to extract the nodes
	// that need to be deployed or updated where they are grouped for concurrent deployment
	// and in order based on links, references and use of the `dependsOn` property.
	processed, err := c.processBlueprint(
		ctx,
		subengine.ResolveForDeployment,
		input.Changes,
		paramOverrides,
	)
	if err != nil {
		channels.FinishChan <- c.createDeploymentFinishedMessage(
			input.InstanceID,
			determineInstanceDeployFailedStatus(input.Rollback),
			[]string{prepareFailureMessage},
			c.clock.Since(startTime),
			/* prepareElapsedTime */ nil,
		)
		return
	}

	deployCtx := &deployContext{
		startTime:             startTime,
		state:                 state,
		rollback:              input.Rollback,
		destroying:            false,
		channels:              channels,
		paramOverrides:        paramOverrides,
		instanceStateSnapshot: &currentInstanceState,
		resourceProviders:     processed.resourceProviderMap,
	}

	flattenedNodes := core.Flatten(processed.parallelGroups)

	_, err = c.removeElements(
		ctx,
		input,
		deployCtx,
		flattenedNodes,
	)
	if err != nil {
		channels.ErrChan <- err
		return
	}

	// Get all components to be deployed or updated.
	// Order components based on dependencies in the blueprint.
	// Group components so those that are not connected can be deployed in parallel.
	// Unlike with change staging, groups are not executed as a unit, they are used as
	// pools to look for components that can be deployed based on the current state of deployment.
	// For each component to be created or updated (including recreated children):
	//  - If resource, resolve condition, check if condition is met, if not, skip deployment.
	//  - call Deploy method component (resource or child blueprint)
	//      - handle specialised provider errors (retryable, resource deploy errors etc.)
	//  - If component is resource and is in config complete status, check if any of its dependents
	//    require the resource to be stable before they can be deployed.
	//    - If so, wait for the resource to be stable before deploying the dependent.
	//    - If not, begin deploying the dependent.
	//  - Check if there are any links that can be deployed based on the current state of deployment.
	//     - If so, deploy the link.
	//	- On failure that can not be retried, roll back all changes successfully made for the current deployment.
	//     - See notes on deployment rollback for how this should be implemented for different states.
}

func (c *defaultBlueprintContainer) handleResourceUpdateMessage(
	ctx context.Context,
	instanceID string,
	msg ResourceDeployUpdateMessage,
	deployCtx *deployContext,
	finished map[string]*deployUpdateMessageWrapper,
) error {
	if msg.InstanceID != instanceID {
		// If message is for a child blueprint, pass through to the client
		// to ensure updates within the child blueprint are surfaced.
		// This allows for the client to provide more detailed feedback to the user
		// for the progress within a child blueprint.
		deployCtx.channels.ResourceUpdateChan <- msg
		return nil
	}

	elementName := core.ResourceElementID(msg.ResourceName)

	if isResourceDestroyEvent(msg.PreciseStatus, deployCtx.rollback) {
		return c.handleResourceDestroyEvent(ctx, msg, deployCtx, finished, elementName)
	}

	return nil
}

func (c *defaultBlueprintContainer) handleChildUpdateMessage(
	ctx context.Context,
	instanceID string,
	msg ChildDeployUpdateMessage,
	deployCtx *deployContext,
	finished map[string]*deployUpdateMessageWrapper,
) error {
	if msg.ParentInstanceID != instanceID {
		// If message is for a child blueprint, pass through to the client
		// to ensure updates within the child blueprint are surfaced.
		// This allows for the client to provide more detailed feedback to the user
		// for the progress within a child blueprint.
		deployCtx.channels.ChildUpdateChan <- msg
		return nil
	}

	elementName := core.ChildElementID(msg.ChildName)

	if isChildDestroyEvent(msg.Status, deployCtx.rollback) {
		return c.handleChildDestroyEvent(ctx, msg, deployCtx, finished, elementName)
	}

	return nil
}

func (c *defaultBlueprintContainer) handleLinkUpdateMessage(
	ctx context.Context,
	instanceID string,
	msg LinkDeployUpdateMessage,
	deployCtx *deployContext,
	finished map[string]*deployUpdateMessageWrapper,
) error {
	if msg.InstanceID != instanceID {
		// If message is for a child blueprint, pass through to the client
		// to ensure updates within the child blueprint are surfaced.
		// This allows for the client to provide more detailed feedback to the user
		// for the progress within a child blueprint.
		deployCtx.channels.LinkUpdateChan <- msg
		return nil
	}

	elementName := linkElementID(msg.LinkName)

	if isLinkDestroyEvent(msg.Status, deployCtx.rollback) {
		return c.handleLinkDestroyEvent(ctx, msg, deployCtx, finished, elementName)
	}

	return nil
}

func (c *defaultBlueprintContainer) updateLinkResourceA(
	ctx context.Context,
	linkImplementation provider.Link,
	input *provider.LinkUpdateResourceInput,
	linkInfo *deploymentElementInfo,
	updateResourceARetryInfo *retryInfo,
	deployCtx *deployContext,
) (*provider.LinkUpdateResourceOutput, bool, error) {
	updateResourceAStartTime := c.clock.Now()
	deployCtx.channels.LinkUpdateChan <- c.createLinkUpdatingResourceAMessage(
		linkInfo,
		deployCtx,
		updateResourceARetryInfo,
		input.LinkUpdateType,
	)

	resourceAOutput, err := linkImplementation.UpdateResourceA(ctx, input)
	if err != nil {
		if provider.IsRetryableError(err) {
			retryErr := err.(*provider.RetryableError)
			return c.handleUpdateLinkResourceARetry(
				ctx,
				linkInfo,
				linkImplementation,
				updateResourceARetryInfo,
				updateResourceAStartTime,
				&linkUpdateResourceInfo{
					failureReasons: []string{retryErr.ChildError.Error()},
					input:          input,
				},
				deployCtx,
			)
		}

		if provider.IsLinkUpdateResourceAError(err) {
			linkUpdateResourceAError := err.(*provider.LinkUpdateResourceAError)
			stop, err := c.handleUpdateResourceATerminalFailure(
				linkInfo,
				updateResourceARetryInfo,
				updateResourceAStartTime,
				&linkUpdateResourceInfo{
					failureReasons: linkUpdateResourceAError.FailureReasons,
					input:          input,
				},
				deployCtx,
			)
			return nil, stop, err
		}

		// For errors that are not wrapped in a provider error, the error is assumed to be fatal
		// and the deployment process will be stopped without reporting a failure state.
		// It is really important that adequate guidance is provided for provider developers
		// to ensure that all errors are wrapped in the appropriate provider error.
		return nil, true, err
	}

	deployCtx.channels.LinkUpdateChan <- c.createLinkResourceAUpdatedMessage(
		linkInfo,
		deployCtx,
		updateResourceARetryInfo,
		input.LinkUpdateType,
		updateResourceAStartTime,
	)

	return resourceAOutput, false, nil
}

func (c *defaultBlueprintContainer) handleUpdateLinkResourceARetry(
	ctx context.Context,
	linkInfo *deploymentElementInfo,
	linkImplementation provider.Link,
	updateResourceARetryInfo *retryInfo,
	updateResourceAStartTime time.Time,
	updateInfo *linkUpdateResourceInfo,
	deployCtx *deployContext,
) (*provider.LinkUpdateResourceOutput, bool, error) {
	currentAttemptDuration := c.clock.Since(updateResourceAStartTime)
	nextRetryInfo := addRetryAttempt(updateResourceARetryInfo, currentAttemptDuration)
	deployCtx.channels.LinkUpdateChan <- LinkDeployUpdateMessage{
		InstanceID: linkInfo.instanceID,
		LinkID:     linkInfo.element.ID(),
		LinkName:   linkInfo.element.LogicalName(),
		Status: determineLinkUpdateFailedStatus(
			deployCtx.rollback,
			updateInfo.input.LinkUpdateType,
		),
		PreciseStatus: determinePreciseLinkResourceAUpdateFailedStatus(
			deployCtx.rollback,
		),
		FailureReasons: updateInfo.failureReasons,
		// Attempt and retry information included the status update is specific to
		// updating resource A, each component of a link change will have its own
		// number of attempts and retry information.
		CurrentStageAttempt:  updateResourceARetryInfo.attempt,
		CanRetryCurrentStage: !nextRetryInfo.exceededMaxRetries,
		UpdateTimestamp:      c.clock.Now().Unix(),
		// Attempt durations will be accumulated and sent in the status updates
		// for each subsequent retry.
		// Total duration will be calculated if retry limit is exceeded.
		Durations: determineLinkUpdateResourceARetryFailureDurations(
			nextRetryInfo,
		),
	}

	if !nextRetryInfo.exceededMaxRetries {
		waitTimeMS := provider.CalculateRetryWaitTimeMS(nextRetryInfo.policy, nextRetryInfo.attempt)
		time.Sleep(time.Duration(waitTimeMS) * time.Millisecond)
		return c.updateLinkResourceA(
			ctx,
			linkImplementation,
			updateInfo.input,
			linkInfo,
			nextRetryInfo,
			deployCtx,
		)
	}

	return nil, true, nil
}

func (c *defaultBlueprintContainer) handleUpdateResourceATerminalFailure(
	linkInfo *deploymentElementInfo,
	updateResourceARetryInfo *retryInfo,
	updateResourceAStartTime time.Time,
	updateInfo *linkUpdateResourceInfo,
	deployCtx *deployContext,
) (bool, error) {
	currentAttemptDuration := c.clock.Since(updateResourceAStartTime)
	deployCtx.channels.LinkUpdateChan <- LinkDeployUpdateMessage{
		InstanceID: linkInfo.instanceID,
		LinkID:     linkInfo.element.ID(),
		LinkName:   linkInfo.element.LogicalName(),
		Status: determineLinkUpdateFailedStatus(
			deployCtx.rollback,
			updateInfo.input.LinkUpdateType,
		),
		PreciseStatus: determinePreciseLinkResourceAUpdateFailedStatus(
			deployCtx.rollback,
		),
		FailureReasons:      updateInfo.failureReasons,
		CurrentStageAttempt: updateResourceARetryInfo.attempt,
		UpdateTimestamp:     c.clock.Now().Unix(),
		Durations: determineLinkUpdateResourceAFinishedDurations(
			updateResourceARetryInfo,
			currentAttemptDuration,
		),
	}

	return true, nil
}

func (c *defaultBlueprintContainer) createLinkUpdatingResourceAMessage(
	linkInfo *deploymentElementInfo,
	deployCtx *deployContext,
	updateResourceARetryInfo *retryInfo,
	linkUpdateType provider.LinkUpdateType,
) LinkDeployUpdateMessage {
	return LinkDeployUpdateMessage{
		InstanceID: linkInfo.instanceID,
		LinkID:     linkInfo.element.ID(),
		LinkName:   linkInfo.element.LogicalName(),
		Status: determineLinkUpdatingStatus(
			deployCtx.rollback,
			linkUpdateType,
		),
		PreciseStatus: determinePreciseLinkUpdatingResourceAStatus(
			deployCtx.rollback,
		),
		UpdateTimestamp:     c.clock.Now().Unix(),
		CurrentStageAttempt: updateResourceARetryInfo.attempt,
	}
}

func (c *defaultBlueprintContainer) createLinkResourceAUpdatedMessage(
	linkInfo *deploymentElementInfo,
	deployCtx *deployContext,
	updateResourceARetryInfo *retryInfo,
	linkUpdateType provider.LinkUpdateType,
	updateResourceAStartTime time.Time,
) LinkDeployUpdateMessage {
	durations := determineLinkUpdateResourceAFinishedDurations(
		updateResourceARetryInfo,
		c.clock.Since(updateResourceAStartTime),
	)
	stashLinkDurationInfo(linkInfo, durations, deployCtx.state)

	return LinkDeployUpdateMessage{
		InstanceID: linkInfo.instanceID,
		LinkID:     linkInfo.element.ID(),
		LinkName:   linkInfo.element.LogicalName(),
		// We are still in the process of updating the link,
		// resource B and intermediary resources still need to be updated.
		Status: determineLinkUpdatingStatus(
			deployCtx.rollback,
			linkUpdateType,
		),
		PreciseStatus:       determinePreciseLinkResourceAUpdatedStatus(deployCtx.rollback),
		UpdateTimestamp:     c.clock.Now().Unix(),
		CurrentStageAttempt: updateResourceARetryInfo.attempt,
		Durations:           durations,
	}
}

func (c *defaultBlueprintContainer) updateLinkResourceB(
	ctx context.Context,
	linkImplementation provider.Link,
	input *provider.LinkUpdateResourceInput,
	linkInfo *deploymentElementInfo,
	updateResourceBRetryInfo *retryInfo,
	deployCtx *deployContext,
) (*provider.LinkUpdateResourceOutput, bool, error) {
	updateResourceBStartTime := c.clock.Now()
	deployCtx.channels.LinkUpdateChan <- c.createLinkUpdatingResourceBMessage(
		linkInfo,
		deployCtx,
		updateResourceBRetryInfo,
		input.LinkUpdateType,
	)

	resourceBOutput, err := linkImplementation.UpdateResourceB(ctx, input)
	if err != nil {
		if provider.IsRetryableError(err) {
			retryErr := err.(*provider.RetryableError)
			return c.handleUpdateLinkResourceBRetry(
				ctx,
				linkInfo,
				linkImplementation,
				updateResourceBRetryInfo,
				updateResourceBStartTime,
				&linkUpdateResourceInfo{
					failureReasons: []string{retryErr.ChildError.Error()},
					input:          input,
				},
				deployCtx,
			)
		}

		if provider.IsLinkUpdateResourceBError(err) {
			linkUpdateResourceBError := err.(*provider.LinkUpdateResourceBError)
			stop, err := c.handleUpdateResourceBTerminalFailure(
				linkInfo,
				updateResourceBRetryInfo,
				updateResourceBStartTime,
				&linkUpdateResourceInfo{
					failureReasons: linkUpdateResourceBError.FailureReasons,
					input:          input,
				},
				deployCtx,
			)
			return nil, stop, err
		}

		// For errors that are not wrapped in a provider error, the error is assumed to be fatal
		// and the deployment process will be stopped without reporting a failure state.
		// It is really important that adequate guidance is provided for provider developers
		// to ensure that all errors are wrapped in the appropriate provider error.
		return nil, true, err
	}

	deployCtx.channels.LinkUpdateChan <- c.createLinkResourceBUpdatedMessage(
		linkInfo,
		deployCtx,
		updateResourceBRetryInfo,
		input.LinkUpdateType,
		updateResourceBStartTime,
	)

	return resourceBOutput, false, nil
}

func (c *defaultBlueprintContainer) handleUpdateLinkResourceBRetry(
	ctx context.Context,
	linkInfo *deploymentElementInfo,
	linkImplementation provider.Link,
	updateResourceBRetryInfo *retryInfo,
	updateResourceBStartTime time.Time,
	updateInfo *linkUpdateResourceInfo,
	deployCtx *deployContext,
) (*provider.LinkUpdateResourceOutput, bool, error) {
	currentAttemptDuration := c.clock.Since(updateResourceBStartTime)
	nextRetryInfo := addRetryAttempt(updateResourceBRetryInfo, currentAttemptDuration)
	deployCtx.channels.LinkUpdateChan <- LinkDeployUpdateMessage{
		InstanceID: linkInfo.instanceID,
		LinkID:     linkInfo.element.ID(),
		LinkName:   linkInfo.element.LogicalName(),
		Status: determineLinkUpdateFailedStatus(
			deployCtx.rollback,
			updateInfo.input.LinkUpdateType,
		),
		PreciseStatus: determinePreciseLinkResourceBUpdateFailedStatus(
			deployCtx.rollback,
		),
		FailureReasons: updateInfo.failureReasons,
		// Attempt and retry information included the status update is specific to
		// updating resource B, each component of a link change will have its own
		// number of attempts and retry information.
		CurrentStageAttempt:  updateResourceBRetryInfo.attempt,
		CanRetryCurrentStage: !nextRetryInfo.exceededMaxRetries,
		UpdateTimestamp:      c.clock.Now().Unix(),
		// Attempt durations will be accumulated and sent in the status updates
		// for each subsequent retry.
		// Total duration will be calculated if retry limit is exceeded.
		Durations: determineLinkUpdateResourceBRetryFailureDurations(
			nextRetryInfo,
		),
	}

	if !nextRetryInfo.exceededMaxRetries {
		waitTimeMS := provider.CalculateRetryWaitTimeMS(nextRetryInfo.policy, nextRetryInfo.attempt)
		time.Sleep(time.Duration(waitTimeMS) * time.Millisecond)
		return c.updateLinkResourceB(
			ctx,
			linkImplementation,
			updateInfo.input,
			linkInfo,
			nextRetryInfo,
			deployCtx,
		)
	}

	return nil, true, nil
}

func (c *defaultBlueprintContainer) handleUpdateResourceBTerminalFailure(
	linkInfo *deploymentElementInfo,
	updateResourceBRetryInfo *retryInfo,
	updateResourceBStartTime time.Time,
	updateInfo *linkUpdateResourceInfo,
	deployCtx *deployContext,
) (bool, error) {
	currentAttemptDuration := c.clock.Since(updateResourceBStartTime)
	accumDurationInfo := getLinkDurationInfo(linkInfo, deployCtx.state)
	durations := determineLinkUpdateResourceBFinishedDurations(
		updateResourceBRetryInfo,
		currentAttemptDuration,
		accumDurationInfo,
	)
	stashLinkDurationInfo(linkInfo, durations, deployCtx.state)
	deployCtx.channels.LinkUpdateChan <- LinkDeployUpdateMessage{
		InstanceID: linkInfo.instanceID,
		LinkID:     linkInfo.element.ID(),
		LinkName:   linkInfo.element.LogicalName(),
		Status: determineLinkUpdateFailedStatus(
			deployCtx.rollback,
			updateInfo.input.LinkUpdateType,
		),
		PreciseStatus: determinePreciseLinkResourceBUpdateFailedStatus(
			deployCtx.rollback,
		),
		FailureReasons:      updateInfo.failureReasons,
		CurrentStageAttempt: updateResourceBRetryInfo.attempt,
		UpdateTimestamp:     c.clock.Now().Unix(),
		Durations:           durations,
	}

	return true, nil
}

func (c *defaultBlueprintContainer) createLinkUpdatingResourceBMessage(
	linkInfo *deploymentElementInfo,
	deployCtx *deployContext,
	updateResourceBRetryInfo *retryInfo,
	linkUpdateType provider.LinkUpdateType,
) LinkDeployUpdateMessage {
	return LinkDeployUpdateMessage{
		InstanceID: linkInfo.instanceID,
		LinkID:     linkInfo.element.ID(),
		LinkName:   linkInfo.element.LogicalName(),
		Status: determineLinkUpdatingStatus(
			deployCtx.rollback,
			linkUpdateType,
		),
		PreciseStatus: determinePreciseLinkUpdatingResourceBStatus(
			deployCtx.rollback,
		),
		UpdateTimestamp:     c.clock.Now().Unix(),
		CurrentStageAttempt: updateResourceBRetryInfo.attempt,
	}
}

func (c *defaultBlueprintContainer) createLinkResourceBUpdatedMessage(
	linkInfo *deploymentElementInfo,
	deployCtx *deployContext,
	updateResourceBRetryInfo *retryInfo,
	linkUpdateType provider.LinkUpdateType,
	updateResourceBStartTime time.Time,
) LinkDeployUpdateMessage {
	accumDurationInfo := getLinkDurationInfo(linkInfo, deployCtx.state)
	durations := determineLinkUpdateResourceBFinishedDurations(
		updateResourceBRetryInfo,
		c.clock.Since(updateResourceBStartTime),
		accumDurationInfo,
	)
	stashLinkDurationInfo(linkInfo, durations, deployCtx.state)
	return LinkDeployUpdateMessage{
		InstanceID: linkInfo.instanceID,
		LinkID:     linkInfo.element.ID(),
		LinkName:   linkInfo.element.LogicalName(),
		// We are still in the process of updating the link,
		// intermediary resources still need to be updated.
		Status: determineLinkUpdatingStatus(
			deployCtx.rollback,
			linkUpdateType,
		),
		PreciseStatus:       determinePreciseLinkResourceBUpdatedStatus(deployCtx.rollback),
		UpdateTimestamp:     c.clock.Now().Unix(),
		CurrentStageAttempt: updateResourceBRetryInfo.attempt,
		Durations:           durations,
	}
}

func (c *defaultBlueprintContainer) updateLinkIntermediaryResources(
	ctx context.Context,
	linkImplementation provider.Link,
	input *provider.LinkUpdateIntermediaryResourcesInput,
	linkInfo *deploymentElementInfo,
	updateIntermediariesRetryInfo *retryInfo,
	deployCtx *deployContext,
) (*provider.LinkUpdateIntermediaryResourcesOutput, error) {
	updateIntermediariesStartTime := c.clock.Now()
	deployCtx.channels.LinkUpdateChan <- c.createLinkUpdatingIntermediaryResourcesMessage(
		linkInfo,
		deployCtx,
		updateIntermediariesRetryInfo,
		input.LinkUpdateType,
	)

	intermediaryResourcesOutput, err := linkImplementation.UpdateIntermediaryResources(ctx, input)
	if err != nil {
		if provider.IsRetryableError(err) {
			retryErr := err.(*provider.RetryableError)
			return c.handleUpdateLinkIntermediaryResourcesRetry(
				ctx,
				linkInfo,
				linkImplementation,
				updateIntermediariesRetryInfo,
				updateIntermediariesStartTime,
				&linkUpdateIntermediaryResourcesInfo{
					failureReasons: []string{retryErr.ChildError.Error()},
					input:          input,
				},
				deployCtx,
			)
		}

		if provider.IsLinkUpdateIntermediaryResourcesError(err) {
			linkUpdateIntermediariesError := err.(*provider.LinkUpdateIntermediaryResourcesError)
			return nil, c.handleUpdateIntermediaryResourcesTerminalFailure(
				linkInfo,
				updateIntermediariesRetryInfo,
				updateIntermediariesStartTime,
				&linkUpdateIntermediaryResourcesInfo{
					failureReasons: linkUpdateIntermediariesError.FailureReasons,
					input:          input,
				},
				deployCtx,
			)
		}

		// For errors that are not wrapped in a provider error, the error is assumed to be fatal
		// and the deployment process will be stopped without reporting a failure state.
		// It is really important that adequate guidance is provided for provider developers
		// to ensure that all errors are wrapped in the appropriate provider error.
		return nil, err
	}

	deployCtx.channels.LinkUpdateChan <- c.createLinkIntermediariesUpdatedMessage(
		linkInfo,
		deployCtx,
		updateIntermediariesRetryInfo,
		input.LinkUpdateType,
		updateIntermediariesStartTime,
	)

	return intermediaryResourcesOutput, nil
}

func (c *defaultBlueprintContainer) createLinkIntermediariesUpdatedMessage(
	linkInfo *deploymentElementInfo,
	deployCtx *deployContext,
	updateIntermediariesRetryInfo *retryInfo,
	linkUpdateType provider.LinkUpdateType,
	updateIntermediariesStartTime time.Time,
) LinkDeployUpdateMessage {
	accumDurationInfo := getLinkDurationInfo(linkInfo, deployCtx.state)
	durations := determineLinkUpdateIntermediariesFinishedDurations(
		updateIntermediariesRetryInfo,
		c.clock.Since(updateIntermediariesStartTime),
		accumDurationInfo,
	)
	stashLinkDurationInfo(linkInfo, durations, deployCtx.state)

	return LinkDeployUpdateMessage{
		InstanceID: linkInfo.instanceID,
		LinkID:     linkInfo.element.ID(),
		LinkName:   linkInfo.element.LogicalName(),
		// Updating intermediary resources is the last step in the link update process.
		Status: determineLinkOperationSuccessfullyFinishedStatus(
			deployCtx.rollback,
			linkUpdateType,
		),
		PreciseStatus: determinePreciseLinkIntermediariesUpdatedStatus(
			deployCtx.rollback,
		),
		UpdateTimestamp:     c.clock.Now().Unix(),
		CurrentStageAttempt: updateIntermediariesRetryInfo.attempt,
		Durations:           durations,
	}
}

func (c *defaultBlueprintContainer) handleUpdateLinkIntermediaryResourcesRetry(
	ctx context.Context,
	linkInfo *deploymentElementInfo,
	linkImplementation provider.Link,
	updateIntermediaryResourcesRetryInfo *retryInfo,
	updateIntermediaryResourcesStartTime time.Time,
	updateInfo *linkUpdateIntermediaryResourcesInfo,
	deployCtx *deployContext,
) (*provider.LinkUpdateIntermediaryResourcesOutput, error) {
	currentAttemptDuration := c.clock.Since(updateIntermediaryResourcesStartTime)
	nextRetryInfo := addRetryAttempt(
		updateIntermediaryResourcesRetryInfo,
		currentAttemptDuration,
	)
	deployCtx.channels.LinkUpdateChan <- LinkDeployUpdateMessage{
		InstanceID: linkInfo.instanceID,
		LinkID:     linkInfo.element.ID(),
		LinkName:   linkInfo.element.LogicalName(),
		Status: determineLinkUpdateFailedStatus(
			deployCtx.rollback,
			updateInfo.input.LinkUpdateType,
		),
		PreciseStatus: determinePreciseLinkIntermediariesUpdateFailedStatus(
			deployCtx.rollback,
		),
		FailureReasons: updateInfo.failureReasons,
		// Attempt and retry information included the status update is specific to
		// updating intermediary resources, each component of a link change will have its own
		// number of attempts and retry information.
		CurrentStageAttempt:  updateIntermediaryResourcesRetryInfo.attempt,
		CanRetryCurrentStage: !nextRetryInfo.exceededMaxRetries,
		UpdateTimestamp:      c.clock.Now().Unix(),
		// Attempt durations will be accumulated and sent in the status updates
		// for each subsequent retry.
		// Total duration will be calculated if retry limit is exceeded.
		Durations: determineLinkUpdateIntermediariesRetryFailureDurations(
			nextRetryInfo,
		),
	}

	if !nextRetryInfo.exceededMaxRetries {
		waitTimeMS := provider.CalculateRetryWaitTimeMS(nextRetryInfo.policy, nextRetryInfo.attempt)
		time.Sleep(time.Duration(waitTimeMS) * time.Millisecond)
		return c.updateLinkIntermediaryResources(
			ctx,
			linkImplementation,
			updateInfo.input,
			linkInfo,
			nextRetryInfo,
			deployCtx,
		)
	}

	return nil, nil
}

func (c *defaultBlueprintContainer) handleUpdateIntermediaryResourcesTerminalFailure(
	linkInfo *deploymentElementInfo,
	updateIntermediariesRetryInfo *retryInfo,
	updateIntermediariesStartTime time.Time,
	updateInfo *linkUpdateIntermediaryResourcesInfo,
	deployCtx *deployContext,
) error {
	currentAttemptDuration := c.clock.Since(updateIntermediariesStartTime)
	accumDurationInfo := getLinkDurationInfo(linkInfo, deployCtx.state)
	durations := determineLinkUpdateIntermediariesFinishedDurations(
		updateIntermediariesRetryInfo,
		currentAttemptDuration,
		accumDurationInfo,
	)
	stashLinkDurationInfo(linkInfo, durations, deployCtx.state)

	deployCtx.channels.LinkUpdateChan <- LinkDeployUpdateMessage{
		InstanceID: linkInfo.instanceID,
		LinkID:     linkInfo.element.ID(),
		LinkName:   linkInfo.element.LogicalName(),
		Status: determineLinkUpdateFailedStatus(
			deployCtx.rollback,
			updateInfo.input.LinkUpdateType,
		),
		PreciseStatus: determinePreciseLinkIntermediariesUpdateFailedStatus(
			deployCtx.rollback,
		),
		FailureReasons:      updateInfo.failureReasons,
		CurrentStageAttempt: updateIntermediariesRetryInfo.attempt,
		UpdateTimestamp:     c.clock.Now().Unix(),
		Durations:           durations,
	}

	return nil
}

func (c *defaultBlueprintContainer) createLinkUpdatingIntermediaryResourcesMessage(
	linkInfo *deploymentElementInfo,
	deployCtx *deployContext,
	updateIntermediariesRetryInfo *retryInfo,
	linkUpdateType provider.LinkUpdateType,
) LinkDeployUpdateMessage {
	return LinkDeployUpdateMessage{
		InstanceID: linkInfo.instanceID,
		LinkID:     linkInfo.element.ID(),
		LinkName:   linkInfo.element.LogicalName(),
		Status: determineLinkUpdatingStatus(
			deployCtx.rollback,
			linkUpdateType,
		),
		PreciseStatus: determinePreciseLinkUpdatingIntermediariesStatus(
			deployCtx.rollback,
		),
		UpdateTimestamp:     c.clock.Now().Unix(),
		CurrentStageAttempt: updateIntermediariesRetryInfo.attempt,
	}
}

func (c *defaultBlueprintContainer) createDeploymentFinishedMessage(
	instanceID string,
	status core.InstanceStatus,
	failureReasons []string,
	elapsedTime time.Duration,
	prepareElapsedTime *time.Duration,
) DeploymentFinishedMessage {
	elapsedMilliseconds := core.FractionalMilliseconds(elapsedTime)
	currentTimestamp := c.clock.Now().Unix()
	msg := DeploymentFinishedMessage{
		InstanceID:      instanceID,
		Status:          status,
		FailureReasons:  failureReasons,
		FinishTimestamp: currentTimestamp,
		UpdateTimestamp: currentTimestamp,
		Durations: &state.InstanceCompletionDuration{
			TotalDuration: &elapsedMilliseconds,
		},
	}

	if prepareElapsedTime != nil {
		prepareEllapsedMilliseconds := core.FractionalMilliseconds(*prepareElapsedTime)
		msg.Durations.PrepareDuration = &prepareEllapsedMilliseconds
	}

	return msg
}

// Keeps track of state regarding when links are ready to be processed
// along with elements that have been successfully processed.
// All instance state including statuses of resources, links and child blueprints
// are stored in the state container.
// This is a temporary representation of the state of the deployment
// that is not persisted.
type deploymentState struct {
	// A mapping of a logical link name to the pending link completion state for links
	// that need to be deployed or updated.
	// A link ID in this context is made up of the resource names of the two resources
	// that are linked together.
	// For example, if resource A is linked to resource B, the link name would be "A::B".
	// This is used to keep track of when links are ready to be deployed or updated.
	// This does not hold the state of the link, only information about when the link is ready
	// to be deployed or updated.
	// Link removals are not tracked here as they do not depend on resource state changes,
	// removal of links happens before resources in the link relationship are processed.
	pendingLinks map[string]*linkPendingCompletion
	// A mapping of resource names to pending links that include the resource.
	resourceNamePendingLinkMap map[string][]string
	// Elements that have been successfully destroyed.
	// This is a mapping of namespaced logical names (e.g. resources.resourceA) to an element
	// representing identifiers and the kind of the element.
	destroyed map[string]state.Element
	// Elements that have been successfully created/deployed.
	// This is a mapping of namespaced logical names (e.g. resources.resourceA) to an element
	// representing identifiers and the kind of the element.
	created map[string]state.Element
	// Elements that have been successfully updated.
	// This is a mapping of namespaced logical names (e.g. resources.resourceA) to an element
	// representing identifiers and the kind of the element.
	updated map[string]state.Element
	// The duration of the preparation phase for the deployment of a blueprint instance.
	prepareDuration *time.Duration
	// A mapping of logical link name to the current duration information for the progress
	// of the link deployment.
	linkDurationInfo map[string]*state.LinkCompletionDurations
	// Mutex is required as resources can be deployed concurrently.
	mu sync.Mutex
}

type deployContext struct {
	startTime  time.Time
	rollback   bool
	destroying bool
	state      *deploymentState
	channels   *DeployChannels
	// A snapshot of the instance state taken before deployment.
	instanceStateSnapshot *state.InstanceState
	paramOverrides        core.BlueprintParams
	resourceProviders     map[string]provider.Provider
	currentGroupIndex     int
}

func deployContextWithChannels(
	deployCtx *deployContext,
	channels *DeployChannels,
) *deployContext {
	return &deployContext{
		startTime:             deployCtx.startTime,
		state:                 deployCtx.state,
		channels:              channels,
		rollback:              deployCtx.rollback,
		destroying:            deployCtx.destroying,
		instanceStateSnapshot: deployCtx.instanceStateSnapshot,
		paramOverrides:        deployCtx.paramOverrides,
		resourceProviders:     deployCtx.resourceProviders,
		currentGroupIndex:     deployCtx.currentGroupIndex,
	}
}

func deployContextWithGroup(
	deployCtx *deployContext,
	groupIndex int,
) *deployContext {
	return &deployContext{
		startTime:             deployCtx.startTime,
		state:                 deployCtx.state,
		channels:              deployCtx.channels,
		rollback:              deployCtx.rollback,
		destroying:            deployCtx.destroying,
		instanceStateSnapshot: deployCtx.instanceStateSnapshot,
		paramOverrides:        deployCtx.paramOverrides,
		resourceProviders:     deployCtx.resourceProviders,
		currentGroupIndex:     groupIndex,
	}
}

type deployUpdateMessageWrapper struct {
	resourceUpdateMessage *ResourceDeployUpdateMessage
	linkUpdateMessage     *LinkDeployUpdateMessage
	childUpdateMessage    *ChildDeployUpdateMessage
}

type retryInfo struct {
	attempt            int
	exceededMaxRetries bool
	policy             *provider.RetryPolicy
	attemptDurations   []float64
}

type linkUpdateResourceInfo struct {
	failureReasons []string
	input          *provider.LinkUpdateResourceInput
}

type linkUpdateIntermediaryResourcesInfo struct {
	failureReasons []string
	input          *provider.LinkUpdateIntermediaryResourcesInput
}

type deploymentElementInfo struct {
	element    state.Element
	instanceID string
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
	// Attempt is the current attempt number for deploying or destroying the resource.
	Attempt int `json:"attempt"`
	// CanRetry indicates if the operation for the resource can be retried
	// after this attempt.
	CanRetry bool `json:"canRetry"`
	// UpdateTimestamp is the unix timestamp in seconds for
	// when the status update occurred.
	UpdateTimestamp int64 `json:"updateTimestamp"`
	// Durations holds duration information for a resource deployment.
	// Duration information is attached on one of the following precise status updates:
	// - PreciseResourceStatusConfigComplete
	// - PreciseResourceStatusCreated
	// - PreciseResourceStatusCreateFailed
	// - PreciseResourceStatusCreateRollbackFailed
	// - PreciseResourceStatusCreateRollbackComplete
	// - PreciseResourceStatusDestroyed
	// - PreciseResourceStatusDestroyFailed
	// - PreciseResourceStatusDestroyRollbackFailed
	// - PreciseResourceStatusDestroyRollbackComplete
	// - PreciseResourceStatusUpdateConfigComplete
	// - PreciseResourceStatusUpdated
	// - PreciseResourceStatusUpdateFailed
	// - PreciseResourceStatusUpdateRollbackFailed
	// - PreciseResourceStatusUpdateRollbackComplete
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
	// Attempt is the current attempt number for applying the changes
	// for the current stage of the link deployment/removal.
	CurrentStageAttempt int `json:"currentStageAttempt"`
	// CanRetryCurrentStage indicates if the operation for the link can be retried
	// after this attempt of the current stage.
	CanRetryCurrentStage bool `json:"canRetryCurrentStage"`
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
	ChildInstanceID string `json:"childInstanceId"`
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
	// UpdateTimestamp is the unix timestamp in seconds for
	// when the status update occurred.
	UpdateTimestamp int64 `json:"updateTimestamp"`
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
	// UpdateTimestamp is the unix timestamp in seconds for
	// when the status update occurred.
	UpdateTimestamp int64 `json:"updateTimestamp"`
	// Durations holds duration information for the blueprint deployment.
	// Duration information is attached on one of the following status updates:
	// - InstanceStatusDeploying (preparation phase duration only)
	// - InstanceStatusDeployed
	// - InstanceStatusDeployFailed
	// - InstanceStatusDestroyed
	// - InstanceStatusUpdated
	// - InstanceStatusUpdateFailed
	Durations *state.InstanceCompletionDuration `json:"durations,omitempty"`
}
