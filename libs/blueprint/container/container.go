package container

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/links"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/refgraph"
	"github.com/two-hundred/celerity/libs/blueprint/resourcehelpers"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/speccore"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
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
		input *StageChangesInput,
		channels *ChangeStagingChannels,
		paramOverrides core.BlueprintParams,
	) error
	// Deploy deals with deploying the blueprint for the given instance ID.
	// When an instance ID is omitted, the container will treat the deployment
	// as a new instance of the blueprint where the provided change set only includes
	// new resources, children and links.
	// Deploying a blueprint involves creating, updating and destroying resources
	// based on the staged changes.
	// This will stream updates to the provided channels for each resource, child blueprint and link
	// that has either been updated, created or removed.
	// Deploy should also be used as the mechanism to rollback a blueprint to a previous
	// revision managed in version control or a data store for blueprint source documents.
	//
	// There is synchronous and asynchronous error handling, synchronous errors will be returned
	// during the initial setup phase of the deployment process.
	// Most errors should be handled by the container and sent to the appropriate channel
	// as a deployment update message.
	Deploy(
		ctx context.Context,
		input *DeployInput,
		channels *DeployChannels,
		paramOverrides core.BlueprintParams,
	) error
	// Destroy deals with destroying all the resources, child blueprints and links
	// for a blueprint instance.
	// Like Deploy, Destroy requires changes to be staged and passed in to ensure that
	// the user gets a chance to review everything that will be destroyed before
	// starting the process; this should go hand and hand with a confirmation step and prompts
	// to allow the user to dig deeper in the tools built on top of the framework.
	// This will stream updates to the provided channels for each resource, child blueprint and link
	// that has been removed.
	//
	// There is no synchronous error handling, all errors will be sent to the provided error
	// channel. Most errors should be handled by the container and sent to the appropriate channel
	// as an update message.
	Destroy(
		ctx context.Context,
		input *DestroyInput,
		channels *DeployChannels,
		paramOverrides core.BlueprintParams,
	)
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
	RefChainCollector() refgraph.RefChainCollector
	// ResourceTemplates holds a mapping of resource names to the name of the resource
	// template it was derived from.
	// This allows retention of information about the original resource template
	// that a resource was derived from in a source blueprint document.
	ResourceTemplates() map[string]string
	// Diagnostics returns warning and informational diagnostics for the loaded blueprint
	// that point out potential issues that may occur when executing
	// a blueprint.
	// These diagnostics can contain errors, however, the error returned on failure to load a blueprint
	// should also be unpacked to get the precise location and information about the reason loading the
	// blueprint failed.
	Diagnostics() []*core.Diagnostic
}

// StageChangesInput contains the primary input needed to stage changes
// for a blueprint instance.
type StageChangesInput struct {
	// InstanceID is the ID of the blueprint instance that the changes will be applied to.
	InstanceID string
	// Destroy is used to indicate that the changes being staged should be for a destroy operation.
	// If this is set to true, the change set will be generated for removal all components
	// in the current state of the blueprint instance.
	Destroy bool
}

// DeployInput contains the primary input needed to deploy a blueprint instance.
type DeployInput struct {
	// InstanceID is the ID of the blueprint instance that the changes will be deployed for.
	InstanceID string
	// Changes contains the changes that will be made to the blueprint instance.
	Changes *BlueprintChanges
	// Rollback is used to indicate that the changes being deployed represent a rollback.
	// This is useful for ensuring the correct statuses are applied when changes within a child
	// blueprint need to be rolled back due to a failure in the parent blueprint.
	// The loaded blueprint is expected to be the version of the blueprint to roll back to
	// for a given blueprint instance.
	Rollback bool
}

// DestroyInput contains the primary input needed to destroy a blueprint instance.
type DestroyInput struct {
	// InstanceID is the ID of the blueprint instance that will be destroyed.
	InstanceID string
	// Changes contains a description of all the elements that need to be
	// removed when destroying the blueprint instance.
	Changes *BlueprintChanges
	// Rollback is used to indicate that the blueprint instance is being destroyed
	// as part of a rollback operation.
	// This is useful for ensuring the correct statuses are applied when changes within a child
	// blueprint need to be rolled back due to a failure in the parent blueprint.
	Rollback bool
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
	// RecreateChildren contains the name of the child blueprints that will be recreated
	// when deploying the changes.
	// The reason for this will primarily be due to a dependency of a child blueprint
	// being removed from the latest version of the host blueprint.
	RecreateChildren []string `json:"recreateChildren"`
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
	// MetadataChanges contains changes to blueprint-wide metadata.
	MetadataChanges MetadataChanges `json:"metadataChanges"`
	// ResolveOnDeploy contains paths to properties in blueprint elements
	// that contain substitutions that can not be resolved until the blueprint
	// is deployed.
	// This includes properties in resources, data sources, blueprint-wide metadata
	// and exported fields.
	ResolveOnDeploy []string `json:"resolveOnDeploy"`
}

type MetadataChanges struct {
	// NewFields contains new fields that will be added to the blueprint-wide metadata.
	NewFields []provider.FieldChange `json:"newFields"`
	// ModifiedFields contains changes to existing fields in the blueprint-wide metadata.
	ModifiedFields []provider.FieldChange `json:"modifiedFields"`
	// UnchangedFields contains the names of fields that will not be changed.
	UnchangedFields []string `json:"unchangedFields"`
	// RemovedFields contains the names of fields that will no longer be present.
	RemovedFields []string `json:"removedFields"`
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
	providers                map[string]provider.Provider
	resourceRegistry         resourcehelpers.Registry
	linkRegistry             provider.LinkRegistry
	spec                     speccore.BlueprintSpec
	linkInfo                 links.SpecLinkInfo
	resourceTemplates        map[string]string
	refChainCollector        refgraph.RefChainCollector
	substitutionResolver     subengine.SubstitutionResolver
	changeStager             ResourceChangeStager
	diagnostics              []*core.Diagnostic
	clock                    core.Clock
	idGenerator              core.IDGenerator
	createDeploymentState    func() DeploymentState
	createChangeStagingState func() ChangeStagingState
	blueprintPreparer        BlueprintPreparer
	linkChangeStager         LinkChangeStager
	childChangeStager        ChildChangeStager
	resourceDestroyer        ResourceDestroyer
	childBlueprintDestroyer  ChildBlueprintDestroyer
	linkDestroyer            LinkDestroyer
	linkDeployer             LinkDeployer
	resourceDeployer         ResourceDeployer
	childDeployer            ChildBlueprintDeployer
	defaultRetryPolicy       *provider.RetryPolicy
	logger                   core.Logger
}

// ChildBlueprintLoaderFactory provides a factory function for creating a new loader
// for child or derived blueprints.
type ChildBlueprintLoaderFactory func(
	derivedFromTemplate []string,
	resourceTemplates map[string]string,
) Loader

// DeploymentStateFactory provides a factory function for creating a new instance
// of a deployment state that is used as an ephemeral store for tracking the state
// of a deployment operation.
type DeploymentStateFactory func() DeploymentState

// ChangeStagingStateFactory provides a factory function for creating a new instance
// of a change staging state that is used as an ephemeral store for tracking the state
// of a change staging operation.
type ChangeStagingStateFactory func() ChangeStagingState

// BlueprintContainerDependencies provides the dependencies
// required to create a new instance of a blueprint container.
type BlueprintContainerDependencies struct {
	StateContainer            state.Container
	Providers                 map[string]provider.Provider
	ResourceRegistry          resourcehelpers.Registry
	LinkRegistry              provider.LinkRegistry
	LinkInfo                  links.SpecLinkInfo
	ResourceTemplates         map[string]string
	RefChainCollector         refgraph.RefChainCollector
	SubstitutionResolver      subengine.SubstitutionResolver
	ChangeStager              ResourceChangeStager
	Clock                     core.Clock
	IDGenerator               core.IDGenerator
	DeploymentStateFactory    DeploymentStateFactory
	ChangeStagingStateFactory ChangeStagingStateFactory
	BlueprintPreparer         BlueprintPreparer
	LinkChangeStager          LinkChangeStager
	ChildChangeStager         ChildChangeStager
	ResourceDestroyer         ResourceDestroyer
	ChildBlueprintDestroyer   ChildBlueprintDestroyer
	LinkDestroyer             LinkDestroyer
	LinkDeployer              LinkDeployer
	ResourceDeployer          ResourceDeployer
	ChildBlueprintDeployer    ChildBlueprintDeployer
	DefaultRetryPolicy        *provider.RetryPolicy
	Logger                    core.Logger
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
		deps.LinkRegistry,
		spec,
		deps.LinkInfo,
		deps.ResourceTemplates,
		deps.RefChainCollector,
		deps.SubstitutionResolver,
		deps.ChangeStager,
		diagnostics,
		deps.Clock,
		deps.IDGenerator,
		deps.DeploymentStateFactory,
		deps.ChangeStagingStateFactory,
		deps.BlueprintPreparer,
		deps.LinkChangeStager,
		deps.ChildChangeStager,
		deps.ResourceDestroyer,
		deps.ChildBlueprintDestroyer,
		deps.LinkDestroyer,
		deps.LinkDeployer,
		deps.ResourceDeployer,
		deps.ChildBlueprintDeployer,
		deps.DefaultRetryPolicy,
		deps.Logger,
	}
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

func (c *defaultBlueprintContainer) RefChainCollector() refgraph.RefChainCollector {
	return c.refChainCollector
}

func (c *defaultBlueprintContainer) ResourceTemplates() map[string]string {
	return c.resourceTemplates
}

func (c *defaultBlueprintContainer) resolveExport(
	ctx context.Context,
	exportName string,
	export *schema.Export,
	resolveFor subengine.ResolveForStage,
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
				ResolveFor: resolveFor,
			},
		)
	}

	return nil, nil
}
