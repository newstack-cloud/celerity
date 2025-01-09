package container

import (
	"context"
	"encoding/json"
	"errors"
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
) error {
	instanceID, err := c.getInstanceID(input)
	if err != nil {
		return err
	}

	ctxWithInstanceID := context.WithValue(ctx, core.BlueprintInstanceIDKey, instanceID)
	state := c.createDeploymentState()

	isNewInstance, err := checkDeploymentForNewInstance(input)
	if err != nil {
		return err
	}

	go c.deploy(
		ctxWithInstanceID,
		&DeployInput{
			InstanceID: instanceID,
			Changes:    input.Changes,
			Rollback:   input.Rollback,
		},
		channels,
		state,
		isNewInstance,
		paramOverrides,
	)

	return nil
}

func (c *defaultBlueprintContainer) deploy(
	ctx context.Context,
	input *DeployInput,
	channels *DeployChannels,
	state DeploymentState,
	isNewInstance bool,
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
			determineInstanceDeployFailedStatus(input.Rollback, isNewInstance),
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
			determineInstanceDeployFailedStatus(input.Rollback, isNewInstance),
			[]string{prepareFailureMessage},
			c.clock.Since(startTime),
			/* prepareElapsedTime */ nil,
		)
		return
	}

	// Use the same behaviour as change staging to extract the nodes
	// that need to be deployed or updated where they are grouped for concurrent deployment
	// and in order based on links, references and use of the `dependsOn` property.
	prepareResult, err := c.blueprintPreparer.Prepare(
		ctx,
		c.spec.Schema(),
		subengine.ResolveForDeployment,
		input.Changes,
		c.linkInfo,
		paramOverrides,
	)
	if err != nil {
		channels.FinishChan <- c.createDeploymentFinishedMessage(
			input.InstanceID,
			determineInstanceDeployFailedStatus(input.Rollback, isNewInstance),
			[]string{prepareFailureMessage},
			c.clock.Since(startTime),
			/* prepareElapsedTime */ nil,
		)
		return
	}

	deployCtx := &DeployContext{
		StartTime:             startTime,
		State:                 state,
		Rollback:              input.Rollback,
		Destroying:            false,
		Channels:              channels,
		ParamOverrides:        paramOverrides,
		InstanceStateSnapshot: &currentInstanceState,
		ResourceProviders:     prepareResult.ResourceProviderMap,
		DeploymentGroups:      prepareResult.ParallelGroups,
	}

	flattenedNodes := core.Flatten(prepareResult.ParallelGroups)

	sentFinishedMessage, err := c.removeElements(
		ctx,
		input,
		deployCtx,
		flattenedNodes,
	)
	if err != nil {
		channels.ErrChan <- wrapErrorForChildContext(err, paramOverrides)
		return
	}
	if sentFinishedMessage {
		return
	}

	err = c.saveNewInstance(
		ctx,
		input.InstanceID,
		isNewInstance,
		determineInstanceDeployingStatus(input.Rollback, isNewInstance),
	)
	if err != nil {
		channels.ErrChan <- wrapErrorForChildContext(err, paramOverrides)
		return
	}

	sentFinishedMessage, err = c.deployElements(
		ctx,
		input,
		deployCtx,
		isNewInstance,
	)
	if err != nil {
		channels.ErrChan <- wrapErrorForChildContext(err, paramOverrides)
		return
	}
	if sentFinishedMessage {
		return
	}

	channels.FinishChan <- c.createDeploymentFinishedMessage(
		input.InstanceID,
		determineInstanceDeployedStatus(input.Rollback, isNewInstance),
		[]string{},
		c.clock.Since(startTime),
		/* prepareElapsedTime */ nil,
	)
}

func (c *defaultBlueprintContainer) getInstanceID(input *DeployInput) (string, error) {
	if input.InstanceID == "" {
		return c.idGenerator.GenerateID()
	}

	return input.InstanceID, nil
}

func (c *defaultBlueprintContainer) saveNewInstance(
	ctx context.Context,
	instanceID string,
	isNewInstance bool,
	currentStatus core.InstanceStatus,
) error {
	if !isNewInstance {
		return nil
	}

	return c.stateContainer.Instances().Save(
		ctx,
		state.InstanceState{
			InstanceID: instanceID,
			Status:     currentStatus,
		},
	)
}

func (c *defaultBlueprintContainer) deployElements(
	ctx context.Context,
	input *DeployInput,
	deployCtx *DeployContext,
	newInstance bool,
) (bool, error) {
	internalChannels := CreateDeployChannels()
	prepareElapsedTime := deployCtx.State.GetPrepareDuration()
	if len(deployCtx.DeploymentGroups) == 0 {
		deployCtx.Channels.FinishChan <- c.createDeploymentFinishedMessage(
			input.InstanceID,
			determineInstanceDeployedStatus(input.Rollback, newInstance),
			[]string{},
			c.clock.Since(deployCtx.StartTime),
			prepareElapsedTime,
		)
		return true, nil
	}

	c.startDeploymentFromFirstGroup(
		ctx,
		input.InstanceID,
		input.Changes,
		deployCtx,
	)

	stopProcessing, err := c.listenToAndProcessDeploymentEvents(
		ctx,
		input.InstanceID,
		deployCtx,
		input.Changes,
		internalChannels,
	)

	return stopProcessing, err

	// Deploy the first group of elements concurrently.

	// Unlike with change staging, groups are not executed as a unit, they are used as
	// pools to look for components that can be deployed based on the current state of deployment.
	// For each component to be created or updated (including recreated children):
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

func (c *defaultBlueprintContainer) startDeploymentFromFirstGroup(
	ctx context.Context,
	instanceID string,
	changes *BlueprintChanges,
	deployCtx *DeployContext,
) {
	instanceTreePath := getInstanceTreePath(deployCtx.ParamOverrides, instanceID)

	for _, node := range deployCtx.DeploymentGroups[0] {
		if node.Type() == "resource" {
			go c.resourceDeployer.Deploy(
				ctx,
				instanceID,
				node.ChainLinkNode,
				changes,
				deployCtx,
			)
		} else if node.Type() == "child" {
			includeTreePath := getIncludeTreePath(deployCtx.ParamOverrides, node.Name())
			go c.childDeployer.Deploy(
				ctx,
				instanceID,
				instanceTreePath,
				includeTreePath,
				node.ChildNode,
				changes,
				deployCtx,
			)
		}
	}
}

func (c *defaultBlueprintContainer) listenToAndProcessDeploymentEvents(
	ctx context.Context,
	instanceID string,
	deployCtx *DeployContext,
	changes *BlueprintChanges,
	internalChannels *DeployChannels,
) (bool, error) {
	finished := map[string]*deployUpdateMessageWrapper{}
	// For this to work, the blueprint changes provided must match
	// the loaded blueprint.
	// The count must reflect the number of elements that will be deployed
	// taking resources, links and child blueprints into account.
	elementsToDeploy := countElementsToDeploy(changes)

	var err error
	for (len(finished) < elementsToDeploy) &&
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

	failed := getFailedElementDeploymentsAndUpdateState(finished, changes, deployCtx)
	if len(failed) > 0 {
		deployCtx.Channels.FinishChan <- c.createDeploymentFinishedMessage(
			instanceID,
			determineFinishedFailureStatus(
				/* destroyingInstance */ false,
				deployCtx.Rollback,
			),
			finishedFailureMessages(deployCtx, failed),
			c.clock.Since(deployCtx.StartTime),
			/* prepareElapsedTime */
			deployCtx.State.GetPrepareDuration(),
		)
		return true, nil
	}

	// TODO: Implement
	// On config complete event for resources, determine
	// the next element to deploy based on references or links,
	// determine if the next element requires the resource to be stable,
	// if so, in a separate goroutine, wait for the resource to be stable
	// and continue to process when is stable (also dispatch status update when stable).
	// If the next element does not require the resource to be stable, deploy the next element
	// in a separate goroutine.
	// Similarly to change staging, keep track of links that are pending completion
	// and deploy them when the resources they depend on are ready.
	// Should links require resources to be stable? Should this be a part of the provider
	// interface for links?

	return false, nil
}

func (c *defaultBlueprintContainer) handleResourceUpdateMessage(
	ctx context.Context,
	instanceID string,
	msg ResourceDeployUpdateMessage,
	deployCtx *DeployContext,
	finished map[string]*deployUpdateMessageWrapper,
) error {
	if msg.InstanceID != instanceID {
		// If message is for a child blueprint, pass through to the client
		// to ensure updates within the child blueprint are surfaced.
		// This allows for the client to provide more detailed feedback to the user
		// for the progress within a child blueprint.
		deployCtx.Channels.ResourceUpdateChan <- msg
		return nil
	}

	elementName := core.ResourceElementID(msg.ResourceName)

	if isResourceDestroyEvent(msg.PreciseStatus, deployCtx.Rollback) {
		return c.handleResourceDestroyEvent(ctx, msg, deployCtx, finished, elementName)
	}

	if isResourceUpdateEvent(msg.PreciseStatus, deployCtx.Rollback) {
		return c.handleResourceUpdateEvent(ctx, msg, deployCtx, finished, elementName)
	}

	if isResourceCreationEvent(msg.PreciseStatus, deployCtx.Rollback) {
		return c.handleResourceCreationEvent(ctx, msg, deployCtx, finished, elementName)
	}

	return nil
}

func (c *defaultBlueprintContainer) handleResourceUpdateEvent(
	ctx context.Context,
	msg ResourceDeployUpdateMessage,
	deployCtx *DeployContext,
	finished map[string]*deployUpdateMessageWrapper,
	elementName string,
) error {
	return nil
}

func (c *defaultBlueprintContainer) handleResourceCreationEvent(
	ctx context.Context,
	msg ResourceDeployUpdateMessage,
	deployCtx *DeployContext,
	finished map[string]*deployUpdateMessageWrapper,
	elementName string,
) error {
	return nil
}

func (c *defaultBlueprintContainer) handleChildUpdateMessage(
	ctx context.Context,
	instanceID string,
	msg ChildDeployUpdateMessage,
	deployCtx *DeployContext,
	finished map[string]*deployUpdateMessageWrapper,
) error {
	if msg.ParentInstanceID != instanceID {
		// If message is for a child blueprint, pass through to the client
		// to ensure updates within the child blueprint are surfaced.
		// This allows for the client to provide more detailed feedback to the user
		// for the progress within a child blueprint.
		deployCtx.Channels.ChildUpdateChan <- msg
		return nil
	}

	elementName := core.ChildElementID(msg.ChildName)

	if isChildDestroyEvent(msg.Status, deployCtx.Rollback) {
		return c.handleChildDestroyEvent(ctx, msg, deployCtx, finished, elementName)
	}

	if isChildUpdateEvent(msg.Status, deployCtx.Rollback) {
		return c.handleChildUpdateEvent(ctx, msg, deployCtx, finished, elementName)
	}

	if isChildDeployEvent(msg.Status, deployCtx.Rollback) {
		return c.handleChildDeployEvent(ctx, msg, deployCtx, finished, elementName)
	}

	return nil
}

func (c *defaultBlueprintContainer) handleChildUpdateEvent(
	ctx context.Context,
	msg ChildDeployUpdateMessage,
	deployCtx *DeployContext,
	finished map[string]*deployUpdateMessageWrapper,
	elementName string,
) error {
	return nil
}

func (c *defaultBlueprintContainer) handleChildDeployEvent(
	ctx context.Context,
	msg ChildDeployUpdateMessage,
	deployCtx *DeployContext,
	finished map[string]*deployUpdateMessageWrapper,
	elementName string,
) error {
	return nil
}

func (c *defaultBlueprintContainer) handleLinkUpdateMessage(
	ctx context.Context,
	instanceID string,
	msg LinkDeployUpdateMessage,
	deployCtx *DeployContext,
	finished map[string]*deployUpdateMessageWrapper,
) error {
	if msg.InstanceID != instanceID {
		// If message is for a child blueprint, pass through to the client
		// to ensure updates within the child blueprint are surfaced.
		// This allows for the client to provide more detailed feedback to the user
		// for the progress within a child blueprint.
		deployCtx.Channels.LinkUpdateChan <- msg
		return nil
	}

	elementName := linkElementID(msg.LinkName)

	if isLinkDestroyEvent(msg.Status, deployCtx.Rollback) {
		return c.handleLinkDestroyEvent(ctx, msg, deployCtx, finished, elementName)
	}

	if isLinkUpdateEvent(msg.Status, deployCtx.Rollback) {
		return c.handleLinkUpdateEvent(ctx, msg, deployCtx, finished, elementName)
	}

	if isLinkCreationEvent(msg.Status, deployCtx.Rollback) {
		return c.handleLinkCreationEvent(ctx, msg, deployCtx, finished, elementName)
	}

	return nil
}

func (c *defaultBlueprintContainer) handleLinkUpdateEvent(
	ctx context.Context,
	msg LinkDeployUpdateMessage,
	deployCtx *DeployContext,
	finished map[string]*deployUpdateMessageWrapper,
	elementName string,
) error {
	return nil
}

func (c *defaultBlueprintContainer) handleLinkCreationEvent(
	ctx context.Context,
	msg LinkDeployUpdateMessage,
	deployCtx *DeployContext,
	finished map[string]*deployUpdateMessageWrapper,
	elementName string,
) error {
	return nil
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

type resourceDeployInfo struct {
	instanceID   string
	resourceID   string
	resourceName string
	resourceImpl provider.Resource
	changes      *provider.Changes
	isNew        bool
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
	// A group is a collection of items that can be deployed or destroyed at the same time.
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

// DeployEvent contains an event that is emitted during the deployment process.
// This is used like a sum type to represent the different types of events that can be emitted.
type DeployEvent struct {
	// ResourceUpdateEvent is an event that is emitted when a resource is updated.
	ResourceUpdateEvent *ResourceDeployUpdateMessage
	// LinkUpdateEvent is an event that is emitted when a link is updated.
	LinkUpdateEvent *LinkDeployUpdateMessage
	// ChildUpdateEvent is an event that is emitted when a child blueprint is updated.
	ChildUpdateEvent *ChildDeployUpdateMessage
	// DeploymentUpdateEvent is an event that is emitted when the
	// deployment status of the blueprint instance is updated.
	DeploymentUpdateEvent *DeploymentUpdateMessage
	// FinishEvent is an event that is emitted when the deployment
	// of the blueprint instance has finished.
	FinishEvent *DeploymentFinishedMessage
}

type intermediaryDeployEvent struct {
	EventType EventType       `json:"type"`
	Message   json.RawMessage `json:"message"`
}

// EventType is a type that represents the different types of events that can be
// emitted during the deployment process.
type EventType string

const (
	// EventTypeResourceUpdate is an event type that represents a resource update event.
	EventTypeResourceUpdate EventType = "resourceUpdate"
	// EventTypeLinkUpdate is an event type that represents a link update event.
	EventTypeLinkUpdate EventType = "linkUpdate"
	// EventTypeChildUpdate is an event type that represents a child blueprint update event.
	EventTypeChildUpdate EventType = "childUpdate"
	// EventTypeDeploymentUpdate is an event type that represents a
	// blueprint instance deployment update event.
	EventTypeDeploymentUpdate EventType = "deploymentUpdate"
	// EventTypeFinish is an event type that represents a
	// blueprint instance deployment finish event.
	EventTypeFinish EventType = "finish"
)

func (e *DeployEvent) MarshalJSON() ([]byte, error) {
	if e.ResourceUpdateEvent != nil {
		return e.marshalEventMessage(
			EventTypeResourceUpdate,
			e.ResourceUpdateEvent,
		)
	}

	if e.LinkUpdateEvent != nil {
		return e.marshalEventMessage(
			EventTypeLinkUpdate,
			e.LinkUpdateEvent,
		)
	}

	if e.ChildUpdateEvent != nil {
		return e.marshalEventMessage(
			EventTypeChildUpdate,
			e.ChildUpdateEvent,
		)
	}

	if e.DeploymentUpdateEvent != nil {
		return e.marshalEventMessage(
			EventTypeDeploymentUpdate,
			e.DeploymentUpdateEvent,
		)
	}

	if e.FinishEvent != nil {
		return e.marshalEventMessage(
			EventTypeFinish,
			e.FinishEvent,
		)
	}

	return nil, errors.New("no event message set")
}

func (e *DeployEvent) marshalEventMessage(
	eventType EventType,
	eventMessage any,
) ([]byte, error) {
	msgBytes, err := json.Marshal(eventMessage)
	if err != nil {
		return nil, err
	}

	return json.Marshal(intermediaryDeployEvent{
		EventType: eventType,
		Message:   msgBytes,
	})
}

func (e *DeployEvent) UnmarshalJSON(data []byte) error {
	intermediaryEvent := &intermediaryDeployEvent{}
	err := json.Unmarshal(data, intermediaryEvent)
	if err != nil {
		return err
	}

	switch intermediaryEvent.EventType {
	case EventTypeResourceUpdate:
		e.ResourceUpdateEvent = &ResourceDeployUpdateMessage{}
		return json.Unmarshal(intermediaryEvent.Message, e.ResourceUpdateEvent)
	case EventTypeLinkUpdate:
		e.LinkUpdateEvent = &LinkDeployUpdateMessage{}
		return json.Unmarshal(intermediaryEvent.Message, e.LinkUpdateEvent)
	case EventTypeChildUpdate:
		e.ChildUpdateEvent = &ChildDeployUpdateMessage{}
		return json.Unmarshal(intermediaryEvent.Message, e.ChildUpdateEvent)
	case EventTypeDeploymentUpdate:
		e.DeploymentUpdateEvent = &DeploymentUpdateMessage{}
		return json.Unmarshal(intermediaryEvent.Message, e.DeploymentUpdateEvent)
	case EventTypeFinish:
		e.FinishEvent = &DeploymentFinishedMessage{}
		return json.Unmarshal(intermediaryEvent.Message, e.FinishEvent)
	}

	return errors.New("no valid event type set")
}
