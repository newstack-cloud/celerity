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
	StageChanges(ctx context.Context, instanceID string, paramOverrides core.BlueprintParams) (BlueprintChanges, error)
	// Deploy deals with deploying the blueprint for the given instance ID.
	// Deploying a blueprint involves creating, updating and destroying resources
	// based on the staged changes.
	// Deploy should also be used as the mechanism to rollback a blueprint to a previous
	// revision managed in version control or a data store for blueprint source documents.
	Deploy(ctx context.Context, instanceID string, changes BlueprintChanges, paramOverrides core.BlueprintParams) (string, error)
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

// BlueprintChanges provides a mapping of resource name
// to the changes that will come into effect upon deploying
// the currently loaded version of a blueprint for a given
// instance ID.
// Changes takes the type parameter interface{} as we can't know the exact
// range of resource types for a blueprint at compile time.
// We must check the resource types associated with a set of changes
// at runtime.
type BlueprintChanges map[string]*provider.Changes

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
	substitutionResolver           SubstitutionResolver
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
	SubstitutionResolver SubstitutionResolver
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
) (BlueprintChanges, error) {
	chains, err := c.linkInfo.Links(ctx)
	if err != nil {
		return nil, err
	}
	orderedLinkNodes, err := OrderLinksForDeployment(ctx, chains, c.refChainCollector, paramOverrides)
	if err != nil {
		return nil, err
	}
	parallelGroups, err := GroupOrderedLinkNodes(ctx, orderedLinkNodes, c.refChainCollector, paramOverrides)
	if err != nil {
		return nil, err
	}

	_, err = c.stateContainer.GetInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}

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
	state *stageChangesState,
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
	// instanceState, err := c.stateContainer.GetInstance(ctx, instanceID)
	// if err != nil {
	// 	return "", err
	// }
	// resource, hasResource := instanceState.Resources[resourceName]
	// if !hasResource || len(resource) <= index {
	// 	// This resource does not exist in the state, it will be created
	// 	// when the changes are deployed.
	// 	return "", nil
	// }

	// return resource[index].ResourceID, nil
	return "", nil
}

func (c *defaultBlueprintContainer) stageIndividualResourceChanges(
	ctx context.Context,
	resourceInfo *stageResourceChangeInfo,
	resourceImplementation provider.Resource,
	paramOverrides core.BlueprintParams,
	changesChan chan *resourceChangesMessage,
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

	changesOutput, err := resourceImplementation.StageChanges(ctx, &provider.ResourceStageChangesInput{
		ResourceInfo: &provider.ResourceInfo{
			ResourceID:               resourceInfo.resourceID,
			InstanceID:               resourceInfo.instanceID,
			ResourceWithResolvedSubs: resolvedResource,
		},
		Params: paramOverrides,
	})
	if err != nil {
		return err
	}

	changesChan <- &resourceChangesMessage{
		resourceName: node.ResourceName,
		index:        resourceInfo.index,
		changes:      changesOutput.Changes,
	}

	return nil
}

func (c *defaultBlueprintContainer) cacheResourceTemplateInputElements(resourceName string, items []*core.MappingNode) {
	c.resourceTemplateInputElemCache.Set(resourceName, items)
}

func (c *defaultBlueprintContainer) Deploy(
	ctx context.Context,
	instanceID string,
	changes BlueprintChanges,
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
	pendingLinks map[string]*linkPendingCompletion
	// Mutex is required as resources can be staged concurrently.
	mu sync.Mutex
}

type linkPendingCompletion struct {
	link             *links.ChainLinkNode
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
