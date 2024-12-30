package container

import (
	"context"
	"strings"
	"time"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

const (
	prepareDestroyFailureMessage = "failed to load instance state while preparing to destroy"
)

func (c *defaultBlueprintContainer) Destroy(
	ctx context.Context,
	input *DestroyInput,
	channels *DeployChannels,
	paramOverrides core.BlueprintParams,
) {
	ctxWithInstanceID := context.WithValue(ctx, core.BlueprintInstanceIDKey, input.InstanceID)
	state := &deploymentState{
		destroyed:        map[string]state.Element{},
		linkDurationInfo: map[string]*state.LinkCompletionDurations{},
	}
	go c.destroy(
		ctxWithInstanceID,
		input,
		channels,
		state,
		paramOverrides,
	)
}

func (c *defaultBlueprintContainer) destroy(
	ctx context.Context,
	input *DestroyInput,
	channels *DeployChannels,
	state *deploymentState,
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

	if input.Changes == nil {
		channels.FinishChan <- c.createDeploymentFinishedMessage(
			input.InstanceID,
			determineInstanceDestroyFailedStatus(input.Rollback),
			[]string{
				emptyChangesDestroyFailedMessage(input.Rollback),
			},
			/* elapsedTime */ 0,
			/* prepareElapsedTime */ nil,
		)
		return
	}

	startTime := c.clock.Now()
	channels.DeploymentUpdateChan <- DeploymentUpdateMessage{
		InstanceID:      input.InstanceID,
		Status:          determineInstanceDestroyingStatus(input.Rollback),
		UpdateTimestamp: startTime.Unix(),
	}

	instances := c.stateContainer.Instances()
	currentInstanceState, err := instances.Get(ctx, input.InstanceID)
	if err != nil {
		channels.FinishChan <- c.createDeploymentFinishedMessage(
			input.InstanceID,
			determineInstanceDestroyFailedStatus(input.Rollback),
			[]string{prepareDestroyFailureMessage},
			c.clock.Since(startTime),
			/* prepareElapsedTime */ nil,
		)
		return
	}

	resourceProviderMap := c.resourceProviderMapFromState(&currentInstanceState)

	finished, err := c.removeElements(
		ctx,
		&DeployInput{
			InstanceID: input.InstanceID,
			Changes:    input.Changes,
			Rollback:   input.Rollback,
		},
		&deployContext{
			startTime:             startTime,
			state:                 state,
			rollback:              input.Rollback,
			destroying:            true,
			channels:              channels,
			paramOverrides:        paramOverrides,
			instanceStateSnapshot: &currentInstanceState,
			resourceProviders:     resourceProviderMap,
		},
		[]*DeploymentNode{},
	)
	if err != nil {
		channels.ErrChan <- wrapErrorForChildContext(err, paramOverrides)
		return
	}

	if finished {
		return
	}

	sentFinishedMessage := c.removeBlueprintInstanceFromState(ctx, input, channels, startTime, instances)
	if sentFinishedMessage {
		return
	}

	channels.FinishChan <- c.createDeploymentFinishedMessage(
		input.InstanceID,
		determineInstanceDestroyedStatus(input.Rollback),
		[]string{},
		c.clock.Since(startTime),
		/* prepareElapsedTime */ nil,
	)
}

func (c *defaultBlueprintContainer) removeBlueprintInstanceFromState(
	ctx context.Context,
	input *DestroyInput,
	channels *DeployChannels,
	startTime time.Time,
	instances state.InstancesContainer,
) bool {
	_, err := instances.Remove(ctx, input.InstanceID)
	if err != nil {
		channels.FinishChan <- c.createDeploymentFinishedMessage(
			input.InstanceID,
			determineInstanceDestroyFailedStatus(input.Rollback),
			[]string{err.Error()},
			c.clock.Since(startTime),
			/* prepareElapsedTime */ nil,
		)
		return true
	}

	return false
}

func (c *defaultBlueprintContainer) resourceProviderMapFromState(
	currentInstanceState *state.InstanceState,
) map[string]provider.Provider {
	resourceProviderMap := map[string]provider.Provider{}
	for _, resourceState := range currentInstanceState.Resources {
		providerNamespace := strings.Split(resourceState.ResourceType, "/")[0]
		resourceProviderMap[resourceState.ResourceName] = c.providers[providerNamespace]
	}
	return resourceProviderMap
}

func (c *defaultBlueprintContainer) removeElements(
	ctx context.Context,
	input *DeployInput,
	deployCtx *deployContext,
	nodesToBeDeployed []*DeploymentNode,
) (bool, error) {

	elementsToRemove, finished, err := c.collectElementsToRemove(
		input.Changes,
		deployCtx,
		nodesToBeDeployed,
	)
	if err != nil {
		return true, err
	}

	if finished {
		return true, nil
	}

	orderedElements := OrderElementsForRemoval(
		elementsToRemove,
		deployCtx.instanceStateSnapshot,
	)
	groupedElements := GroupOrderedElementsForRemoval(orderedElements)

	if !deployCtx.destroying {
		// Stash the prepare duration here for both destroy and deploy as where there are
		// elements to be removed, they will always be processed first, this allows us to more
		// accurately track duration as the prepare phase is complete once the elements to be
		// removed have been collected, ordered and grouped.
		// In the case where there are no elements to be removed, the prepare duration
		// will be stashed in the deploy phase.
		stashPrepareDuration(
			c.clock.Since(deployCtx.startTime),
			deployCtx.state,
		)
		deployCtx.channels.DeploymentUpdateChan <- DeploymentUpdateMessage{
			InstanceID:      input.InstanceID,
			Status:          core.InstanceStatusDeploying,
			UpdateTimestamp: c.clock.Now().Unix(),
		}
	}

	stopProcessing, err := c.removeGroupedElements(
		ctx,
		groupedElements,
		input.InstanceID,
		deployCtx,
	)
	if err != nil {
		return stopProcessing, err
	}

	return stopProcessing, nil
}

func (c *defaultBlueprintContainer) removeGroupedElements(
	ctx context.Context,
	parallelGroups [][]state.Element,
	instanceID string,
	deployCtx *deployContext,
) (bool, error) {
	internalChannels := CreateDeployChannels()

	stopProcessing := false
	i := 0
	var err error
	for !stopProcessing && i < len(parallelGroups) {
		group := parallelGroups[i]
		c.stageGroupRemovals(
			ctx,
			instanceID,
			group,
			deployContextWithGroup(
				deployContextWithChannels(deployCtx, internalChannels),
				i,
			),
		)

		stopProcessing, err = c.listenToAndProcessGroupRemovals(
			ctx,
			instanceID,
			group,
			deployCtx,
			internalChannels,
		)
		i += 1
	}

	return stopProcessing, err
}

func (c *defaultBlueprintContainer) listenToAndProcessGroupRemovals(
	ctx context.Context,
	instanceID string,
	group []state.Element,
	deployCtx *deployContext,
	internalChannels *DeployChannels,
) (bool, error) {
	finished := map[string]*deployUpdateMessageWrapper{}

	var err error
	for (len(finished) < len(group)) &&
		err == nil {
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case msg := <-internalChannels.ResourceUpdateChan:
			err = c.handleResourceUpdateMessage(ctx, instanceID, msg, deployCtx, finished)
		case msg := <-internalChannels.ChildUpdateChan:
			err = c.handleChildUpdateMessage(ctx, instanceID, msg, deployCtx, finished)
		case msg := <-internalChannels.LinkUpdateChan:
			err = c.handleLinkUpdateMessage(ctx, instanceID, msg, deployCtx, finished)
		case err = <-internalChannels.ErrChan:
		}
	}

	if err != nil {
		return true, err
	}

	failed := getFailedRemovalsAndUpdateState(finished, group, deployCtx.state, deployCtx.rollback)
	if len(failed) > 0 {
		deployCtx.channels.FinishChan <- c.createDeploymentFinishedMessage(
			instanceID,
			determineFinishedFailureStatus(deployCtx.destroying, deployCtx.rollback),
			finishedFailureMessages(deployCtx, failed),
			c.clock.Since(deployCtx.startTime),
			// prepareDuration is written to once, before the first group is processed;
			// this makes it is safe to read it here without locking.
			/* prepareElapsedTime */
			deployCtx.state.prepareDuration,
		)
		return true, nil
	}

	return false, nil
}

func (c *defaultBlueprintContainer) handleResourceDestroyEvent(
	ctx context.Context,
	msg ResourceDeployUpdateMessage,
	deployCtx *deployContext,
	finished map[string]*deployUpdateMessageWrapper,
	elementName string,
) error {
	resources := c.stateContainer.Resources()
	if startedDestroyingResource(msg.PreciseStatus, deployCtx.rollback) {
		err := resources.UpdateStatus(
			ctx,
			msg.InstanceID,
			msg.ResourceID,
			state.ResourceStatusInfo{
				Status:        msg.Status,
				PreciseStatus: msg.PreciseStatus,
			},
		)
		if err != nil {
			return err
		}
	}

	if finishedDestroyingResource(msg, deployCtx.rollback) {
		finished[elementName] = &deployUpdateMessageWrapper{
			resourceUpdateMessage: &msg,
		}

		if wasResourceDestroyedSuccessfully(msg.PreciseStatus, deployCtx.rollback) {
			_, err := resources.Remove(
				ctx,
				msg.InstanceID,
				msg.ResourceID,
			)
			if err != nil {
				return err
			}
		} else {
			err := resources.UpdateStatus(
				ctx,
				msg.InstanceID,
				msg.ResourceID,
				state.ResourceStatusInfo{
					Status:         msg.Status,
					PreciseStatus:  msg.PreciseStatus,
					FailureReasons: msg.FailureReasons,
					Durations:      msg.Durations,
				},
			)
			if err != nil {
				return err
			}
		}
	}

	deployCtx.channels.ResourceUpdateChan <- msg
	return nil
}

func (c *defaultBlueprintContainer) handleChildDestroyEvent(
	ctx context.Context,
	msg ChildDeployUpdateMessage,
	deployCtx *deployContext,
	finished map[string]*deployUpdateMessageWrapper,
	elementName string,
) error {
	instances := c.stateContainer.Instances()
	children := c.stateContainer.Children()
	if startedDestroyingChild(msg.Status, deployCtx.rollback) {
		err := instances.UpdateStatus(
			ctx,
			msg.ChildInstanceID,
			state.InstanceStatusInfo{
				Status: msg.Status,
			},
		)
		if err != nil {
			return err
		}
	}

	if finishedDestroyingChild(msg, deployCtx.rollback) {
		finished[elementName] = &deployUpdateMessageWrapper{
			childUpdateMessage: &msg,
		}

		if wasChildDestroyedSuccessfully(msg.Status, deployCtx.rollback) {
			_, err := children.Remove(
				ctx,
				msg.ParentInstanceID,
				msg.ChildName,
			)
			if err != nil {
				return err
			}
		} else {
			err := instances.UpdateStatus(
				ctx,
				msg.ChildInstanceID,
				state.InstanceStatusInfo{
					Status:    msg.Status,
					Durations: msg.Durations,
				},
			)
			if err != nil {
				return err
			}
		}
	}

	deployCtx.channels.ChildUpdateChan <- msg
	return nil
}

func (c *defaultBlueprintContainer) handleLinkDestroyEvent(
	ctx context.Context,
	msg LinkDeployUpdateMessage,
	deployCtx *deployContext,
	finished map[string]*deployUpdateMessageWrapper,
	elementName string,
) error {
	links := c.stateContainer.Links()
	if startedDestroyingLink(msg.Status, deployCtx.rollback) {
		err := links.UpdateStatus(
			ctx,
			msg.InstanceID,
			msg.LinkID,
			state.LinkStatusInfo{
				Status:        msg.Status,
				PreciseStatus: msg.PreciseStatus,
				// For links, there are multiple stages to the destroy process,
				// a status update for each stage will contain
				// duration information for the previous stages.
				Durations: msg.Durations,
			},
		)
		if err != nil {
			return err
		}
	}

	if finishedDestroyingLink(msg, deployCtx.rollback) {
		finished[elementName] = &deployUpdateMessageWrapper{
			linkUpdateMessage: &msg,
		}

		if wasLinkDestroyedSuccessfully(msg.Status, deployCtx.rollback) {
			_, err := links.Remove(
				ctx,
				msg.InstanceID,
				msg.LinkID,
			)
			if err != nil {
				return err
			}
		} else {
			err := links.UpdateStatus(
				ctx,
				msg.InstanceID,
				msg.LinkID,
				state.LinkStatusInfo{
					Status:         msg.Status,
					PreciseStatus:  msg.PreciseStatus,
					FailureReasons: msg.FailureReasons,
					Durations:      msg.Durations,
				},
			)
			if err != nil {
				return err
			}
		}
	}

	deployCtx.channels.LinkUpdateChan <- msg
	return nil
}

func stashPrepareDuration(
	prepareDuration time.Duration,
	state *deploymentState,
) {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.prepareDuration = &prepareDuration
}

func (c *defaultBlueprintContainer) stageGroupRemovals(
	ctx context.Context,
	instanceID string,
	group []state.Element,
	deployCtx *deployContext,
) {
	instanceTreePath := getInstanceTreePath(deployCtx.paramOverrides, instanceID)

	for _, element := range group {
		if element.Kind() == state.ResourceElement {
			go c.prepareAndDestroyResource(
				ctx,
				element,
				instanceID,
				deployCtx,
			)
		} else if element.Kind() == state.ChildElement {
			includeTreePath := getIncludeTreePath(deployCtx.paramOverrides, element.LogicalName())
			go c.prepareAndDestroyChild(
				ctx,
				element,
				instanceID,
				instanceTreePath,
				includeTreePath,
				deployCtx,
			)
		} else if element.Kind() == state.LinkElement {
			go c.prepareAndDestroyLink(
				ctx,
				element,
				instanceID,
				deployCtx,
			)
		}
	}
}

func (c *defaultBlueprintContainer) prepareAndDestroyResource(
	ctx context.Context,
	resourceElement state.Element,
	instanceID string,
	deployCtx *deployContext,
) {
	resourceState := getResourceStateByName(
		deployCtx.instanceStateSnapshot,
		resourceElement.LogicalName(),
	)
	if resourceState == nil {
		deployCtx.channels.ErrChan <- errResourceNotFoundInState(
			resourceElement.LogicalName(),
			instanceID,
		)
		return
	}

	resourceImplementation, err := c.getProviderResourceImplementation(
		ctx,
		resourceElement.LogicalName(),
		resourceState.ResourceType,
		deployCtx.resourceProviders,
	)
	if err != nil {
		deployCtx.channels.ErrChan <- err
		return
	}

	policy, err := c.getRetryPolicy(ctx, deployCtx.resourceProviders, resourceElement.LogicalName())
	if err != nil {
		deployCtx.channels.ErrChan <- err
		return
	}

	err = c.destroyResource(
		ctx,
		&deploymentElementInfo{
			element:    resourceElement,
			instanceID: instanceID,
		},
		resourceImplementation,
		deployCtx,
		createRetryInfo(policy),
	)
	if err != nil {
		deployCtx.channels.ErrChan <- err
	}
}

func (c *defaultBlueprintContainer) destroyResource(
	ctx context.Context,
	resourceInfo *deploymentElementInfo,
	resourceImplementation provider.Resource,
	deployCtx *deployContext,
	resourceRetryInfo *retryInfo,
) error {
	resourceRemovalStartTime := c.clock.Now()
	deployCtx.channels.ResourceUpdateChan <- ResourceDeployUpdateMessage{
		InstanceID:      resourceInfo.instanceID,
		ResourceID:      resourceInfo.element.ID(),
		ResourceName:    resourceInfo.element.LogicalName(),
		Group:           deployCtx.currentGroupIndex,
		Status:          determineResourceDestroyingStatus(deployCtx.rollback),
		PreciseStatus:   determinePreciseResourceDestroyingStatus(deployCtx.rollback),
		UpdateTimestamp: c.clock.Now().Unix(),
		Attempt:         resourceRetryInfo.attempt,
	}

	resourceState := getResourceStateByName(
		deployCtx.instanceStateSnapshot,
		resourceInfo.element.LogicalName(),
	)
	err := resourceImplementation.Destroy(ctx, &provider.ResourceDestroyInput{
		InstanceID:    resourceInfo.instanceID,
		ResourceID:    resourceInfo.element.ID(),
		ResourceState: resourceState,
		Params:        deployCtx.paramOverrides,
	})
	if err != nil {
		if provider.IsRetryableError(err) {
			retryErr := err.(*provider.RetryableError)
			return c.handleDestroyResourceRetry(
				ctx,
				resourceInfo,
				resourceImplementation,
				resourceRetryInfo,
				resourceRemovalStartTime,
				[]string{retryErr.ChildError.Error()},
				deployCtx,
			)
		}

		if provider.IsResourceDestroyError(err) {
			resourceDestroyErr := err.(*provider.ResourceDestroyError)
			return c.handleDestroyResourceTerminalFailure(
				resourceInfo,
				resourceRetryInfo,
				resourceRemovalStartTime,
				resourceDestroyErr.FailureReasons,
				deployCtx,
			)
		}

		// For errors that are not wrapped in a provider error, the error is assumed to be fatal
		// and the deployment process will be stopped without reporting a failure state.
		// It is really important that adequate guidance is provided for provider developers
		// to ensure that all errors are wrapped in the appropriate provider error.
		return err
	}

	deployCtx.channels.ResourceUpdateChan <- ResourceDeployUpdateMessage{
		InstanceID:      resourceInfo.instanceID,
		ResourceID:      resourceInfo.element.ID(),
		ResourceName:    resourceInfo.element.LogicalName(),
		Group:           deployCtx.currentGroupIndex,
		Status:          determineResourceDestroyedStatus(deployCtx.rollback),
		PreciseStatus:   determinePreciseResourceDestroyedStatus(deployCtx.rollback),
		UpdateTimestamp: c.clock.Now().Unix(),
		Attempt:         resourceRetryInfo.attempt,
		Durations: determineResourceDestroyFinishedDurations(
			resourceRetryInfo,
			c.clock.Since(resourceRemovalStartTime),
		),
	}

	return nil
}

func (c *defaultBlueprintContainer) handleDestroyResourceRetry(
	ctx context.Context,
	resourceInfo *deploymentElementInfo,
	resourceImplementation provider.Resource,
	resourceRetryInfo *retryInfo,
	resourceRemovalStartTime time.Time,
	failureReasons []string,
	deployCtx *deployContext,
) error {
	currentAttemptDuration := c.clock.Since(resourceRemovalStartTime)
	nextRetryInfo := addRetryAttempt(resourceRetryInfo, currentAttemptDuration)
	deployCtx.channels.ResourceUpdateChan <- ResourceDeployUpdateMessage{
		InstanceID:      resourceInfo.instanceID,
		ResourceID:      resourceInfo.element.ID(),
		ResourceName:    resourceInfo.element.LogicalName(),
		Group:           deployCtx.currentGroupIndex,
		Status:          determineResourceDestroyFailedStatus(deployCtx.rollback),
		PreciseStatus:   determinePreciseResourceDestroyFailedStatus(deployCtx.rollback),
		FailureReasons:  failureReasons,
		Attempt:         resourceRetryInfo.attempt,
		CanRetry:        !nextRetryInfo.exceededMaxRetries,
		UpdateTimestamp: c.clock.Now().Unix(),
		// Attempt durations will be accumulated and sent in the status updates
		// for each subsequent retry.
		// Total duration will be calculated if retry limit is exceeded.
		Durations: determineResourceRetryFailureDurations(
			nextRetryInfo,
		),
	}

	if !nextRetryInfo.exceededMaxRetries {
		waitTimeMS := provider.CalculateRetryWaitTimeMS(nextRetryInfo.policy, nextRetryInfo.attempt)
		time.Sleep(time.Duration(waitTimeMS) * time.Millisecond)
		return c.destroyResource(
			ctx,
			resourceInfo,
			resourceImplementation,
			deployCtx,
			nextRetryInfo,
		)
	}

	return nil
}

func (c *defaultBlueprintContainer) handleDestroyResourceTerminalFailure(
	resourceInfo *deploymentElementInfo,
	resourceRetryInfo *retryInfo,
	resourceRemovalStartTime time.Time,
	failureReasons []string,
	deployCtx *deployContext,
) error {
	currentAttemptDuration := c.clock.Since(resourceRemovalStartTime)
	deployCtx.channels.ResourceUpdateChan <- ResourceDeployUpdateMessage{
		InstanceID:      resourceInfo.instanceID,
		ResourceID:      resourceInfo.element.ID(),
		ResourceName:    resourceInfo.element.LogicalName(),
		Group:           deployCtx.currentGroupIndex,
		Status:          determineResourceDestroyFailedStatus(deployCtx.rollback),
		PreciseStatus:   determinePreciseResourceDestroyFailedStatus(deployCtx.rollback),
		FailureReasons:  failureReasons,
		Attempt:         resourceRetryInfo.attempt,
		CanRetry:        false,
		UpdateTimestamp: c.clock.Now().Unix(),
		Durations: determineResourceDestroyFinishedDurations(
			resourceRetryInfo,
			currentAttemptDuration,
		),
	}

	return nil
}

func (c *defaultBlueprintContainer) prepareAndDestroyChild(
	ctx context.Context,
	element state.Element,
	parentInstanceID string,
	parentInstanceTreePath string,
	includeTreePath string,
	deployCtx *deployContext,
) {
	childState := getChildStateByName(deployCtx.instanceStateSnapshot, element.LogicalName())
	if childState == nil {
		deployCtx.channels.ErrChan <- errChildNotFoundInState(
			element.LogicalName(),
			parentInstanceID,
		)
		return
	}
	destroyChildChanges := createDestroyChangesFromChildState(childState)

	childParams := deployCtx.paramOverrides.
		WithContextVariables(
			createContextVarsForChildBlueprint(
				parentInstanceID,
				parentInstanceTreePath,
				includeTreePath,
			),
			/* keepExisting */ true,
		)

	// Create an intermediary set of channels so we can dispatch child blueprint-wide
	// events to the parent blueprint's channels.
	// Resource and link events will be passed through to be surfaced to the user,
	// trusting that they wil be handled within the Destroy call for the child blueprint.
	childChannels := CreateDeployChannels()
	// Destroy does not make use of the loaded blueprint spec directly.
	// For this reason, we don't need to load an entirely new container
	// for destroying a child blueprint instance.
	// Destroy relies purely on the provided blueprint changes and the current state
	// of the instance persisted in the state container.
	c.Destroy(
		ctx,
		&DestroyInput{
			InstanceID: element.ID(),
			Changes:    destroyChildChanges,
			Rollback:   deployCtx.rollback,
		},
		childChannels,
		childParams,
	)

	finished := false
	var err error
	for !finished && err == nil {
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case msg := <-childChannels.DeploymentUpdateChan:
			deployCtx.channels.ChildUpdateChan <- updateToChildUpdateMessage(
				&msg,
				parentInstanceID,
				element,
				deployCtx.currentGroupIndex,
			)
		case msg := <-childChannels.FinishChan:
			deployCtx.channels.ChildUpdateChan <- finishedToChildUpdateMessage(
				&msg,
				parentInstanceID,
				element,
				deployCtx.currentGroupIndex,
			)
			finished = true
		case msg := <-childChannels.ResourceUpdateChan:
			deployCtx.channels.ResourceUpdateChan <- msg
		case msg := <-childChannels.LinkUpdateChan:
			deployCtx.channels.LinkUpdateChan <- msg
		case msg := <-childChannels.ChildUpdateChan:
			deployCtx.channels.ChildUpdateChan <- msg
		case err = <-childChannels.ErrChan:
		}
	}
}

func (c *defaultBlueprintContainer) prepareAndDestroyLink(
	ctx context.Context,
	linkElement state.Element,
	instanceID string,
	deployCtx *deployContext,
) {
	linkState := getLinkStateByName(
		deployCtx.instanceStateSnapshot,
		linkElement.LogicalName(),
	)
	if linkState == nil {
		deployCtx.channels.ErrChan <- errLinkNotFoundInState(
			linkElement.LogicalName(),
			instanceID,
		)
		return
	}

	linkImplementation, err := c.getProviderLinkImplementation(
		ctx,
		linkElement.LogicalName(),
		deployCtx.instanceStateSnapshot,
	)
	if err != nil {
		deployCtx.channels.ErrChan <- err
		return
	}

	retryPolicy, err := c.getLinkRetryPolicy(
		ctx,
		linkElement.LogicalName(),
		deployCtx.instanceStateSnapshot,
	)
	if err != nil {
		deployCtx.channels.ErrChan <- err
		return
	}

	err = c.destroyLink(
		ctx,
		&deploymentElementInfo{
			element:    linkElement,
			instanceID: instanceID,
		},
		linkImplementation,
		deployCtx,
		retryPolicy,
	)
	if err != nil {
		deployCtx.channels.ErrChan <- err
	}
}

func (c *defaultBlueprintContainer) destroyLink(
	ctx context.Context,
	linkInfo *deploymentElementInfo,
	linkImplementation provider.Link,
	deployCtx *deployContext,
	retryPolicy *provider.RetryPolicy,
) error {
	linkDependencyInfo := extractLinkDirectDependencies(
		linkInfo.element.LogicalName(),
	)

	resourceAInfo := getResourceInfoFromStateForLinkRemoval(
		deployCtx.instanceStateSnapshot,
		linkDependencyInfo.resourceAName,
	)
	_, stop, err := c.updateLinkResourceA(
		ctx,
		linkImplementation,
		&provider.LinkUpdateResourceInput{
			ResourceInfo:   resourceAInfo,
			LinkUpdateType: provider.LinkUpdateTypeDestroy,
			Params:         deployCtx.paramOverrides,
		},
		linkInfo,
		createRetryInfo(retryPolicy),
		deployCtx,
	)
	if err != nil {
		return err
	}
	if stop {
		return nil
	}

	resourceBInfo := getResourceInfoFromStateForLinkRemoval(
		deployCtx.instanceStateSnapshot,
		linkDependencyInfo.resourceBName,
	)
	_, stop, err = c.updateLinkResourceB(
		ctx,
		linkImplementation,
		&provider.LinkUpdateResourceInput{
			ResourceInfo:   resourceBInfo,
			LinkUpdateType: provider.LinkUpdateTypeDestroy,
			Params:         deployCtx.paramOverrides,
		},
		linkInfo,
		createRetryInfo(retryPolicy),
		deployCtx,
	)
	if err != nil {
		return err
	}
	if stop {
		return nil
	}

	_, err = c.updateLinkIntermediaryResources(
		ctx,
		linkImplementation,
		&provider.LinkUpdateIntermediaryResourcesInput{
			ResourceAInfo:  resourceAInfo,
			ResourceBInfo:  resourceBInfo,
			LinkUpdateType: provider.LinkUpdateTypeDestroy,
			Params:         deployCtx.paramOverrides,
		},
		linkInfo,
		createRetryInfo(retryPolicy),
		deployCtx,
	)
	if err != nil {
		return err
	}

	return nil
}

func (c *defaultBlueprintContainer) getProviderLinkImplementation(
	ctx context.Context,
	linkName string,
	currentState *state.InstanceState,
) (provider.Link, error) {

	resourceTypeA, resourceTypeB, err := getResourceTypesForLink(linkName, currentState)
	if err != nil {
		return nil, err
	}

	return c.linkRegistry.Link(ctx, resourceTypeA, resourceTypeB)
}

func (c *defaultBlueprintContainer) collectElementsToRemove(
	changes *BlueprintChanges,
	deployCtx *deployContext,
	nodesToBeDeployed []*DeploymentNode,
) (*CollectedElements, bool, error) {
	if len(changes.RemovedChildren) == 0 &&
		len(changes.RemovedResources) == 0 &&
		len(changes.RemovedLinks) == 0 {
		return &CollectedElements{}, false, nil
	}

	resourcesToRemove, err := c.collectResourcesToRemove(
		deployCtx.instanceStateSnapshot,
		changes,
		nodesToBeDeployed,
	)
	if err != nil {
		message := getDeploymentErrorSpecificMessage(err, prepareFailureMessage)
		deployCtx.channels.FinishChan <- c.createDeploymentFinishedMessage(
			deployCtx.instanceStateSnapshot.InstanceID,
			determineFinishedFailureStatus(deployCtx.destroying, deployCtx.rollback),
			[]string{message},
			c.clock.Since(deployCtx.startTime),
			/* prepareElapsedTime */ nil,
		)
		return &CollectedElements{}, true, nil
	}

	childrenToRemove, err := c.collectChildrenToRemove(
		deployCtx.instanceStateSnapshot,
		changes,
		nodesToBeDeployed,
	)
	if err != nil {
		message := getDeploymentErrorSpecificMessage(err, prepareFailureMessage)
		deployCtx.channels.FinishChan <- c.createDeploymentFinishedMessage(
			deployCtx.instanceStateSnapshot.InstanceID,
			determineFinishedFailureStatus(deployCtx.destroying, deployCtx.rollback),
			[]string{message},
			c.clock.Since(deployCtx.startTime),
			/* prepareElapsedTime */ nil,
		)
		return &CollectedElements{}, true, nil
	}

	linksToRemove := c.collectLinksToRemove(deployCtx.instanceStateSnapshot, changes)

	return &CollectedElements{
		Resources: resourcesToRemove,
		Children:  childrenToRemove,
		Links:     linksToRemove,
		Total:     len(resourcesToRemove) + len(childrenToRemove) + len(linksToRemove),
	}, false, nil
}

func (c *defaultBlueprintContainer) collectResourcesToRemove(
	currentState *state.InstanceState,
	changes *BlueprintChanges,
	nodesToBeDeployed []*DeploymentNode,
) ([]*ResourceIDInfo, error) {
	resourcesToRemove := []*ResourceIDInfo{}
	for _, resourceToRemove := range changes.RemovedResources {
		toBeRemovedResourceState := getResourceStateByName(currentState, resourceToRemove)
		if toBeRemovedResourceState != nil {
			// Check if the resource has dependents that will not be removed or recreated.
			// Resources that previously depended on the resource to be removed
			// and are marked to be recreated will no longer have a dependency on the resource
			// in question. This is because the same logic is applied during change staging
			// to mark a resource or child blueprint to be recreated if
			// it previously depended on a resource that is being removed.
			elements := filterOutRecreated(
				// For this purpose, there is no need to check transitive dependencies
				// as for a transitive dependency to exist, a direct dependency would also need to exist
				// and as soon as a direct dependency is found that will not be removed or recreated,
				// the deployment process will be stopped.
				findDependents(toBeRemovedResourceState, nodesToBeDeployed, currentState),
				changes,
			)
			if elements.Total > 0 {
				return nil, errResourceToBeRemovedHasDependents(resourceToRemove, elements)
			}
			resourcesToRemove = append(resourcesToRemove, &ResourceIDInfo{
				ResourceID:   toBeRemovedResourceState.ResourceID,
				ResourceName: toBeRemovedResourceState.ResourceName,
			})
		}
	}
	return resourcesToRemove, nil
}

func (c *defaultBlueprintContainer) collectChildrenToRemove(
	currentState *state.InstanceState,
	changes *BlueprintChanges,
	nodesToBeDeployed []*DeploymentNode,
) ([]*ChildBlueprintIDInfo, error) {
	childrenToRemove := []*ChildBlueprintIDInfo{}
	// Child blueprints that are marked to be recreated will need to be removed an addition
	// to those that have been removed from the source blueprint.
	combinedChildrenToRemove := append(changes.RemovedChildren, changes.RecreateChildren...)
	for _, childToRemove := range combinedChildrenToRemove {
		toBeRemovedChildState := getChildStateByName(currentState, childToRemove)
		if toBeRemovedChildState != nil {
			elements := filterOutRecreated(
				findDependents(
					state.WrapChildBlueprintInstance(childToRemove, toBeRemovedChildState),
					nodesToBeDeployed,
					currentState,
				),
				changes,
			)
			if elements.Total > 0 {
				return nil, errChildToBeRemovedHasDependents(childToRemove, elements)
			}
			childrenToRemove = append(childrenToRemove, &ChildBlueprintIDInfo{
				ChildInstanceID: toBeRemovedChildState.InstanceID,
				ChildName:       childToRemove,
			})
		}
	}
	return childrenToRemove, nil
}

func (c *defaultBlueprintContainer) collectLinksToRemove(
	currentState *state.InstanceState,
	changes *BlueprintChanges,
) []*LinkIDInfo {
	linksToRemove := []*LinkIDInfo{}
	for _, linkToRemove := range changes.RemovedLinks {
		toBeRemovedLinkState := getLinkStateByName(currentState, linkToRemove)
		if toBeRemovedLinkState != nil {
			linksToRemove = append(linksToRemove, &LinkIDInfo{
				LinkID:   toBeRemovedLinkState.LinkID,
				LinkName: linkToRemove,
			})
		}
	}
	return linksToRemove
}
