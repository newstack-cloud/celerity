package container

import (
	"context"
	"time"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

func (c *defaultBlueprintContainer) Destroy(
	ctx context.Context,
	input *DestroyInput,
	channels *DestroyChannels,
	paramOverrides core.BlueprintParams,
) {
	// todo: implement
}

func (c *defaultBlueprintContainer) removeElements(
	ctx context.Context,
	input *DeployInput,
	deployCtx *deployContext,
	nodesToBeDeployed []*DeploymentNode,
) (bool, error) {

	elementsToRemove, finished, err := c.collectElementsToRemove(
		deployCtx.instanceStateSnapshot,
		input.Changes,
		deployCtx.startTime,
		nodesToBeDeployed,
		deployCtx.channels,
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

	deployCtx.channels.DeploymentUpdateChan <- DeploymentUpdateMessage{
		InstanceID: input.InstanceID,
		Status:     core.InstanceStatusDeploying,
	}

	go c.removeGroupedElements(
		ctx,
		groupedElements,
		input.InstanceID,
		deployCtx,
	)

	return false, nil
}

func (c *defaultBlueprintContainer) removeGroupedElements(
	ctx context.Context,
	parallelGroups [][]state.Element,
	instanceID string,
	deployCtx *deployContext,
) {

	resourceUpdateChan := make(chan ResourceDeployUpdateMessage)
	childUpdateChan := make(chan ChildDeployUpdateMessage)
	linkUpdateChan := make(chan LinkDeployUpdateMessage)
	errChan := make(chan error)

	internalChannels := &DeployChannels{
		ResourceUpdateChan: resourceUpdateChan,
		ChildUpdateChan:    childUpdateChan,
		LinkUpdateChan:     linkUpdateChan,
		ErrChan:            errChan,
	}

	for groupIndex, group := range parallelGroups {
		c.stageGroupRemovals(
			ctx,
			instanceID,
			group,
			deployContextWithGroup(
				deployContextWithChannels(deployCtx, internalChannels),
				groupIndex,
			),
		)

		err := c.listenToAndProcessGroupRemovals(
			ctx,
			instanceID,
			group,
			deployCtx,
			internalChannels,
		)
		if err != nil {
			deployCtx.channels.ErrChan <- wrapErrorForChildContext(
				err,
				deployCtx.paramOverrides,
			)
			return
		}
	}
}

func (c *defaultBlueprintContainer) listenToAndProcessGroupRemovals(
	ctx context.Context,
	instanceID string,
	group []state.Element,
	deployCtx *deployContext,
	internalChannels *DeployChannels,
) error {
	finished := map[string]*deployUpdateMessageWrapper{}
	// todo: figure out best way to surface detailed child blueprint updates
	// Let events from child blueprint pass through to the external channels
	// to ensure they are surfaced to the client, but do not block on them,
	// the container for the child blueprint will be responsible for orchestrating
	// the removal of elements in the child blueprint.
	var err error
	for (len(finished) < len(group)) &&
		err == nil {
		select {
		case msg := <-internalChannels.ResourceUpdateChan:
			c.handleResourceUpdateMessage(ctx, instanceID, msg, deployCtx, finished)
		case err = <-internalChannels.ErrChan:
		}
	}

	// If any have failed and current context is not rolling back,
	// roll back any removals that have succeeded.

	return err
}

func (c *defaultBlueprintContainer) handleResourceUpdateMessage(
	ctx context.Context,
	instanceID string,
	msg ResourceDeployUpdateMessage,
	deployCtx *deployContext,
	finished map[string]*deployUpdateMessageWrapper,
) {
	if msg.InstanceID != instanceID {
		// If message is for a child blueprint, pass through to the client
		// to ensure updates within the child blueprint are surfaced.
		// This allows for the client to provide more detailed feedback to the user
		// for the progress within a child blueprint.
		deployCtx.channels.ResourceUpdateChan <- msg
		return
	}

	// elementName := core.ResourceElementID(msg.ResourceName)

	// if startedDestroying(msg, deployCtx.rollback) {
	// 	prevResourceState := getResourceStateByName(
	// 		deployCtx.instanceStateSnapshot,
	// 		msg.ResourceName,
	// 	)
	// 	stashPreviousResourceState(msg.ResourceName, prevResourceState, deployCtx.state)
	// 	c.stateContainer.UpdateResourceStatus(ctx)
	// }

	// if finishedDestroyingResource(msg, deployCtx.rollback) {
	// 	finished[elementName] = &deployUpdateMessageWrapper{
	// 		resourceUpdateMessage: &msg,
	// 	}
	// }

	// if wasResourceDestroySuccessful(msg, deployCtx.rollback) {
	// 	resourceState, err := c.stateContainer.RemoveResource(
	// 		ctx,
	// 		msg.InstanceID,
	// 		msg.ResourceID,
	// 	)
	// }
	deployCtx.channels.ResourceUpdateChan <- msg
}

func (c *defaultBlueprintContainer) stageGroupRemovals(
	ctx context.Context,
	instanceID string,
	group []state.Element,
	deployCtx *deployContext,
) {
	instanceTreePath := getInstanceTreePath(deployCtx.paramOverrides, instanceID)

	for _, element := range group {
		if element.Type() == state.ResourceElement {
			go c.prepareAndDestroyResource(
				ctx,
				element,
				instanceID,
				instanceTreePath,
				deployCtx,
			)
		} else if element.Type() == state.ChildElement {
			go c.destroyChild(
				ctx,
				instanceID,
				instanceTreePath,
				deployCtx,
			)
		} else if element.Type() == state.LinkElement {
			go c.destroyLink(
				ctx,
				instanceID,
				instanceTreePath,
				deployCtx,
			)
		}
	}
}

func (c *defaultBlueprintContainer) prepareAndDestroyResource(
	ctx context.Context,
	resourceElement state.Element,
	instanceID string,
	instanceTreePath string,
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
		&retryInfo{
			// Start at 0 for first attempt as retries are counted from 1.
			attempt:            0,
			attemptDurations:   []float64{},
			exceededMaxRetries: false,
			policy:             policy,
		},
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

	err := resourceImplementation.Destroy(ctx, &provider.ResourceDestroyInput{
		InstanceID: resourceInfo.instanceID,
		ResourceID: resourceInfo.element.ID(),
		Params:     deployCtx.paramOverrides,
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

func (c *defaultBlueprintContainer) destroyChild(
	ctx context.Context,
	instanceID string,
	instanceTreePath string,
	deployCtx *deployContext,
) {
}

func (c *defaultBlueprintContainer) destroyLink(
	ctx context.Context,
	instanceID string,
	instanceTreePath string,
	deployCtx *deployContext,
) {
}

func (c *defaultBlueprintContainer) collectElementsToRemove(
	currentInstanceState *state.InstanceState,
	changes *BlueprintChanges,
	startTime time.Time,
	nodesToBeDeployed []*DeploymentNode,
	channels *DeployChannels,
) (*CollectedElements, bool, error) {
	if len(changes.RemovedChildren) == 0 &&
		len(changes.RemovedResources) == 0 &&
		len(changes.RemovedLinks) == 0 {
		return &CollectedElements{}, false, nil
	}

	resourcesToRemove, err := c.collectResourcesToRemove(currentInstanceState, changes, nodesToBeDeployed)
	if err != nil {
		message := getDeploymentErrorSpecificMessage(err, prepareFailureMessage)
		channels.FinishChan <- c.createDeploymentFinishedMessage(
			currentInstanceState.InstanceID,
			core.InstanceStatusDeployFailed,
			[]string{message},
			c.clock.Since(startTime),
		)
		return &CollectedElements{}, true, nil
	}

	childrenToRemove, err := c.collectChildrenToRemove(currentInstanceState, changes, nodesToBeDeployed)
	if err != nil {
		message := getDeploymentErrorSpecificMessage(err, prepareFailureMessage)
		channels.FinishChan <- c.createDeploymentFinishedMessage(
			currentInstanceState.InstanceID,
			core.InstanceStatusDeployFailed,
			[]string{message},
			c.clock.Since(startTime),
		)
		return &CollectedElements{}, true, nil
	}

	linksToRemove := c.collectLinksToRemove(currentInstanceState, changes)

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
