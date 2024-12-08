package container

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/links"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	"github.com/two-hundred/celerity/libs/blueprint/validation"
)

func (c *defaultBlueprintContainer) StageChanges(
	ctx context.Context,
	input *StageChangesInput,
	channels *ChangeStagingChannels,
	paramOverrides core.BlueprintParams,
) error {

	instanceTreePath := getInstanceTreePath(paramOverrides, input.InstanceID)
	if exceedsMaxDepth(instanceTreePath, MaxBlueprintDepth) {
		return errMaxBlueprintDepthExceeded(
			instanceTreePath,
			MaxBlueprintDepth,
		)
	}

	if input.Destroy {
		go c.stageInstanceRemoval(ctx, input.InstanceID, channels)
		return nil
	}

	expandedBlueprintContainer, err := c.expandResourceTemplates(
		ctx,
		paramOverrides,
	)
	if err != nil {
		return wrapErrorForChildContext(err, paramOverrides)
	}

	chains, err := expandedBlueprintContainer.SpecLinkInfo().Links(ctx)
	if err != nil {
		return wrapErrorForChildContext(err, paramOverrides)
	}

	// We must use the ref chain collector from the expanded blueprint to correctly
	// order and resolve references for resources expanded from templates
	// in the blueprint.
	refChainCollector := expandedBlueprintContainer.RefChainCollector()

	childrenRefNodes := extractChildRefNodes(expandedBlueprintContainer.BlueprintSpec().Schema(), refChainCollector)
	orderedNodes, err := OrderItemsForDeployment(ctx, chains, childrenRefNodes, refChainCollector, paramOverrides)
	if err != nil {
		return wrapErrorForChildContext(err, paramOverrides)
	}
	parallelGroups, err := GroupOrderedNodes(orderedNodes, refChainCollector)
	if err != nil {
		return wrapErrorForChildContext(err, paramOverrides)
	}

	expandedResourceProviderMap := createResourceProviderMap(
		expandedBlueprintContainer.BlueprintSpec(),
		c.providers,
	)

	go c.stageChanges(
		ctx,
		input.InstanceID,
		parallelGroups,
		paramOverrides,
		expandedResourceProviderMap,
		expandedBlueprintContainer.BlueprintSpec().Schema(),
		channels,
	)

	return nil
}

func (c *defaultBlueprintContainer) stageChanges(
	ctx context.Context,
	instanceID string,
	parallelGroups [][]*DeploymentNode,
	paramOverrides core.BlueprintParams,
	resourceProviders map[string]provider.Provider,
	blueprint *schema.Blueprint,
	channels *ChangeStagingChannels,
) {
	state := &stageChangesState{
		pendingLinks:        map[string]*linkPendingCompletion{},
		resourceNameLinkMap: map[string][]string{},
		outputChanges:       &intermediaryBlueprintChanges{},
		mustRecreate: &collectedElements{
			resources: []*resourceIDInfo{},
			children:  []*childBlueprintIDInfo{},
			total:     0,
		},
	}
	resourceChangesChan := make(chan ResourceChangesMessage)
	childChangesChan := make(chan ChildChangesMessage)
	linkChangesChan := make(chan LinkChangesMessage)
	errChan := make(chan error)

	internalChannels := &ChangeStagingChannels{
		ResourceChangesChan: resourceChangesChan,
		ChildChangesChan:    childChangesChan,
		LinkChangesChan:     linkChangesChan,
		ErrChan:             errChan,
	}

	// Check for all the removed resources, links and child blueprints.
	// All removals will be handled before the groups of new elements and element
	// updates are staged.
	// Staging state changes will be applied synchronously for all resources, links and child blueprints
	// that have been removed in the source blueprint being staged for deployment.
	// A message is dispatched to the external channels for each removal so that the caller
	// can gather and display removals in the same way as other changes.
	err := c.stageRemovals(ctx, instanceID, state, parallelGroups, channels)
	if err != nil {
		channels.ErrChan <- wrapErrorForChildContext(err, paramOverrides)
		return
	}

	for _, group := range parallelGroups {
		c.stageGroupChanges(
			ctx,
			instanceID,
			state,
			group,
			paramOverrides,
			resourceProviders,
			internalChannels,
		)

		err := c.listenToAndProcessGroupChanges(
			group,
			internalChannels,
			channels,
			state,
		)
		if err != nil {
			channels.ErrChan <- wrapErrorForChildContext(err, paramOverrides)
			return
		}
	}

	err = c.resolveAndCollectExportChanges(ctx, instanceID, blueprint, state)
	if err != nil {
		channels.ErrChan <- wrapErrorForChildContext(err, paramOverrides)
		return
	}

	// Get children that must be recreated due to removed dependencies and remove
	// from child changes if present in child changes map.
	recreateChildren := updateChildChangesAndCollectChildrenToRecreate(state)

	channels.CompleteChan <- BlueprintChanges{
		NewResources:     copyPointerMap(state.outputChanges.NewResources),
		ResourceChanges:  copyPointerMap(state.outputChanges.ResourceChanges),
		RemovedResources: state.outputChanges.RemovedResources,
		RemovedLinks:     state.outputChanges.RemovedLinks,
		NewChildren:      copyPointerMap(state.outputChanges.NewChildren),
		RecreateChildren: recreateChildren,
		ChildChanges:     copyPointerMap(state.outputChanges.ChildChanges),
		RemovedChildren:  state.outputChanges.RemovedChildren,
		NewExports:       copyPointerMap(state.outputChanges.NewExports),
		ExportChanges:    copyPointerMap(state.outputChanges.ExportChanges),
		RemovedExports:   state.outputChanges.RemovedExports,
		ResolveOnDeploy:  state.outputChanges.ResolveOnDeploy,
	}
}

func updateChildChangesAndCollectChildrenToRecreate(state *stageChangesState) []string {
	state.mu.Lock()
	defer state.mu.Unlock()

	recreateChildren := []string{}
	for _, child := range state.mustRecreate.children {
		if state.outputChanges.ChildChanges[child.childName] != nil {
			delete(state.outputChanges.ChildChanges, child.childName)
			recreateChildren = append(recreateChildren, child.childName)
		}
	}
	return recreateChildren
}

func (c *defaultBlueprintContainer) listenToAndProcessGroupChanges(
	group []*DeploymentNode,
	internalChannels *ChangeStagingChannels,
	externalChannels *ChangeStagingChannels,
	state *stageChangesState,
) error {
	// The criteria to move on to the next group is the following:
	// - All resources in the group current have been processed.
	// - All child blueprints in the current group have been processed.
	// - All links that were previously pending completion and waiting on the
	//    resources in the current group have been processed.
	expectedLinkChangesCount := countPendingLinksForGroupFromState(group, state) +
		// We need to account for soft links where two resources that are linked can be deployed
		// at the same time.
		countPendingLinksContainedInGroup(group)

	linkChangesCount := 0
	collected := map[string]*changesWrapper{}
	var err error
	waitingForLinkChanges := expectedLinkChangesCount > 0
	for (len(collected) < len(group) || waitingForLinkChanges) &&
		err == nil {
		select {
		case changes := <-internalChannels.ResourceChangesChan:
			elementName := core.ResourceElementID(changes.ResourceName)
			collected[elementName] = &changesWrapper{
				resourceChanges: &changes.Changes,
			}
			externalChannels.ResourceChangesChan <- changes
		case changes := <-internalChannels.LinkChangesChan:
			applyLinkChangesToState(changes, state)
			linkChangesCount += 1
			externalChannels.LinkChangesChan <- changes
		case changes := <-internalChannels.ChildChangesChan:
			elementName := core.ChildElementID(changes.ChildBlueprintName)
			collected[elementName] = &changesWrapper{
				childChanges: &changes.Changes,
			}
			applyChildChangesToState(changes, state)
			externalChannels.ChildChangesChan <- changes
		case err = <-internalChannels.ErrChan:
		}

		waitingForLinkChanges = expectedLinkChangesCount > 0 && linkChangesCount < expectedLinkChangesCount
	}

	return err
}

func countPendingLinksForGroupFromState(group []*DeploymentNode, state *stageChangesState) int {
	state.mu.Lock()
	defer state.mu.Unlock()

	count := 0
	for _, node := range group {
		if node.Type() == "resource" {
			pendingLinkNames := state.resourceNameLinkMap[node.ChainLinkNode.ResourceName]
			for _, linkName := range pendingLinkNames {
				if state.pendingLinks[linkName].linkPending {
					count += 1
				}
			}
		}
	}

	return count
}

func countPendingLinksContainedInGroup(group []*DeploymentNode) int {
	count := 0
	for _, node := range group {
		if node.Type() == "resource" {
			for _, otherNode := range node.ChainLinkNode.LinksTo {
				if groupContainsResourceLinkNode(group, otherNode) {
					count += 1
				}
			}
		}
	}

	return count
}

func groupContainsResourceLinkNode(group []*DeploymentNode, resourceLinkNode *links.ChainLinkNode) bool {
	return slices.ContainsFunc(group, func(compareWith *DeploymentNode) bool {
		return compareWith.Type() == "resource" &&
			compareWith.ChainLinkNode.ResourceName == resourceLinkNode.ResourceName
	})
}

func applyResourceChangesToState(changes ResourceChangesMessage, state *stageChangesState) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if changes.New {
		if state.outputChanges.NewResources == nil {
			state.outputChanges.NewResources = map[string]*provider.Changes{}
		}
		state.outputChanges.NewResources[changes.ResourceName] = &changes.Changes
	} else if changes.Removed {
		if state.outputChanges.RemovedResources == nil {
			state.outputChanges.RemovedResources = []string{}
		}
		state.outputChanges.RemovedResources = append(
			state.outputChanges.RemovedResources,
			changes.ResourceName,
		)
	} else {
		if state.outputChanges.ResourceChanges == nil {
			state.outputChanges.ResourceChanges = map[string]*provider.Changes{}
		}
		state.outputChanges.ResourceChanges[changes.ResourceName] = &changes.Changes
	}
}

func addElementsThatMustBeRecreatedToState(dependents *collectedElements, state *stageChangesState) {
	state.mu.Lock()
	defer state.mu.Unlock()

	for _, resource := range dependents.resources {
		if !collectedElementsHasResource(state.mustRecreate, resource) {
			state.mustRecreate.resources = append(state.mustRecreate.resources, resource)
			state.mustRecreate.total += 1
		}
	}

	for _, child := range dependents.children {
		if !collectedElementsHasChild(state.mustRecreate, child) {
			state.mustRecreate.children = append(state.mustRecreate.children, child)
			state.mustRecreate.total += 1
		}
	}
}

func applyLinkChangesToState(changes LinkChangesMessage, state *stageChangesState) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if changes.Removed {
		state.outputChanges.RemovedLinks = append(
			state.outputChanges.RemovedLinks,
			createLogicalLinkName(changes.ResourceAName, changes.ResourceBName),
		)
		return
	}

	resourceChanges := getResourceChanges(changes.ResourceAName, state.outputChanges)
	if resourceChanges != nil {
		if changes.New {
			if resourceChanges.NewOutboundLinks == nil {
				resourceChanges.NewOutboundLinks = map[string]provider.LinkChanges{}
			}
			resourceChanges.NewOutboundLinks[changes.ResourceBName] = changes.Changes
		} else {
			if resourceChanges.OutboundLinkChanges == nil {
				resourceChanges.OutboundLinkChanges = map[string]provider.LinkChanges{}
			}
			resourceChanges.OutboundLinkChanges[changes.ResourceBName] = changes.Changes
		}
	}
}

func applyChildChangesToState(changes ChildChangesMessage, state *stageChangesState) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if changes.New {
		if state.outputChanges.NewChildren == nil {
			state.outputChanges.NewChildren = map[string]*NewBlueprintDefinition{}
		}

		state.outputChanges.NewChildren[changes.ChildBlueprintName] = &NewBlueprintDefinition{
			NewResources: changes.Changes.NewResources,
			NewChildren:  changes.Changes.NewChildren,
			NewExports:   changes.Changes.NewExports,
		}
	} else if changes.Removed {
		state.outputChanges.RemovedChildren = append(
			state.outputChanges.RemovedChildren,
			changes.ChildBlueprintName,
		)
	} else {
		if state.outputChanges.ChildChanges == nil {
			state.outputChanges.ChildChanges = map[string]*BlueprintChanges{}
		}
		state.outputChanges.ChildChanges[changes.ChildBlueprintName] = &changes.Changes
	}
}

// A lock must be held on the staging state when calling this function.
func getResourceChanges(resourceName string, changes *intermediaryBlueprintChanges) *provider.Changes {

	newResourceChanges, hasNewResourceChanges := changes.NewResources[resourceName]
	if hasNewResourceChanges {
		return newResourceChanges
	}

	resourceChanges, hasResourceChanges := changes.ResourceChanges[resourceName]
	if hasResourceChanges {
		return resourceChanges
	}

	return nil
}

func (c *defaultBlueprintContainer) stageGroupChanges(
	ctx context.Context,
	instanceID string,
	stagingState *stageChangesState,
	group []*DeploymentNode,
	paramOverrides core.BlueprintParams,
	resourceProviders map[string]provider.Provider,
	channels *ChangeStagingChannels,
) {
	instanceTreePath := getInstanceTreePath(paramOverrides, instanceID)

	for _, node := range group {
		if node.Type() == "resource" {
			go c.prepareAndStageResourceChanges(
				ctx,
				instanceID,
				stagingState,
				node.ChainLinkNode,
				channels,
				resourceProviders,
				paramOverrides,
			)
		} else if node.Type() == "child" {
			includeTreePath := getIncludeTreePath(paramOverrides, node.Name())
			go c.stageChildBlueprintChanges(
				ctx,
				instanceID,
				instanceTreePath,
				includeTreePath,
				node.ChildNode,
				paramOverrides,
				channels,
			)
		}
	}
}

func (c *defaultBlueprintContainer) prepareAndStageResourceChanges(
	ctx context.Context,
	instanceID string,
	stagingState *stageChangesState,
	node *links.ChainLinkNode,
	channels *ChangeStagingChannels,
	resourceProviders map[string]provider.Provider,
	params core.BlueprintParams,
) {
	resourceProvider, hasResourceProvider := resourceProviders[node.ResourceName]
	if !hasResourceProvider {
		channels.ErrChan <- fmt.Errorf("no provider found for resource %q", node.ResourceName)
		return
	}

	resourceImplementation, err := resourceProvider.Resource(ctx, node.Resource.Type.Value)
	if err != nil {
		channels.ErrChan <- err
		return
	}

	err = c.stageResourceChanges(
		ctx,
		&stageResourceChangeInfo{
			node:       node,
			instanceID: instanceID,
		},
		resourceImplementation,
		channels.ResourceChangesChan,
		channels.LinkChangesChan,
		stagingState,
		params,
	)
	if err != nil {
		channels.ErrChan <- err
		return
	}
}

func (c *defaultBlueprintContainer) stageResourceChanges(
	ctx context.Context,
	stageResourceInfo *stageResourceChangeInfo,
	resourceImplementation provider.Resource,
	changesChan chan ResourceChangesMessage,
	linkChangesChan chan LinkChangesMessage,
	stagingState *stageChangesState,
	params core.BlueprintParams,
) error {

	resourceInfo, resolveResourceResult, err := c.getResourceInfo(ctx, stageResourceInfo)
	if err != nil {
		return err
	}

	changes, err := c.changeStager.StageChanges(
		ctx,
		resourceInfo,
		resourceImplementation,
		resolveResourceResult.ResolveOnDeploy,
		params,
	)
	if err != nil {
		return err
	}

	// The resource must be recreated if an element that it previously depended on
	// has been removed.
	if !changes.MustRecreate {
		changes.MustRecreate, err = mustRecreateResourceOnRemovedDependencies(
			resourceInfo,
			stagingState,
		)
		if err != nil {
			return err
		}
	}

	changesMsg := ResourceChangesMessage{
		ResourceName:    stageResourceInfo.node.ResourceName,
		Changes:         *changes,
		Removed:         false,
		New:             resourceInfo.CurrentResourceState == nil,
		ResolveOnDeploy: resolveResourceResult.ResolveOnDeploy,
		ConditionKnownOnDeploy: isConditionKnownOnDeploy(
			stageResourceInfo.node.ResourceName,
			resolveResourceResult.ResolveOnDeploy,
		),
	}
	changesChan <- changesMsg

	// We must make sure that resource changes are applied to the internal changing state
	// before we can stage links that are dependent on the resource changes.
	// Otherwise, we can end up with inconsistent state where links are staged before the
	// resource changes are applied, leading to incorrect link changes being reported.
	applyResourceChangesToState(changesMsg, stagingState)
	linksReadyToBeStaged := c.updateStagingState(stageResourceInfo.node, stagingState)

	err = c.prepareAndStageLinkChanges(
		ctx,
		resourceInfo,
		linksReadyToBeStaged,
		linkChangesChan,
		stagingState,
		params,
	)
	if err != nil {
		return err
	}

	return nil
}

func mustRecreateResourceOnRemovedDependencies(
	resourceInfo *provider.ResourceInfo,
	stagingState *stageChangesState,
) (bool, error) {
	stagingState.mu.Lock()
	defer stagingState.mu.Unlock()

	for _, element := range stagingState.mustRecreate.resources {
		if element.resourceName == resourceInfo.ResourceName {
			return true, nil
		}
	}

	return false, nil
}

func (c *defaultBlueprintContainer) getResourceInfo(
	ctx context.Context,
	stageInfo *stageResourceChangeInfo,
) (*provider.ResourceInfo, *subengine.ResolveInResourceResult, error) {
	resolveResourceResult, err := c.substitutionResolver.ResolveInResource(
		ctx,
		stageInfo.node.ResourceName,
		stageInfo.node.Resource,
		&subengine.ResolveResourceTargetInfo{
			ResolveFor: subengine.ResolveForChangeStaging,
		},
	)
	if err != nil {
		return nil, nil, err
	}
	_, cached := c.resourceCache.Get(stageInfo.node.ResourceName)
	if !cached {
		c.resourceCache.Set(
			stageInfo.node.ResourceName,
			resolveResourceResult.ResolvedResource,
		)
	}

	var currentResourceStatePtr *state.ResourceState
	currentResourceState, err := c.stateContainer.GetResourceByName(
		ctx,
		stageInfo.instanceID,
		stageInfo.node.ResourceName,
	)
	if err != nil {
		if !state.IsResourceNotFound(err) {
			return nil, nil, err
		}
	} else {
		currentResourceStatePtr = &currentResourceState
	}

	return &provider.ResourceInfo{
		ResourceID:               stageInfo.resourceID,
		ResourceName:             stageInfo.node.ResourceName,
		InstanceID:               stageInfo.instanceID,
		CurrentResourceState:     currentResourceStatePtr,
		ResourceWithResolvedSubs: resolveResourceResult.ResolvedResource,
	}, resolveResourceResult, nil
}

func (c *defaultBlueprintContainer) prepareAndStageLinkChanges(
	ctx context.Context,
	currentResourceInfo *provider.ResourceInfo,
	linksReadyToBeStaged []*linkPendingCompletion,
	linkChangesChan chan LinkChangesMessage,
	stagingState *stageChangesState,
	params core.BlueprintParams,
) error {
	for _, readyToStage := range linksReadyToBeStaged {
		linkImpl, err := getLinkImplementation(
			readyToStage.resourceANode,
			readyToStage.resourceBNode,
		)
		if err != nil {
			return err
		}

		// Links are staged in series to reflect what happens with deployment.
		// For deployment, multiple links could be modifying the same resource,
		// to ensure consistency in state, links involving the same resource will be
		// both staged and deployed synchronously.
		err = c.stageLinkChanges(
			ctx,
			linkImpl,
			currentResourceInfo,
			readyToStage,
			linkChangesChan,
			stagingState,
			params,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *defaultBlueprintContainer) stageLinkChanges(
	ctx context.Context,
	linkImpl provider.Link,
	currentResourceInfo *provider.ResourceInfo,
	readyToStage *linkPendingCompletion,
	linkChangesChan chan LinkChangesMessage,
	stagingState *stageChangesState,
	params core.BlueprintParams,
) error {
	resourceAInfo, err := c.getResourceInfoForLink(ctx, readyToStage.resourceANode, currentResourceInfo)
	if err != nil {
		return err
	}

	resourceBInfo, err := c.getResourceInfoForLink(ctx, readyToStage.resourceBNode, currentResourceInfo)
	if err != nil {
		return err
	}

	var currentLinkStatePtr *state.LinkState
	currentLinkState, err := c.stateContainer.GetLink(
		ctx,
		resourceAInfo.InstanceID,
		createLogicalLinkName(resourceAInfo.ResourceName, resourceBInfo.ResourceName),
	)
	if err != nil {
		if !state.IsLinkNotFound(err) {
			return err
		}
	} else {
		currentLinkStatePtr = &currentLinkState
	}

	// Obtain a lock as getResourceChanges requires a lock to have already been
	// acquired on the staging state.
	stagingState.mu.Lock()
	resourceAChanges := getResourceChanges(resourceAInfo.ResourceName, stagingState.outputChanges)
	resourceBChanges := getResourceChanges(resourceBInfo.ResourceName, stagingState.outputChanges)
	stagingState.mu.Unlock()

	output, err := linkImpl.StageChanges(ctx, &provider.LinkStageChangesInput{
		ResourceAChanges: resourceAChanges,
		ResourceBChanges: resourceBChanges,
		CurrentLinkState: currentLinkStatePtr,
		Params:           params,
	})
	if err != nil {
		return err
	}

	c.markLinkAsNoLongerPending(
		readyToStage.resourceANode,
		readyToStage.resourceBNode,
		stagingState,
	)

	linkChangesChan <- LinkChangesMessage{
		ResourceAName: resourceAInfo.ResourceName,
		ResourceBName: resourceBInfo.ResourceName,
		Changes:       getChangesFromStageLinkChangesOutput(output),
		New:           currentLinkStatePtr == nil,
		Removed:       false,
	}

	return nil
}

func (c *defaultBlueprintContainer) getResourceInfoForLink(
	ctx context.Context,
	node *links.ChainLinkNode,
	currentResourceInfo *provider.ResourceInfo,
) (*provider.ResourceInfo, error) {
	if node.ResourceName != currentResourceInfo.ResourceName {
		resourceInfo, _, err := c.getResourceInfo(ctx, &stageResourceChangeInfo{
			node:       node,
			instanceID: currentResourceInfo.InstanceID,
		})
		return resourceInfo, err
	}

	return currentResourceInfo, nil
}

func (c *defaultBlueprintContainer) stageChildBlueprintChanges(
	ctx context.Context,
	parentInstanceID string,
	parentInstanceTreePath string,
	includeTreePath string,
	node *validation.ReferenceChainNode,
	paramOverrides core.BlueprintParams,
	channels *ChangeStagingChannels,
) {

	includeName := strings.TrimPrefix(node.ElementName, "children.")

	resolvedInclude, err := c.resolveIncludeForChildBlueprint(
		ctx,
		node,
		includeName,
	)
	if err != nil {
		channels.ErrChan <- err
		return
	}

	childBlueprintInfo, err := c.childResolver.Resolve(ctx, includeName, resolvedInclude, paramOverrides)
	if err != nil {
		channels.ErrChan <- err
		return
	}

	childParams := paramOverrides.
		WithBlueprintVariables(
			extractIncludeVariables(resolvedInclude),
			/* keepExisting */ false,
		).
		WithContextVariables(
			createContextVarsForChildBlueprint(
				parentInstanceID,
				parentInstanceTreePath,
				includeTreePath,
			),
			/* keepExisting */ true,
		)

	childLoader := c.createChildBlueprintLoader([]string{})

	var childContainer BlueprintContainer
	if childBlueprintInfo.AbsolutePath != nil {
		childContainer, err = childLoader.Load(ctx, *childBlueprintInfo.AbsolutePath, childParams)
		if err != nil {
			channels.ErrChan <- err
			return
		}
	} else {
		format, err := extractChildBlueprintFormat(includeName, resolvedInclude)
		if err != nil {
			channels.ErrChan <- err
			return
		}

		childContainer, err = childLoader.LoadString(
			ctx,
			*childBlueprintInfo.BlueprintSource,
			format,
			childParams,
		)
		if err != nil {
			channels.ErrChan <- err
			return
		}
	}

	childState, err := c.getChildState(ctx, parentInstanceID, includeName)
	if err != nil {
		channels.ErrChan <- err
		return
	}

	if hasBlueprintCycle(parentInstanceTreePath, childState.InstanceID) {
		channels.ErrChan <- errBlueprintCycleDetected(
			includeName,
			parentInstanceTreePath,
			childState.InstanceID,
		)
		return
	}

	childChannels := &ChangeStagingChannels{
		ResourceChangesChan: make(chan ResourceChangesMessage),
		ChildChangesChan:    make(chan ChildChangesMessage),
		LinkChangesChan:     make(chan LinkChangesMessage),
		CompleteChan:        make(chan BlueprintChanges),
		ErrChan:             make(chan error),
	}
	err = childContainer.StageChanges(
		ctx,
		&StageChangesInput{
			InstanceID: childState.InstanceID,
		},
		childChannels,
		childParams,
	)
	if err != nil {
		channels.ErrChan <- err
		return
	}

	c.waitForChildChanges(includeName, childState, childChannels, channels)
}

func (c *defaultBlueprintContainer) getChildState(
	ctx context.Context,
	parentInstanceID string,
	includeName string,
) (*state.InstanceState, error) {
	childState, err := c.stateContainer.GetChild(ctx, parentInstanceID, includeName)
	if err != nil {
		if !state.IsInstanceNotFound(err) {
			return nil, err
		} else {
			// Change staging includes describing the planned state for a new blueprint,
			// an empty instance ID will be used to indicate that the blueprint instance is new.
			return &state.InstanceState{
				InstanceID: "",
			}, nil
		}
	}

	return &childState, nil
}

func (c *defaultBlueprintContainer) waitForChildChanges(
	includeName string,
	childState *state.InstanceState,
	childChannels *ChangeStagingChannels,
	channels *ChangeStagingChannels,
) {
	// For now, when it comes to child blueprints,
	// wait for all changes to be staged before sending
	// an update message for the parent blueprint context.
	// In the future, we may want to stream changes
	// in child blueprints like with resources and links
	// in the parent blueprint.
	var changes BlueprintChanges
	receivedFullChildChanges := false
	var stagingErr error
	for !receivedFullChildChanges && stagingErr == nil {
		select {
		case <-childChannels.ResourceChangesChan:
		case <-childChannels.LinkChangesChan:
		case <-childChannels.ChildChangesChan:
		case changes = <-childChannels.CompleteChan:
			c.cacheChildExportFields(includeName, &changes)
			channels.ChildChangesChan <- ChildChangesMessage{
				ChildBlueprintName: includeName,
				Removed:            false,
				New:                childState.InstanceID == "",
				Changes:            changes,
			}
		case stagingErr = <-childChannels.ErrChan:
			channels.ErrChan <- stagingErr
		}
	}
}

func (c *defaultBlueprintContainer) cacheChildExportFields(
	childName string,
	changes *BlueprintChanges,
) {
	for exportName, fieldChange := range changes.ExportChanges {
		c.cacheChildExportField(
			childName,
			changes,
			exportName,
			fieldChange,
		)
	}

	for exportName, fieldChange := range changes.NewExports {
		c.cacheChildExportField(
			childName,
			changes,
			exportName,
			fieldChange,
		)
	}

	for _, exportName := range changes.RemovedExports {
		key := substitutions.RenderFieldPath(childName, exportName)
		c.childExportFieldCache.Set(
			key,
			&subengine.ChildExportFieldInfo{
				Value:           nil,
				Removed:         true,
				ResolveOnDeploy: false,
			},
		)
	}
}

func (c *defaultBlueprintContainer) cacheChildExportField(
	childName string,
	changes *BlueprintChanges,
	exportName string,
	fieldChange provider.FieldChange,
) {
	key := substitutions.RenderFieldPath(childName, exportName)
	exportFieldPath := substitutions.RenderFieldPath("exports", exportName)
	willResolveOnDeploy := slices.Contains(
		changes.ResolveOnDeploy,
		exportFieldPath,
	)

	c.childExportFieldCache.Set(
		key,
		&subengine.ChildExportFieldInfo{
			Value:           fieldChange.NewValue,
			Removed:         false,
			ResolveOnDeploy: willResolveOnDeploy,
		},
	)
}

func (c *defaultBlueprintContainer) resolveIncludeForChildBlueprint(
	ctx context.Context,
	node *validation.ReferenceChainNode,
	includeName string,
) (*subengine.ResolvedInclude, error) {
	include, isInclude := node.Element.(*schema.Include)
	if !isInclude {
		return nil, fmt.Errorf("child blueprint node is not an include")
	}

	resolvedIncludeResult, err := c.substitutionResolver.ResolveInInclude(
		ctx,
		includeName,
		include,
		&subengine.ResolveIncludeTargetInfo{
			ResolveFor: subengine.ResolveForChangeStaging,
		},
	)
	if err != nil {
		return nil, err
	}

	if len(resolvedIncludeResult.ResolveOnDeploy) > 0 {
		return nil, fmt.Errorf(
			"child blueprint include %q has unresolved substitutions, "+
				"changes can only be staged for child blueprints when "+
				"all the information required to fetch and load the blueprint is available",
			node.ElementName,
		)
	}

	return resolvedIncludeResult.ResolvedInclude, nil
}

func (c *defaultBlueprintContainer) stageRemovals(
	ctx context.Context,
	instanceID string,
	stagingState *stageChangesState,
	// Use the grouped deployment nodes to compare with the current instance
	// state.
	// c.spec.Schema() must NOT be used at this stage as it does not contain
	// the expanded representation of blueprints that contain resource
	// templates.
	deploymentNodes [][]*DeploymentNode,
	channels *ChangeStagingChannels,
) error {
	instanceState, err := c.stateContainer.GetInstance(ctx, instanceID)
	if err != nil {
		if !state.IsInstanceNotFound(err) {
			return err
		}
	}

	flattenedNodes := core.Flatten(deploymentNodes)

	c.stageResourceRemovals(&instanceState, stagingState, flattenedNodes, channels)
	c.stageLinkRemovals(&instanceState, stagingState, flattenedNodes, channels)
	c.stageChildRemovals(&instanceState, stagingState, flattenedNodes, channels)

	return nil
}

func (c *defaultBlueprintContainer) stageResourceRemovals(
	instanceState *state.InstanceState,
	stagingState *stageChangesState,
	flattenedNodes []*DeploymentNode,
	channels *ChangeStagingChannels,
) {
	for _, resourceState := range instanceState.Resources {
		inDeployNodes := slices.ContainsFunc(flattenedNodes, func(node *DeploymentNode) bool {
			return node.ChainLinkNode != nil &&
				node.ChainLinkNode.ResourceName == resourceState.ResourceName
		})
		if !inDeployNodes {
			dependents := findDependents(
				resourceState,
				flattenedNodes,
				instanceState,
			)
			addElementsThatMustBeRecreatedToState(
				dependents,
				stagingState,
			)
			changes := ResourceChangesMessage{
				ResourceName: resourceState.ResourceName,
				Removed:      true,
			}
			applyResourceChangesToState(changes, stagingState)
			channels.ResourceChangesChan <- changes
		}
	}
}

func (c *defaultBlueprintContainer) stageLinkRemovals(
	instanceState *state.InstanceState,
	stagingState *stageChangesState,
	flattenedNodes []*DeploymentNode,
	channels *ChangeStagingChannels,
) {
	for linkName := range instanceState.Links {
		// Links are stored as a map of "resourceAName::resourceBName"
		// so we need to split the link name to get the resource names.
		linkParts := strings.Split(linkName, "::")
		resourceAName := linkParts[0]
		resourceBName := linkParts[1]

		inDeployNodes := slices.ContainsFunc(flattenedNodes, func(node *DeploymentNode) bool {
			return node.ChainLinkNode != nil &&
				(node.ChainLinkNode.ResourceName == resourceAName ||
					node.ChainLinkNode.ResourceName == resourceBName)
		})
		if !inDeployNodes {
			changes := LinkChangesMessage{
				ResourceAName: resourceAName,
				ResourceBName: resourceBName,
				Removed:       true,
			}
			applyLinkChangesToState(changes, stagingState)
			channels.LinkChangesChan <- changes
		}
	}
}

func (c *defaultBlueprintContainer) stageChildRemovals(
	instanceState *state.InstanceState,
	stagingState *stageChangesState,
	flattenedNodes []*DeploymentNode,
	channels *ChangeStagingChannels,
) {
	for childName, childState := range instanceState.ChildBlueprints {
		childElementID := core.ChildElementID(childName)
		inDeployNodes := slices.ContainsFunc(flattenedNodes, func(node *DeploymentNode) bool {
			return node.ChildNode != nil &&
				node.ChildNode.ElementName == childElementID
		})
		if !inDeployNodes {
			dependents := findDependents(
				state.WrapChildBlueprintInstance(childName, childState),
				flattenedNodes,
				instanceState,
			)
			addElementsThatMustBeRecreatedToState(
				dependents,
				stagingState,
			)

			changes := ChildChangesMessage{
				ChildBlueprintName: childName,
				Removed:            true,
			}
			applyChildChangesToState(changes, stagingState)
			channels.ChildChangesChan <- changes
		}
	}
}

func (c *defaultBlueprintContainer) resolveAndCollectExportChanges(
	ctx context.Context,
	instanceID string,
	blueprint *schema.Blueprint,
	stagingState *stageChangesState,
) error {

	if blueprint.Exports == nil {
		return nil
	}

	resolvedExports := map[string]*subengine.ResolveResult{}
	for exportName, export := range blueprint.Exports.Values {
		resolvedExport, err := c.resolveExport(ctx, exportName, export)
		if err != nil {
			return err
		}

		if resolvedExport != nil {
			resolvedExports[exportName] = resolvedExport
		}
	}

	blueprintExportsState, err := c.stateContainer.GetExports(ctx, instanceID)
	if err != nil {
		if !state.IsInstanceNotFound(err) {
			return err
		}
	}
	// Collect export changes in a temporary structure to avoid locking the staging state
	// for the entire duration of the operation.
	collectedExportChanges := &intermediaryBlueprintChanges{
		NewExports:       map[string]*provider.FieldChange{},
		ExportChanges:    map[string]*provider.FieldChange{},
		RemovedExports:   []string{},
		UnchangedExports: []string{},
		ResolveOnDeploy:  []string{},
	}
	collectExportChanges(collectedExportChanges, resolvedExports, blueprintExportsState)
	c.updateExportChangesInState(collectedExportChanges, stagingState)

	return nil
}

func (c *defaultBlueprintContainer) updateExportChangesInState(
	collectedExportChanges *intermediaryBlueprintChanges,
	stagingState *stageChangesState,
) {
	stagingState.mu.Lock()
	defer stagingState.mu.Unlock()

	stagingState.outputChanges.NewExports = collectedExportChanges.NewExports
	stagingState.outputChanges.ExportChanges = collectedExportChanges.ExportChanges
	stagingState.outputChanges.UnchangedExports = collectedExportChanges.UnchangedExports
	stagingState.outputChanges.RemovedExports = collectedExportChanges.RemovedExports
	stagingState.outputChanges.ResolveOnDeploy = append(
		stagingState.outputChanges.ResolveOnDeploy,
		collectedExportChanges.ResolveOnDeploy...,
	)
}

func (c *defaultBlueprintContainer) resolveExport(
	ctx context.Context,
	exportName string,
	export *schema.Export,
) (*subengine.ResolveResult, error) {
	if export.Field != nil && export.Field.StringValue != nil {
		exportFieldAsSub, err := substitutions.ParseSubstitution(
			"exports",
			*export.Field.StringValue,
			/* parentSourceStart */ &source.Meta{Position: source.Position{}},
			/* outputLineInfo */ false,
			/* ignoreParentColumn */ true,
		)
		if err != nil {
			return nil, err
		}

		return c.substitutionResolver.ResolveSubstitution(
			ctx,
			&substitutions.StringOrSubstitution{
				SubstitutionValue: exportFieldAsSub,
			},
			core.ExportElementID(exportName),
			"field",
			&subengine.ResolveTargetInfo{
				ResolveFor: subengine.ResolveForChangeStaging,
			},
		)
	}

	return nil, nil
}

func (c *defaultBlueprintContainer) markLinkAsNoLongerPending(
	resourceANode, resourceBNode *links.ChainLinkNode,
	stagingState *stageChangesState,
) {
	stagingState.mu.Lock()
	defer stagingState.mu.Unlock()

	linkName := createLogicalLinkName(resourceANode.ResourceName, resourceBNode.ResourceName)
	pendingLink := stagingState.pendingLinks[linkName]
	pendingLink.linkPending = false
}

func (c *defaultBlueprintContainer) updateStagingState(
	node *links.ChainLinkNode,
	stagingState *stageChangesState,
) []*linkPendingCompletion {
	stagingState.mu.Lock()
	defer stagingState.mu.Unlock()

	hasLinks := len(node.LinksTo) > 0 || len(node.LinkedFrom) > 0
	pendingLinkNames := stagingState.resourceNameLinkMap[node.ResourceName]
	if hasLinks {
		c.addPendingLinksToStagingState(node, pendingLinkNames, stagingState)
	}
	return c.updatePendingLinksInStagingState(node, stagingState, pendingLinkNames)
}

// This must only be called when a lock has already been held on the staging state.
func (c *defaultBlueprintContainer) addPendingLinksToStagingState(
	node *links.ChainLinkNode,
	alreadyPendingLinks []string,
	stagingState *stageChangesState,
) {
	for _, linksToNode := range node.LinksTo {
		linkName := createLogicalLinkName(node.ResourceName, linksToNode.ResourceName)
		if !slices.Contains(alreadyPendingLinks, linkName) {
			completionState := &linkPendingCompletion{
				resourceANode:    node,
				resourceBNode:    linksToNode,
				resourceAPending: false,
				resourceBPending: true,
				linkPending:      true,
			}
			stagingState.pendingLinks[linkName] = completionState
			stagingState.resourceNameLinkMap[node.ResourceName] = append(
				stagingState.resourceNameLinkMap[node.ResourceName],
				linkName,
			)
			stagingState.resourceNameLinkMap[linksToNode.ResourceName] = append(
				stagingState.resourceNameLinkMap[linksToNode.ResourceName],
				linkName,
			)
		}
	}

	for _, linkedFromNode := range node.LinkedFrom {
		linkName := createLogicalLinkName(linkedFromNode.ResourceName, node.ResourceName)
		if !slices.Contains(alreadyPendingLinks, linkName) {
			completionState := &linkPendingCompletion{
				resourceANode:    linkedFromNode,
				resourceBNode:    node,
				resourceAPending: true,
				resourceBPending: false,
				linkPending:      true,
			}
			stagingState.pendingLinks[linkName] = completionState
			stagingState.resourceNameLinkMap[linkedFromNode.ResourceName] = append(
				stagingState.resourceNameLinkMap[linkedFromNode.ResourceName],
				linkName,
			)
			stagingState.resourceNameLinkMap[node.ResourceName] = append(
				stagingState.resourceNameLinkMap[node.ResourceName],
				linkName,
			)
		}
	}
}

// This must only be called when a lock has already been held on the staging state.
func (c *defaultBlueprintContainer) updatePendingLinksInStagingState(
	node *links.ChainLinkNode,
	stagingState *stageChangesState,
	pendingLinkNames []string,
) []*linkPendingCompletion {
	linksReadyToBeStaged := []*linkPendingCompletion{}

	for _, linkName := range pendingLinkNames {
		completionState := stagingState.pendingLinks[linkName]
		if completionState.resourceANode.ResourceName == node.ResourceName {
			completionState.resourceAPending = false
		} else if completionState.resourceBNode.ResourceName == node.ResourceName {
			completionState.resourceBPending = false
		}

		if !completionState.resourceAPending && !completionState.resourceBPending {
			linksReadyToBeStaged = append(linksReadyToBeStaged, completionState)
		}
	}

	return linksReadyToBeStaged
}

func (c *defaultBlueprintContainer) applyResourceConditions(
	ctx context.Context,
	blueprint *schema.Blueprint,
	resolveFor subengine.ResolveForStage,
) (*schema.Blueprint, error) {

	if blueprint.Resources == nil {
		return blueprint, nil
	}

	resourcesAfterConditions := map[string]*schema.Resource{}
	for resourceName, resource := range blueprint.Resources.Values {
		if resource.Condition != nil {
			resolveResourceResult, err := c.substitutionResolver.ResolveInResource(
				ctx,
				resourceName,
				resource,
				&subengine.ResolveResourceTargetInfo{
					ResolveFor: resolveFor,
				},
			)
			if err != nil {
				return nil, err
			}

			conditionKnownOnDeploy := isConditionKnownOnDeploy(
				resourceName,
				resolveResourceResult.ResolveOnDeploy,
			)
			if resolveFor == subengine.ResolveForChangeStaging &&
				(conditionKnownOnDeploy ||
					evaluateCondition(resolveResourceResult.ResolvedResource.Condition)) {

				c.resourceCache.Set(resourceName, resolveResourceResult.ResolvedResource)

				resourcesAfterConditions[resourceName] = resource
			}
		} else {
			resourcesAfterConditions[resourceName] = resource
		}
	}

	return &schema.Blueprint{
		Version:   blueprint.Version,
		Transform: blueprint.Transform,
		Variables: blueprint.Variables,
		Values:    blueprint.Values,
		Include:   blueprint.Include,
		Resources: &schema.ResourceMap{
			Values: resourcesAfterConditions,
		},
		DataSources: blueprint.DataSources,
		Exports:     blueprint.Exports,
		Metadata:    blueprint.Metadata,
	}, nil
}

func (c *defaultBlueprintContainer) expandResourceTemplates(
	ctx context.Context,
	params core.BlueprintParams,
) (BlueprintContainer, error) {

	chains, err := c.linkInfo.Links(ctx)
	if err != nil {
		return nil, err
	}

	expandResult, err := ExpandResourceTemplates(
		ctx,
		c.spec.Schema(),
		c.substitutionResolver,
		chains,
		c.resourceTemplateInputElemCache,
	)
	if err != nil {
		return nil, err
	}

	populateDefaultsIn := c.spec.Schema()
	if len(expandResult.ResourceTemplateMap) > 0 {
		populateDefaultsIn = expandResult.ExpandedBlueprint
	}

	// Populate defaults before applying conditions to ensure that the
	// resolved resources that are cached when applying conditions
	// are populated with the default values.
	applyConditionsTo, err := PopulateResourceSpecDefaults(
		ctx,
		populateDefaultsIn,
		params,
		c.resourceRegistry,
	)
	if err != nil {
		return nil, err
	}

	afterConditionsApplied, err := c.applyResourceConditions(
		ctx,
		applyConditionsTo,
		subengine.ResolveForChangeStaging,
	)
	if err != nil {
		return nil, err
	}

	loader := c.createChildBlueprintLoader(
		flattenMapLists(expandResult.ResourceTemplateMap),
	)
	return loader.LoadFromSchema(ctx, afterConditionsApplied, params)
}

func (c *defaultBlueprintContainer) stageInstanceRemoval(
	ctx context.Context,
	instanceID string,
	channels *ChangeStagingChannels,
) {

	instanceState, err := c.stateContainer.GetInstance(ctx, instanceID)
	if err != nil {
		channels.ErrChan <- err
		return
	}

	changes := getInstanceRemovalChanges(&instanceState)

	// For staging changes for destroying an instance, we don't need to individually
	// dispatch resource, link, and child changes. We can just send the complete
	// set of changes to the complete channel.
	channels.CompleteChan <- changes
}

// ChangeStagingChannels contains all the channels required to stream
// change staging events.
type ChangeStagingChannels struct {
	// ResourceChangesChan receives change sets for each resource in the blueprint.
	ResourceChangesChan chan ResourceChangesMessage
	// ChildChangesChan receives change sets for child blueprints once all
	// changes for the child blueprint have been staged.
	ChildChangesChan chan ChildChangesMessage
	// LinkChangesChan receives change sets for links between resources.
	LinkChangesChan chan LinkChangesMessage
	// CompleteChan is used to signal that all changes have been staged
	// containing the full set of changes that will be made to the blueprint instance
	// when deploying the changes.
	CompleteChan chan BlueprintChanges
	// ErrChan is used to signal that an error occurred while staging changes.
	ErrChan chan error
}

// ResourceChangesMessage provides a message containing the changes
// that will be made to a resource in a blueprint instance.
type ResourceChangesMessage struct {
	ResourceName    string           `json:"resourceName"`
	Removed         bool             `json:"removed"`
	New             bool             `json:"new"`
	Changes         provider.Changes `json:"changes"`
	ResolveOnDeploy []string         `json:"resolveOnDeploy"`
	// ConditionKnownOnDeploy is used to indicate that the condition for the resource
	// can not be resolved until the blueprint is deployed.
	// This means the changes described in this message may not be applied
	// if the condition evaluates to false when the blueprint is deployed.
	ConditionKnownOnDeploy bool `json:"conditionKnownOnDeploy"`
}

// ChildChangesMessage provides a message containing the changes
// that will be made to a child blueprint in a blueprint instance.
type ChildChangesMessage struct {
	ChildBlueprintName string           `json:"childBlueprintName"`
	Removed            bool             `json:"removed"`
	New                bool             `json:"new"`
	Changes            BlueprintChanges `json:"changes"`
}

// LinkChangesMessage provides a message containing the changes
// that will be made to a link between resources in a blueprint instance.
type LinkChangesMessage struct {
	ResourceAName string               `json:"resourceAName"`
	ResourceBName string               `json:"resourceBName"`
	Removed       bool                 `json:"removed"`
	New           bool                 `json:"new"`
	Changes       provider.LinkChanges `json:"changes"`
}

type stageChangesState struct {
	// A mapping of a link ID to the pending link completion state.
	// A link ID in this context is made up of the resource names of the two resources
	// that are linked together.
	// For example, if resource A is linked to resource B, the link ID would be "A::B".
	pendingLinks map[string]*linkPendingCompletion
	// A mapping of resource names to pending links that include the resource.
	resourceNameLinkMap map[string][]string
	// The full set of changes that will be sent to the caller-provided complete channel
	// when all changes have been staged.
	// This is an intermediary format that holds pointers to resource change sets to allow
	// modification without needing to copy and patch resource change sets back in to the state
	// each time resource change set state needs to be updated with link change sets.
	outputChanges *intermediaryBlueprintChanges
	// A set of elements that must be recreated due to removal of dependencies.
	mustRecreate *collectedElements
	// Mutex is required as resources can be staged concurrently.
	mu sync.Mutex
}

type intermediaryBlueprintChanges struct {
	NewResources     map[string]*provider.Changes
	ResourceChanges  map[string]*provider.Changes
	RemovedResources []string
	RemovedLinks     []string
	NewChildren      map[string]*NewBlueprintDefinition
	ChildChanges     map[string]*BlueprintChanges
	RemovedChildren  []string
	NewExports       map[string]*provider.FieldChange
	ExportChanges    map[string]*provider.FieldChange
	RemovedExports   []string
	UnchangedExports []string
	ResolveOnDeploy  []string
}

type stageResourceChangeInfo struct {
	node       *links.ChainLinkNode
	instanceID string
	resourceID string
}

type changesWrapper struct {
	resourceChanges *provider.Changes
	childChanges    *BlueprintChanges
}
