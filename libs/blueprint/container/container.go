package container

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/includes"
	"github.com/two-hundred/celerity/libs/blueprint/links"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/speccore"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
	"github.com/two-hundred/celerity/libs/blueprint/validation"
)

// BlueprintContainer provides the interface for a service that manages
// staging and deploying an instance of a blueprint.
type BlueprintContainer interface {
	// StageChanges deals with staging changes to be deployed, comparing the effect of applying
	// the loaded blueprint to the existing instance stored in state for the instance ID it was
	// loaded with.
	// This will stream changes to the provided channels for each resource, child blueprint and link
	// that will be affected by the changes, a final message will be sent to the complete channel
	// containing the full set of changes that will be made to the blueprint instance when deploying
	// the changes.
	// Parameter overrides can be provided to add extra instance-specific variables
	// that were not defined when the container was loaded or to provide all variables
	// when the container was loaded with an empty set.
	StageChanges(
		ctx context.Context,
		instanceID string,
		channels *ChangeStagingChannels,
		paramOverrides core.BlueprintParams,
	) error
	// Deploy deals with deploying the blueprint for the given instance ID.
	// Deploying a blueprint involves creating, updating and destroying resources
	// based on the staged changes.
	// This will stream updates to the provided channels for each resource, child blueprint and link
	// that has either been updated, created or removed.
	// Deploy should also be used as the mechanism to rollback a blueprint to a previous
	// revision managed in version control or a data store for blueprint source documents.
	Deploy(
		ctx context.Context,
		instanceID string,
		changes *BlueprintChanges,
		channels *DeployChannels,
		paramOverrides core.BlueprintParams,
	) error
	// Destroy deals with destroying all the resources, child blueprints and links
	// for a blueprint instance.
	// This will stream updates to the provided channels for each resource, child blueprint and link
	// that has been removed.
	Destroy(
		ctx context.Context,
		instanceID string,
		channels *DestroyChannels,
		paramOverrides core.BlueprintParams,
	) error
	// SpecLinkInfo provides the chain link and warnings for potential issues
	// with links provided in the given specification.
	SpecLinkInfo() links.SpecLinkInfo
	// BlueprintSpec returns the specification for the loaded blueprint
	// including the parsed schema and a convenience method to get resource
	// schemas by name.
	BlueprintSpec() speccore.BlueprintSpec
	// Diagnostics returns warning and informational diagnostics for the loaded blueprint
	// that point out potential issues that may occur when executing
	// a blueprint.
	// These diagnostics can contain errors, however, the error returned on failure to load a blueprint
	// should also be unpacked to get the precise location and information about the reason loading the
	// blueprint failed.
	Diagnostics() []*core.Diagnostic
}

// BlueprintChanges provides a set of changes that will be made
// to a blueprint instance when deploying a new version of the source blueprint.
// This contains a mapping of resource name
// to the changes that will come into effect upon deploying
// the currently loaded version of a blueprint for a given
// instance ID.
// This also contains a mapping of child blueprint names to the changes
// that will come into effect upon deploying the child blueprint.
// Changes takes the type parameter interface{} as we can't know the exact
// range of resource types for a blueprint at compile time.
// We must check the resource types associated with a set of changes
// at runtime.
type BlueprintChanges struct {
	// NewResources contains the resources that will be created
	// when deploying the changes.
	NewResources map[string]provider.Changes `json:"newResources"`
	// ResourceChanges contains the changes that will be made to
	// existing resources when deploying the changes.
	ResourceChanges map[string]provider.Changes `json:"resourceChanges"`
	// RemovedResources contains the name of the resources that will be removed
	// when deploying the changes.
	RemovedResources []string `json:"removedResources"`
	// RemovedLinks contains the name of the links that will be removed
	// when deploying the changes.
	// These will be in the format "resourceAName::resourceBName".
	RemovedLinks []string `json:"removedLinks"`
	// NewChildren contains the child blueprints that will be created
	// when deploying the changes.
	NewChildren map[string]NewBlueprintDefinition `json:"newChildren"`
	// ChildChanges contains the changes that will be made to the child blueprints
	// when deploying the changes.
	ChildChanges map[string]BlueprintChanges `json:"childChanges"`
	// RemovedChildren contains the name of the child blueprints that will be removed
	// when deploying the changes.
	RemovedChildren []string `json:"removedChildren"`
	// ResolveOnDeploy contains paths to properties in blueprint elements
	// that contain substitutions that can not be resolved until the blueprint
	// is deployed.
	ResolveOnDeploy []string `json:"resolveOnDeploy"`
}

// NewBlueprintDefinition provides a definition for a new child blueprint
// that will be created when deploying a blueprint instance.
type NewBlueprintDefinition struct {
	NewResources map[string]provider.Changes       `json:"newResources"`
	NewChildren  map[string]NewBlueprintDefinition `json:"newChildren"`
}

type defaultBlueprintContainer struct {
	stateContainer state.Container
	// Should be a mapping of resource name to the provider
	// that serves the resource.
	resourceProviders              map[string]provider.Provider
	spec                           speccore.BlueprintSpec
	linkInfo                       links.SpecLinkInfo
	refChainCollector              validation.RefChainCollector
	substitutionResolver           subengine.SubstitutionResolver
	changeStager                   ResourceChangeStager
	childResolver                  includes.ChildResolver
	resourceCache                  *core.Cache[*provider.ResolvedResource]
	resourceTemplateInputElemCache *core.Cache[[]*core.MappingNode]
	diagnostics                    []*core.Diagnostic
	createChildBlueprintLoader     func() Loader
}

// BlueprintContainerDependencies provides the dependencies
// required to create a new instance of a blueprint container.
type BlueprintContainerDependencies struct {
	StateContainer              state.Container
	ResourceProviders           map[string]provider.Provider
	LinkInfo                    links.SpecLinkInfo
	RefChainCollector           validation.RefChainCollector
	SubstitutionResolver        subengine.SubstitutionResolver
	ChangeStager                ResourceChangeStager
	ChildResolver               includes.ChildResolver
	ResourceCache               *core.Cache[*provider.ResolvedResource]
	ChildBlueprintLoaderFactory func() Loader
}

// NewDefaultBlueprintContainer creates a new instance of the default
// implementation of a blueprint container for a loaded spec.
// The map of resource providers must be a map of provider resource name
// to a provider.
func NewDefaultBlueprintContainer(
	spec speccore.BlueprintSpec,
	deps *BlueprintContainerDependencies,
	diagnostics []*core.Diagnostic,
) BlueprintContainer {
	return &defaultBlueprintContainer{
		deps.StateContainer,
		deps.ResourceProviders,
		spec,
		deps.LinkInfo,
		deps.RefChainCollector,
		deps.SubstitutionResolver,
		deps.ChangeStager,
		deps.ChildResolver,
		deps.ResourceCache,
		core.NewCache[[]*core.MappingNode](),
		diagnostics,
		deps.ChildBlueprintLoaderFactory,
	}
}

func (c *defaultBlueprintContainer) StageChanges(
	ctx context.Context,
	instanceID string,
	channels *ChangeStagingChannels,
	paramOverrides core.BlueprintParams,
) error {

	expandedBlueprintContainer, err := c.expandResourceTemplates(ctx, paramOverrides)
	if err != nil {
		return err
	}

	chains, err := expandedBlueprintContainer.SpecLinkInfo().Links(ctx)
	if err != nil {
		return err
	}

	childrenRefNodes := extractChildRefNodes(expandedBlueprintContainer.BlueprintSpec().Schema(), c.refChainCollector)
	orderedNodes, err := OrderItemsForDeployment(ctx, chains, childrenRefNodes, c.refChainCollector, paramOverrides)
	if err != nil {
		return err
	}
	parallelGroups, err := GroupOrderedNodes(ctx, orderedNodes, c.refChainCollector, paramOverrides)
	if err != nil {
		return err
	}

	go c.stageChanges(
		ctx,
		instanceID,
		parallelGroups,
		paramOverrides,
		channels,
	)

	return nil
}

func (c *defaultBlueprintContainer) stageChanges(
	ctx context.Context,
	instanceID string,
	parallelGroups [][]*DeploymentNode,
	paramOverrides core.BlueprintParams,
	channels *ChangeStagingChannels,
) {
	state := &stageChangesState{}
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
		channels.ErrChan <- err
		return
	}

	for _, group := range parallelGroups {
		c.stageGroupChanges(
			ctx,
			instanceID,
			state,
			group,
			paramOverrides,
			internalChannels,
		)

		err := c.listenToAndProcessGroupChanges(
			group,
			internalChannels,
			channels,
			state,
		)
		if err != nil {
			channels.ErrChan <- err
			return
		}
	}
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
	expectedLinkChangesCount := countPendingLinksForGroup(group, state)
	linkChangesCount := 0
	collected := map[string]*changesWrapper{}
	var err error
	for len(collected) < len(group) &&
		linkChangesCount < expectedLinkChangesCount &&
		err == nil {
		select {
		case changes := <-internalChannels.ResourceChangesChan:
			elementName := core.ResourceElementID(changes.ResourceName)
			collected[elementName] = &changesWrapper{
				resourceChanges: &changes.Changes,
			}
			applyResourceChangesToState(changes, state)
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
	}

	if err != nil {
		return err
	}

	if len(collected) == len(group) && linkChangesCount == expectedLinkChangesCount {
		externalChannels.CompleteChan <- BlueprintChanges{
			NewResources:     state.outputChanges.NewResources,
			ResourceChanges:  state.outputChanges.ResourceChanges,
			RemovedResources: state.outputChanges.RemovedResources,
			RemovedLinks:     state.outputChanges.RemovedLinks,
			NewChildren:      state.outputChanges.NewChildren,
			ChildChanges:     state.outputChanges.ChildChanges,
			RemovedChildren:  state.outputChanges.RemovedChildren,
			ResolveOnDeploy:  state.outputChanges.ResolveOnDeploy,
		}
	}

	return nil
}

func countPendingLinksForGroup(group []*DeploymentNode, state *stageChangesState) int {
	state.mu.Lock()
	defer state.mu.Unlock()

	count := 0
	for _, node := range group {
		if node.Type() == "resource" {
			pendingLinkIDs := state.resourceNameLinkMap[node.ChainLinkNode.ResourceName]
			for _, linkID := range pendingLinkIDs {
				if state.pendingLinks[linkID].linkPending {
					count += 1
				}
			}
		}
	}

	return count
}

func applyResourceChangesToState(changes ResourceChangesMessage, state *stageChangesState) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if changes.New {
		state.outputChanges.NewResources[changes.ResourceName] = changes.Changes
	} else if changes.Removed {
		state.outputChanges.RemovedResources = append(
			state.outputChanges.RemovedResources,
			changes.ResourceName,
		)
	} else {
		state.outputChanges.ResourceChanges[changes.ResourceName] = changes.Changes
	}
}

func applyLinkChangesToState(changes LinkChangesMessage, state *stageChangesState) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if changes.Removed {
		state.outputChanges.RemovedLinks = append(
			state.outputChanges.RemovedLinks,
			createLinkID(changes.ResourceAName, changes.ResourceBName),
		)
		return
	}

	pendingLink := state.pendingLinks[createLinkID(changes.ResourceAName, changes.ResourceBName)]
	pendingLink.linkPending = false

	resourceChanges := getResourceChanges(changes.ResourceAName, state.outputChanges)
	if resourceChanges != nil {
		if changes.New {
			resourceChanges.NewOutboundLinks[changes.ResourceBName] = changes.Changes
		} else {
			resourceChanges.OutboundLinkChanges[changes.ResourceBName] = changes.Changes
		}
	}
}

func applyChildChangesToState(changes ChildChangesMessage, state *stageChangesState) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if changes.New {
		state.outputChanges.NewChildren[changes.ChildBlueprintName] = NewBlueprintDefinition{
			NewResources: changes.Changes.NewResources,
			NewChildren:  changes.Changes.NewChildren,
		}
	} else if changes.Removed {
		state.outputChanges.RemovedChildren = append(
			state.outputChanges.RemovedChildren,
			changes.ChildBlueprintName,
		)
	} else {
		state.outputChanges.ChildChanges[changes.ChildBlueprintName] = changes.Changes
	}
}

// A lock must be held on the staging state when calling this function.
func getResourceChanges(resourceName string, changes *BlueprintChanges) *provider.Changes {

	newResourceChanges, hasNewResourceChanges := changes.NewResources[resourceName]
	if hasNewResourceChanges {
		return &newResourceChanges
	}

	resourceChanges, hasResourceChanges := changes.ResourceChanges[resourceName]
	if hasResourceChanges {
		return &resourceChanges
	}

	return nil
}

func (c *defaultBlueprintContainer) stageGroupChanges(
	ctx context.Context,
	instanceID string,
	stagingState *stageChangesState,
	group []*DeploymentNode,
	paramOverrides core.BlueprintParams,
	channels *ChangeStagingChannels,
) {
	for _, node := range group {
		if node.Type() == "resource" {
			go c.prepareAndStageResourceChanges(
				ctx,
				instanceID,
				stagingState,
				node.ChainLinkNode,
				channels,
				paramOverrides,
			)
		} else if node.Type() == "child" {
			go c.stageChildBlueprintChanges(
				ctx,
				instanceID,
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
	params core.BlueprintParams,
) {
	resourceProvider, hasResourceProvider := c.resourceProviders[node.ResourceName]
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

	changesChan <- ResourceChangesMessage{
		ResourceName:    stageResourceInfo.node.ResourceName,
		Changes:         *changes,
		Removed:         false,
		New:             resourceInfo.CurrentResourceState == nil,
		ResolveOnDeploy: resolveResourceResult.ResolveOnDeploy,
	}

	linksReadyToBeStaged := c.updateStagingState(stageResourceInfo.node, stagingState)

	err = c.prepareAndStageLinkChanges(
		ctx,
		resourceInfo,
		linksReadyToBeStaged,
		linkChangesChan,
		params,
	)
	if err != nil {
		return err
	}

	return nil
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

	var currentResourceStatePtr *state.ResourceState
	currentResourceState, err := c.stateContainer.GetResource(
		ctx,
		stageInfo.instanceID,
		stageInfo.resourceID,
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
		err = c.stageLinkChanges(ctx, linkImpl, currentResourceInfo, readyToStage, linkChangesChan, params)
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

	output, err := linkImpl.StageChanges(ctx, &provider.LinkStageChangesInput{
		ResourceAInfo: resourceAInfo,
		ResourceBInfo: resourceBInfo,
		Params:        params,
	})
	if err != nil {
		return err
	}

	linkChangesChan <- LinkChangesMessage{
		ResourceAName: resourceAInfo.ResourceName,
		ResourceBName: resourceBInfo.ResourceName,
		Changes:       *output.Changes,
		New:           resourceAInfo.CurrentResourceState == nil || resourceBInfo.CurrentResourceState == nil,
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
	node *validation.ReferenceChainNode,
	paramOverrides core.BlueprintParams,
	channels *ChangeStagingChannels,
) {
	include, isInclude := node.Element.(*subengine.ResolvedInclude)
	if !isInclude {
		channels.ErrChan <- fmt.Errorf("child blueprint node is not an include")
		return
	}

	includeName := strings.TrimPrefix(node.ElementName, "children.")
	childBlueprintInfo, err := c.childResolver.Resolve(ctx, includeName, include, paramOverrides)
	if err != nil {
		channels.ErrChan <- err
		return
	}

	childParams := paramOverrides.WithBlueprintVariables(
		extractIncludeVariables(include),
		/* keepExisting */ false,
	)

	childLoader := c.createChildBlueprintLoader()

	var childContainer BlueprintContainer
	if childBlueprintInfo.AbsolutePath != nil {
		childContainer, err = childLoader.Load(ctx, *childBlueprintInfo.AbsolutePath, childParams)
		if err != nil {
			channels.ErrChan <- err
			return
		}
	} else {
		format, err := extractChildBlueprintFormat(includeName, include)
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

	childState, err := c.stateContainer.GetChild(ctx, parentInstanceID, includeName)
	if err != nil {
		if !state.IsInstanceNotFound(err) {
			channels.ErrChan <- err
			return
		} else {
			// Change staging includes describing the planned state for a new blueprint,
			// an empty instance ID will be used to indicate that the blueprint instance is new.
			childState = state.InstanceState{
				InstanceID: "",
			}
		}
	}

	childChannels := &ChangeStagingChannels{
		ResourceChangesChan: make(chan ResourceChangesMessage),
		ChildChangesChan:    make(chan ChildChangesMessage),
		LinkChangesChan:     make(chan LinkChangesMessage),
		CompleteChan:        make(chan BlueprintChanges),
		ErrChan:             make(chan error),
	}
	err = childContainer.StageChanges(ctx, childState.InstanceID, childChannels, childParams)
	if err != nil {
		channels.ErrChan <- err
		return
	}

	// For now, when it comes to child blueprints,
	// wait for all changes to be staged before sending
	// an update message for the parent blueprint context.
	// In the future, we may want to stream changes
	// in child blueprints like with resources and links
	// in the parent blueprint.
	select {
	case changes := <-childChannels.CompleteChan:
		channels.ChildChangesChan <- ChildChangesMessage{
			ChildBlueprintName: includeName,
			Removed:            false,
			New:                childState.InstanceID == "",
			Changes:            changes,
		}
	case err := <-childChannels.ErrChan:
		channels.ErrChan <- err
	}
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

	// TODO: check if resource condition resolved to false and the resource is in the current state,
	// if so, remove the resource for the next deployment.

	// TODO: check if resource condition can not be resolved until deploy time,
	// if so, mark resource as could be removed at deploy time based on how the condition
	// is resolved at deploy time.

	for resourceName := range instanceState.Resources {
		inDeployNodes := slices.ContainsFunc(flattenedNodes, func(node *DeploymentNode) bool {
			return node.ChainLinkNode.ResourceName == resourceName
		})
		if !inDeployNodes {
			changes := ResourceChangesMessage{
				ResourceName: resourceName,
				Removed:      true,
			}
			applyResourceChangesToState(changes, stagingState)
			channels.ResourceChangesChan <- changes
		}
	}

	for linkName := range instanceState.Links {
		// Links are stored as a map of "resourceAName::resourceBName"
		// so we need to split the link name to get the resource names.
		linkParts := strings.Split(linkName, "::")
		resourceAName := linkParts[0]
		resourceBName := linkParts[1]

		inDeployNodes := slices.ContainsFunc(flattenedNodes, func(node *DeploymentNode) bool {
			return node.ChainLinkNode.ResourceName == resourceAName ||
				node.ChainLinkNode.ResourceName == resourceBName
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

	for childName := range instanceState.ChildBlueprints {
		childElementID := core.ChildElementID(childName)
		inDeployNodes := slices.ContainsFunc(flattenedNodes, func(node *DeploymentNode) bool {
			return node.ChildNode.ElementName == childElementID
		})
		if !inDeployNodes {
			changes := ChildChangesMessage{
				ChildBlueprintName: childName,
				Removed:            true,
			}
			applyChildChangesToState(changes, stagingState)
			channels.ChildChangesChan <- changes
		}
	}

	return nil
}

func (c *defaultBlueprintContainer) updateStagingState(
	node *links.ChainLinkNode,
	stagingState *stageChangesState,
) []*linkPendingCompletion {
	stagingState.mu.Lock()
	defer stagingState.mu.Unlock()

	hasLinks := len(node.LinksTo) > 0 || len(node.LinkedFrom) > 0
	pendingLinkIDs := stagingState.resourceNameLinkMap[node.ResourceName]
	if len(pendingLinkIDs) == 0 {
		if hasLinks {
			c.addPendingLinksToStagingState(node, stagingState)
		}
	} else {
		return c.updatePendingLinksInStagingState(node, stagingState, pendingLinkIDs)
	}

	return []*linkPendingCompletion{}
}

// This must only be called when a lock has already been held on the staging state.
func (c *defaultBlueprintContainer) addPendingLinksToStagingState(node *links.ChainLinkNode, stagingState *stageChangesState) {
	for _, linksToNode := range node.LinksTo {
		completionState := &linkPendingCompletion{
			resourceANode:    node,
			resourceBNode:    linksToNode,
			resourceAPending: false,
			resourceBPending: true,
			linkPending:      true,
		}
		linkID := createLinkID(node.ResourceName, linksToNode.ResourceName)
		stagingState.pendingLinks[linkID] = completionState
		stagingState.resourceNameLinkMap[node.ResourceName] = append(
			stagingState.resourceNameLinkMap[node.ResourceName],
			linkID,
		)
		stagingState.resourceNameLinkMap[linksToNode.ResourceName] = append(
			stagingState.resourceNameLinkMap[linksToNode.ResourceName],
			linkID,
		)
	}

	for _, linkedFromNode := range node.LinkedFrom {
		completionState := &linkPendingCompletion{
			resourceANode:    linkedFromNode,
			resourceBNode:    node,
			resourceAPending: true,
			resourceBPending: false,
			linkPending:      true,
		}
		linkID := createLinkID(linkedFromNode.ResourceName, node.ResourceName)
		stagingState.pendingLinks[linkID] = completionState
		stagingState.resourceNameLinkMap[linkedFromNode.ResourceName] = append(
			stagingState.resourceNameLinkMap[linkedFromNode.ResourceName],
			linkID,
		)
		stagingState.resourceNameLinkMap[node.ResourceName] = append(
			stagingState.resourceNameLinkMap[node.ResourceName],
			linkID,
		)
	}
}

// This must only be called when a lock has already been held on the staging state.
func (c *defaultBlueprintContainer) updatePendingLinksInStagingState(
	node *links.ChainLinkNode,
	stagingState *stageChangesState,
	pendingLinkIDs []string,
) []*linkPendingCompletion {
	linksReadyToBeStaged := []*linkPendingCompletion{}

	for _, linkID := range pendingLinkIDs {
		completionState := stagingState.pendingLinks[linkID]
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

	if len(expandResult.ResourceTemplateMap) > 0 {
		loader := c.createChildBlueprintLoader()
		return loader.LoadFromSchema(ctx, expandResult.ExpandedBlueprint, params)
	}

	return c, nil
}

func (c *defaultBlueprintContainer) Deploy(
	ctx context.Context,
	instanceID string,
	changes *BlueprintChanges,
	channels *DeployChannels,
	paramOverrides core.BlueprintParams,
) error {
	// 1. get chain links
	// 2. traverse through chains and order resources to be created, destroyed or updated
	// 3. carry out deployment
	return nil
}

func (c *defaultBlueprintContainer) Destroy(
	ctx context.Context,
	instanceID string,
	channels *DestroyChannels,
	paramOverrides core.BlueprintParams,
) error {
	return nil
}

func (c *defaultBlueprintContainer) SpecLinkInfo() links.SpecLinkInfo {
	return c.linkInfo
}

func (c *defaultBlueprintContainer) BlueprintSpec() speccore.BlueprintSpec {
	return c.spec
}

func (c *defaultBlueprintContainer) Diagnostics() []*core.Diagnostic {
	return c.diagnostics
}

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
	ResourceName    string
	Removed         bool
	New             bool
	Changes         provider.Changes
	ResolveOnDeploy []string
}

// ChildChangesMessage provides a message containing the changes
// that will be made to a child blueprint in a blueprint instance.
type ChildChangesMessage struct {
	ChildBlueprintName string
	Removed            bool
	New                bool
	Changes            BlueprintChanges
}

// LinkChangesMessage provides a message containing the changes
// that will be made to a link between resources in a blueprint instance.
type LinkChangesMessage struct {
	ResourceAName string
	ResourceBName string
	Removed       bool
	New           bool
	Changes       provider.LinkChanges
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
	outputChanges *BlueprintChanges
	// Mutex is required as resources can be staged concurrently.
	mu sync.Mutex
}

type linkPendingCompletion struct {
	resourceANode    *links.ChainLinkNode
	resourceBNode    *links.ChainLinkNode
	resourceAPending bool
	resourceBPending bool
	linkPending      bool
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

type DeployChannels struct{}

type DestroyChannels struct{}
