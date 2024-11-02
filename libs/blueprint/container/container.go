package container

import (
	"context"
	"fmt"
	"sync"

	"github.com/two-hundred/celerity/libs/blueprint/core"
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
	// Parameter overrides can be provided to add extra instance-specific variables
	// that were not defined when the container was loaded or to provide all variables
	// when the container was loaded with an empty set.
	StageChanges(ctx context.Context, instanceID string, paramOverrides core.BlueprintParams) (*BlueprintChanges, error)
	// Deploy deals with deploying the blueprint for the given instance ID.
	// Deploying a blueprint involves creating, updating and destroying resources
	// based on the staged changes.
	// Deploy should also be used as the mechanism to rollback a blueprint to a previous
	// revision managed in version control or a data store for blueprint source documents.
	Deploy(ctx context.Context, instanceID string, changes *BlueprintChanges, paramOverrides core.BlueprintParams) (string, error)
	// Destroy deals with destroying all the resources and links
	// for a blueprint instance.
	Destroy(ctx context.Context, instanceID string, paramOverrides core.BlueprintParams) error
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
	ResourceChanges map[string]*provider.Changes `json:"resourceChanges"`
	ChildChanges    map[string]*BlueprintChanges `json:"childChanges"`
}

// UpdateEventType provides a convenience alias
// to allow us to distinguish between deployment
// and change staging update events.
type UpdateEventType string

const (
	// DeployUpdateEvent is the event update type
	// for deployments.
	DeployUpdateEvent UpdateEventType = "deploy"
	// StageChangesUpdateEvent is the event update type
	// for staging changes.
	StageChangesUpdateEvent UpdateEventType = "stageChanges"
)

// Update holds an update to be sent to a caller
// representing an updating in staging changes or deploying
// a blueprint instance.
type Update struct {
	EventType     UpdateEventType
	Description   string
	ResourceName  string
	ResourceType  string
	ResourceState state.ResourceState
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
	resourceCache                  *core.Cache[[]*provider.ResolvedResource]
	resourceTemplateInputElemCache *core.Cache[[]*core.MappingNode]
	diagnostics                    []*core.Diagnostic
	// The channel to send deployment and change-staging updates to.
	updateChan chan Update
}

// BlueprintContainerDependencies provides the dependencies
// required to create a new instance of a blueprint container.
type BlueprintContainerDependencies struct {
	StateContainer       state.Container
	ResourceProviders    map[string]provider.Provider
	LinkInfo             links.SpecLinkInfo
	RefChainCollector    validation.RefChainCollector
	SubstitutionResolver subengine.SubstitutionResolver
	ChangeStager         ResourceChangeStager
	ResourceCache        *core.Cache[[]*provider.ResolvedResource]
}

// NewDefaultBlueprintContainer creates a new instance of the default
// implementation of a blueprint container for a loaded spec.
// The map of resource providers must be a map of provider resource name
// to a provider.
func NewDefaultBlueprintContainer(
	spec speccore.BlueprintSpec,
	deps *BlueprintContainerDependencies,
	diagnostics []*core.Diagnostic,
	updateChan chan Update,
) BlueprintContainer {
	return &defaultBlueprintContainer{
		deps.StateContainer,
		deps.ResourceProviders,
		spec,
		deps.LinkInfo,
		deps.RefChainCollector,
		deps.SubstitutionResolver,
		deps.ChangeStager,
		deps.ResourceCache,
		core.NewCache[[]*core.MappingNode](),
		diagnostics,
		updateChan,
	}
}

func (c *defaultBlueprintContainer) StageChanges(
	ctx context.Context,
	instanceID string,
	paramOverrides core.BlueprintParams,
) (*BlueprintChanges, error) {
	// instanceState, err := c.getInstanceState(ctx, instanceID)
	// if err != nil {
	// 	return nil, err
	// }

	// Collect changes from child blueprints before collecting changes from resources
	// childBlueprintChanges := map[string]*BlueprintChanges{}
	// for childName, childState := range instanceState.ChildBlueprints {
	// 	childBlueprintContainer := NewDefaultBlueprintContainer(
	// 		c.spec,
	// 		&BlueprintContainerDependencies{
	// 			StateContainer:       c.stateContainer,
	// 			ResourceProviders:    c.resourceProviders,
	// 			LinkInfo:             c.linkInfo,
	// 			RefChainCollector:    c.refChainCollector,
	// 			SubstitutionResolver: c.substitutionResolver,
	// 			ChangeStager:         c.changeStager,
	// 			ResourceCache:        c.resourceCache,
	// 		},
	// 		c.diagnostics,
	// 		c.updateChan,
	// 	)
	// 	// childParams := c.createParamsForChildBlueprint(paramOverrides, childVariables)
	// 	childChanges, err := childBlueprintContainer.StageChanges(ctx, childState.InstanceID, paramOverrides)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	childBlueprintChanges[childName] = childChanges
	// }

	chains, err := c.linkInfo.Links(ctx)
	if err != nil {
		return nil, err
	}
	childrenRefNodes := extractChildRefNodes(c.spec.Schema(), c.refChainCollector)
	_, err = OrderItemsForDeployment(ctx, chains, childrenRefNodes, c.refChainCollector, paramOverrides)
	if err != nil {
		return nil, err
	}
	// parallelGroups, err := GroupOrderedLinkNodes(ctx, orderedNodes, c.refChainCollector, paramOverrides)
	// if err != nil {
	// 	return nil, err
	// }
	parallelGroups := [][]*links.ChainLinkNode{}

	state := &stageChangesState{}
	for _, group := range parallelGroups {
		changesChan := make(chan *resourceChangesMessage)
		linkChangesChan := make(chan *linkChangesMessage)
		errChan := make(chan error)
		channels := &stageResourceChangeChannels{
			changesChan,
			linkChangesChan,
			errChan,
		}
		c.stageResourceGroupChanges(
			ctx,
			instanceID,
			state,
			group,
			paramOverrides,
			channels,
		)

		collected := map[string]*provider.Changes{}
		var err error
		for len(collected) < len(group) && err == nil {
			select {
			case changes := <-changesChan:
				collected[changes.resourceName] = changes.changes
			case err = <-errChan:
			}
		}

		if err != nil {
			return nil, err
		}
	}

	// If persisted

	return nil, nil
}

func (c *defaultBlueprintContainer) getInstanceState(ctx context.Context, instanceID string) (*state.InstanceState, error) {
	var instanceStatePtr *state.InstanceState
	instanceState, err := c.stateContainer.GetInstance(ctx, instanceID)
	if err != nil {
		if !state.IsInstanceNotFound(err) {
			return nil, err
		}
	} else {
		instanceStatePtr = &instanceState
	}
	return instanceStatePtr, nil
}

func (c *defaultBlueprintContainer) stageResourceGroupChanges(
	ctx context.Context,
	instanceID string,
	state *stageChangesState,
	group []*links.ChainLinkNode,
	paramOverrides core.BlueprintParams,
	channels *stageResourceChangeChannels,
) {
	for _, node := range group {
		go c.stageResourceChanges(
			ctx,
			instanceID,
			state,
			node,
			paramOverrides,
			channels,
		)
	}
}

func (c *defaultBlueprintContainer) stageResourceChanges(
	ctx context.Context,
	instanceID string,
	stagingState *stageChangesState,
	node *links.ChainLinkNode,
	paramOverrides core.BlueprintParams,
	channels *stageResourceChangeChannels,
) {
	resourceProvider, hasResourceProvider := c.resourceProviders[node.ResourceName]
	if !hasResourceProvider {
		channels.errChan <- fmt.Errorf("no provider found for resource %q", node.ResourceName)
		return
	}

	resourceImplementation, err := resourceProvider.Resource(ctx, node.Resource.Type.Value)
	if err != nil {
		channels.errChan <- err
		return
	}

	items, err := c.substitutionResolver.ResolveResourceEach(ctx, node.ResourceName, node.Resource)
	if err != nil {
		channels.errChan <- err
		return
	}

	if len(items) == 0 {
		err := c.stageIndividualResourceChanges(
			ctx,
			&stageResourceChangeInfo{
				node:       node,
				instanceID: instanceID,
				index:      0,
			},
			resourceImplementation,
			paramOverrides,
			channels.changesChan,
			stagingState,
		)
		if err != nil {
			channels.errChan <- err
			return
		}
		return
	}

	c.cacheResourceTemplateInputElements(node.ResourceName, items)
	for index := range items {

		resourceID, err := c.getResourceID(ctx, instanceID, node.ResourceName, index)
		if err != nil {
			channels.errChan <- err
			return
		}

		err = c.stageIndividualResourceChanges(
			ctx,
			&stageResourceChangeInfo{
				node:       node,
				instanceID: instanceID,
				resourceID: resourceID,
				index:      index,
			},
			resourceImplementation,
			paramOverrides,
			channels.changesChan,
			stagingState,
		)
		if err != nil {
			channels.errChan <- err
			return
		}
	}
}

func (c *defaultBlueprintContainer) getResourceID(
	ctx context.Context,
	instanceID string,
	resourceName string,
	index int,
) (string, error) {
	instanceState, err := c.stateContainer.GetInstance(ctx, instanceID)
	if err != nil {
		return "", err
	}

	resourceIDs, hasResourceIDs := instanceState.ResourceIDs[resourceName]
	if hasResourceIDs && len(resourceIDs) > index {
		return resourceIDs[index], nil
	}

	// This resource does not exist in the state, it will be created
	// when the changes are deployed.
	return "", nil
}

func (c *defaultBlueprintContainer) stageIndividualResourceChanges(
	ctx context.Context,
	resourceInfo *stageResourceChangeInfo,
	resourceImplementation provider.Resource,
	paramOverrides core.BlueprintParams,
	changesChan chan *resourceChangesMessage,
	stagingState *stageChangesState,
) error {
	node := resourceInfo.node
	resolvedResource, err := c.substitutionResolver.ResolveInResource(
		ctx,
		node.ResourceName,
		node.Resource,
		resourceInfo.index,
	)
	if err != nil {
		return err
	}

	var currentResourceStatePtr *state.ResourceState
	currentResourceState, err := c.stateContainer.GetResource(
		ctx,
		resourceInfo.instanceID,
		resourceInfo.resourceID,
	)
	if err != nil {
		if !state.IsResourceNotFound(err) {
			return err
		}
	} else {
		currentResourceStatePtr = &currentResourceState
	}

	changes, err := c.changeStager.StageChanges(ctx, &provider.ResourceInfo{
		ResourceID:               resourceInfo.resourceID,
		InstanceID:               resourceInfo.instanceID,
		CurrentResourceState:     currentResourceStatePtr,
		ResourceWithResolvedSubs: resolvedResource,
	})
	if err != nil {
		return err
	}

	changesChan <- &resourceChangesMessage{
		resourceName: node.ResourceName,
		index:        resourceInfo.index,
		changes:      changes,
	}

	linksReadyToBeStaged := c.updateStagingState(node, stagingState)

	err = c.stageLinkChanges(ctx, node, linksReadyToBeStaged)
	if err != nil {
		return err
	}

	return nil
}

func (c *defaultBlueprintContainer) stageLinkChanges(
	ctx context.Context,
	node *links.ChainLinkNode,
	linksReadyToBeStaged []*linkPendingCompletion,
) error {
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
		linkID := fmt.Sprintf("%s::%s", node.ResourceName, linksToNode.ResourceName)
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
		linkID := fmt.Sprintf("%s::%s", linkedFromNode.ResourceName, node.ResourceName)
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

func (c *defaultBlueprintContainer) cacheResourceTemplateInputElements(
	resourceName string,
	items []*core.MappingNode,
) {
	c.resourceTemplateInputElemCache.Set(resourceName, items)
}

func (c *defaultBlueprintContainer) Deploy(
	ctx context.Context,
	instanceID string,
	changes *BlueprintChanges,
	paramOverrides core.BlueprintParams,
) (string, error) {
	// 1. get chain links
	// 2. traverse through chains and order resources to be created, destroyed or updated
	// 3. carry out deployment
	return "", nil
}

func (c *defaultBlueprintContainer) Destroy(ctx context.Context, instanceID string, paramOverrides core.BlueprintParams) error {
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

type stageResourceChangeChannels struct {
	changesChan     chan *resourceChangesMessage
	linkChangesChan chan *linkChangesMessage
	errChan         chan error
}

type resourceChangesMessage struct {
	resourceName string
	index        int
	removed      bool
	new          bool
	changes      *provider.Changes
}

type linkChangesMessage struct {
	resourceAName string
	resourceBName string
	removed       bool
	new           bool
	changes       *provider.LinkChanges
}

type stageChangesState struct {
	// A mapping of a link ID to the pending link completion state.
	// A link ID in this context is made up of the resource names of the two resources
	// that are linked together.
	// For example, if resource A is linked to resource B, the link ID would be "A::B".
	pendingLinks map[string]*linkPendingCompletion
	// A mapping of resource names to pending links that include the resource.
	resourceNameLinkMap map[string][]string
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
	index      int
}
