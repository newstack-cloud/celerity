package container

import (
	"context"
	"sync"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/includes"
	"github.com/two-hundred/celerity/libs/blueprint/links"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/resourcehelpers"
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
	// RefChainCollector returns the reference chain collector used to collect
	// reference chains for resources and child blueprints.
	// This is useful for allowing parent contexts to get access to the collected
	// references for a blueprint.
	// For example, extracting the references from an expanded version of a blueprint
	// that contains resource templates.
	RefChainCollector() validation.RefChainCollector
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
	// NewExports contains new fields that will be exported from the blueprint.
	NewExports map[string]provider.FieldChange `json:"newExports"`
	// ExportChanges contains changes to exported fields.
	ExportChanges map[string]provider.FieldChange `json:"exportChanges"`
	// UnchangedExports contains the names of fields that will not be changed.
	UnchangedExports []string `json:"unchangedExports"`
	// RemovedExports contains the names of fields that will no longer be exported.
	RemovedExports []string `json:"removedExports"`
	// ResolveOnDeploy contains paths to properties in blueprint elements
	// that contain substitutions that can not be resolved until the blueprint
	// is deployed.
	// This includes properties in resources and exported fields.
	ResolveOnDeploy []string `json:"resolveOnDeploy"`
}

// NewBlueprintDefinition provides a definition for a new child blueprint
// that will be created when deploying a blueprint instance.
type NewBlueprintDefinition struct {
	NewResources map[string]provider.Changes       `json:"newResources"`
	NewChildren  map[string]NewBlueprintDefinition `json:"newChildren"`
	NewExports   map[string]provider.FieldChange   `json:"newExports"`
}

type defaultBlueprintContainer struct {
	stateContainer state.Container
	// Should be a namespace to provider map.
	providers                      map[string]provider.Provider
	resourceRegistry               resourcehelpers.Registry
	spec                           speccore.BlueprintSpec
	linkInfo                       links.SpecLinkInfo
	refChainCollector              validation.RefChainCollector
	substitutionResolver           subengine.SubstitutionResolver
	changeStager                   ResourceChangeStager
	childResolver                  includes.ChildResolver
	resourceCache                  *core.Cache[*provider.ResolvedResource]
	resourceTemplateInputElemCache *core.Cache[[]*core.MappingNode]
	childExportFieldCache          *core.Cache[*subengine.ChildExportFieldInfo]
	diagnostics                    []*core.Diagnostic
	createChildBlueprintLoader     func(derivedFromTemplate []string) Loader
}

// BlueprintContainerDependencies provides the dependencies
// required to create a new instance of a blueprint container.
type BlueprintContainerDependencies struct {
	StateContainer                 state.Container
	Providers                      map[string]provider.Provider
	ResourceRegistry               resourcehelpers.Registry
	LinkInfo                       links.SpecLinkInfo
	RefChainCollector              validation.RefChainCollector
	SubstitutionResolver           subengine.SubstitutionResolver
	ChangeStager                   ResourceChangeStager
	ChildResolver                  includes.ChildResolver
	ResourceCache                  *core.Cache[*provider.ResolvedResource]
	ResourceTemplateInputElemCache *core.Cache[[]*core.MappingNode]
	ChildExportFieldCache          *core.Cache[*subengine.ChildExportFieldInfo]
	ChildBlueprintLoaderFactory    func(derivedFromTemplate []string) Loader
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
		deps.Providers,
		deps.ResourceRegistry,
		spec,
		deps.LinkInfo,
		deps.RefChainCollector,
		deps.SubstitutionResolver,
		deps.ChangeStager,
		deps.ChildResolver,
		deps.ResourceCache,
		deps.ResourceTemplateInputElemCache,
		deps.ChildExportFieldCache,
		diagnostics,
		deps.ChildBlueprintLoaderFactory,
	}
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

func (c *defaultBlueprintContainer) RefChainCollector() validation.RefChainCollector {
	return c.refChainCollector
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
	// ConditionKnownOnDeploy is used to indicate that the condition for the resource
	// can not be resolved until the blueprint is deployed.
	// This means the changes described in this message may not be applied
	// if the condition evaluates to false when the blueprint is deployed.
	ConditionKnownOnDeploy bool
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
	// This is an intermediary format that holds pointers to resource change sets to allow
	// modification without needing to copy and patch resource change sets back in to the state
	// each time resource change set state needs to be updated with link change sets.
	outputChanges *intermediaryBlueprintChanges
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
