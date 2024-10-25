package container

import (
	"context"

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
	// This will create a new revision and destroy the previous revision once the new revision
	// has been successfully deployed.
	// This returns the revision ID of the newly deployed instance revision upon
	// successful deployment.
	Deploy(ctx context.Context, instanceID string) (string, error)
	// Destroy deals with destroying all the resources and links
	// for a revision of a blueprint instance.
	Destroy(ctx context.Context, instanceID string, revisionID string) error
	// Rollback deals with rolling a blueprint instance back to a previous revision.
	// This will destroy any new resources that were created as a part of the revision
	// that is being rolled back.
	Rollback(ctx context.Context, instanceID string, revisionIDToRollback string, prevRevisionID string) error
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
	resourceProviders map[string]provider.Provider
	spec              speccore.BlueprintSpec
	linkInfo          links.SpecLinkInfo
	refChainCollector validation.RefChainCollector
	diagnostics       []*core.Diagnostic
	// The channel to send deployment and change-staging updates to.
	updateChan chan Update
}

// NewDefaultBlueprintContainer creates a new instance of the default
// implementation of a blueprint container for a loaded spec.
// The map of resource providers must be a map of provider resource name
// to a provider.
func NewDefaultBlueprintContainer(
	stateContainer state.Container,
	resourceProviders map[string]provider.Provider,
	spec speccore.BlueprintSpec,
	linkInfo links.SpecLinkInfo,
	refChainCollector validation.RefChainCollector,
	diagnostics []*core.Diagnostic,
	updateChan chan Update,
) BlueprintContainer {
	return &defaultBlueprintContainer{
		stateContainer,
		resourceProviders,
		spec,
		linkInfo,
		refChainCollector,
		diagnostics,
		updateChan,
	}
}

func (c *defaultBlueprintContainer) StageChanges(
	ctx context.Context, instanceID string,
	paramOverrides core.BlueprintParams,
) (BlueprintChanges, error) {
	// chains, err := c.linkInfo.Links(ctx)
	// if err != nil {
	// 	return nil, err
	// }
	// orderedLinkNodes, err := OrderLinksForDeployment(ctx, chains, c.refChainCollector, paramOverrides)
	// if err != nil {
	// 	return nil, err
	// }
	// parallelGroups, err := GroupOrderedLinkNodes(ctx, orderedLinkNodes, c.refChainCollector, paramOverrides)
	// if err != nil {
	// 	return nil, err
	// }

	// state := &stageChangesState{}
	// for _, group := range parallelGroups {
	// 	// If resources do not depend on each other, they can be staged concurrently.
	// }

	return nil, nil
}

func (c *defaultBlueprintContainer) Deploy(ctx context.Context, instanceID string) (string, error) {
	// 1. get chain links
	// 2. traverse through chains and order resources to be created, destroyed or updated
	// 3. carry out deployment
	// 4. upon success, destroy any remaining resources from the previous revision
	return "", nil
}

func (c *defaultBlueprintContainer) Rollback(ctx context.Context, instanceID string, revisionIDToRollback string, prevRevisionID string) error {
	return nil
}

func (c *defaultBlueprintContainer) Destroy(ctx context.Context, instanceID string, revisionID string) error {
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

// type stageChangesState struct {
// 	pendingLinks map[string]*linkPendingCompletion
// 	// Mutex is required as resources can be staged concurrently.
// 	mu sync.Mutex
// }

// type linkPendingCompletion struct {
// 	link             *links.ChainLinkNode
// 	resourceAPending bool
// 	resourceBPending bool
// 	linkPending      bool
// }
