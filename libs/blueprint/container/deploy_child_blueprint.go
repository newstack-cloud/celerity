package container

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/includes"
	"github.com/two-hundred/celerity/libs/blueprint/refgraph"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
)

// ChildBlueprintDeployer provides an interface for a service that deploys a child
// blueprint as a part of the deployment process for a blueprint instance.
type ChildBlueprintDeployer interface {
	Deploy(
		ctx context.Context,
		parentInstanceID string,
		parentInstanceTreePath string,
		includeTreePath string,
		childNode *refgraph.ReferenceChainNode,
		changes *BlueprintChanges,
		deployCtx *DeployContext,
	)
}

// IncludeSubstitutionResolver provides an interface for a service that
// is responsible for resolving substitutions in an include definition.
type IncludeSubstitutionResolver interface {
	// ResolveInResource resolves substitutions in an include.
	ResolveInInclude(
		ctx context.Context,
		includeName string,
		include *schema.Include,
		resolveTargetInfo *subengine.ResolveIncludeTargetInfo,
	) (*subengine.ResolveInIncludeResult, error)
}

// NewDefaultChildBlueprintDeployer creates a new instance of the default implementation
// of the service that deploys a child blueprint as a part of the deployment process
// for a blueprint instance.
func NewDefaultChildBlueprintDeployer(
	substitutionResolver IncludeSubstitutionResolver,
	childResolver includes.ChildResolver,
	createChildBlueprintLoader ChildBlueprintLoaderFactory,
	stateContainer state.Container,
) ChildBlueprintDeployer {
	return &defaultChildBlueprintDeployer{
		substitutionResolver:       substitutionResolver,
		childResolver:              childResolver,
		createChildBlueprintLoader: createChildBlueprintLoader,
		stateContainer:             stateContainer,
	}
}

type defaultChildBlueprintDeployer struct {
	substitutionResolver       IncludeSubstitutionResolver
	childResolver              includes.ChildResolver
	createChildBlueprintLoader ChildBlueprintLoaderFactory
	stateContainer             state.Container
}

func (d *defaultChildBlueprintDeployer) Deploy(
	ctx context.Context,
	parentInstanceID string,
	parentInstanceTreePath string,
	includeTreePath string,
	childNode *refgraph.ReferenceChainNode,
	changes *BlueprintChanges,
	deployCtx *DeployContext,
) {
	loadResult, err := loadChildBlueprint(
		ctx,
		&childBlueprintLoadInput{
			parentInstanceID:       parentInstanceID,
			parentInstanceTreePath: parentInstanceTreePath,
			instanceTreePath:       childNode.ElementName,
			includeTreePath:        includeTreePath,
			node:                   childNode,
			resolveFor:             subengine.ResolveForDeployment,
		},
		d.substitutionResolver,
		d.childResolver,
		d.createChildBlueprintLoader,
		d.stateContainer,
		deployCtx.ParamOverrides,
	)
	if err != nil {
		deployCtx.Channels.ErrChan <- err
		return
	}

	childChannels := &DeployChannels{
		ResourceUpdateChan:   make(chan ResourceDeployUpdateMessage),
		LinkUpdateChan:       make(chan LinkDeployUpdateMessage),
		ChildUpdateChan:      make(chan ChildDeployUpdateMessage),
		DeploymentUpdateChan: make(chan DeploymentUpdateMessage),
		FinishChan:           make(chan DeploymentFinishedMessage),
		ErrChan:              make(chan error),
	}
	err = loadResult.childContainer.Deploy(
		ctx,
		&DeployInput{
			InstanceID: loadResult.childState.InstanceID,
			Changes:    changes,
			Rollback:   deployCtx.Rollback,
		},
		childChannels,
		loadResult.childParams,
	)
	if err != nil {
		deployCtx.Channels.ErrChan <- err
		return
	}

	d.waitForChildDeployment(
		ctx,
		parentInstanceID,
		loadResult.includeName,
		loadResult.childState,
		childChannels,
		deployCtx,
	)
}

func (d *defaultChildBlueprintDeployer) waitForChildDeployment(
	ctx context.Context,
	parentInstanceID string,
	childName string,
	childState *state.InstanceState,
	childChannels *DeployChannels,
	deployCtx *DeployContext,

) {
	childBlueprintElement := &ChildBlueprintIDInfo{
		ChildInstanceID: childState.InstanceID,
		ChildName:       childName,
	}

	finished := false
	var err error
	for !finished && err == nil {
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case msg := <-childChannels.DeploymentUpdateChan:
			deployCtx.Channels.ChildUpdateChan <- updateToChildUpdateMessage(
				&msg,
				parentInstanceID,
				childBlueprintElement,
				deployCtx.CurrentGroupIndex,
			)
		case msg := <-childChannels.FinishChan:
			deployCtx.Channels.ChildUpdateChan <- finishedToChildUpdateMessage(
				&msg,
				parentInstanceID,
				childBlueprintElement,
				deployCtx.CurrentGroupIndex,
			)
			finished = true
		case msg := <-childChannels.ResourceUpdateChan:
			deployCtx.Channels.ResourceUpdateChan <- msg
		case msg := <-childChannels.LinkUpdateChan:
			deployCtx.Channels.LinkUpdateChan <- msg
		case msg := <-childChannels.ChildUpdateChan:
			deployCtx.Channels.ChildUpdateChan <- msg
		case err = <-childChannels.ErrChan:
			deployCtx.Channels.ErrChan <- err
		}
	}
}
