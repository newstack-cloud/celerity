package container

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/links"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
)

// LinkChangeStager provides an interface for a service that stages changes for a link
// between two resources.
type LinkChangeStager interface {
	StageChanges(
		ctx context.Context,
		linkImpl provider.Link,
		currentResourceInfo *provider.ResourceInfo,
		readyToStage *LinkPendingCompletion,
		changeStagingState ChangeStagingState,
		linkChangesChan chan LinkChangesMessage,
		params core.BlueprintParams,
	) error
}

// NewDefaultLinkChangeStager creates a new instance of the default implementation
// of the service that stages changes for a link between two resources.
func NewDefaultLinkChangeStager(
	stateContainer state.Container,
	substitutionResolver subengine.SubstitutionResolver,
	resourceCache *core.Cache[*provider.ResolvedResource],
) LinkChangeStager {
	return &defaultLinkChangeStager{
		stateContainer:       stateContainer,
		substitutionResolver: substitutionResolver,
		resourceCache:        resourceCache,
	}
}

type defaultLinkChangeStager struct {
	stateContainer       state.Container
	substitutionResolver subengine.SubstitutionResolver
	resourceCache        *core.Cache[*provider.ResolvedResource]
}

func (d *defaultLinkChangeStager) StageChanges(
	ctx context.Context,
	linkImpl provider.Link,
	currentResourceInfo *provider.ResourceInfo,
	readyToStage *LinkPendingCompletion,
	changeStagingState ChangeStagingState,
	linkChangesChan chan LinkChangesMessage,
	params core.BlueprintParams,
) error {
	resourceAInfo, err := d.getResourceInfoForLink(ctx, readyToStage.resourceANode, currentResourceInfo)
	if err != nil {
		return err
	}

	resourceBInfo, err := d.getResourceInfoForLink(ctx, readyToStage.resourceBNode, currentResourceInfo)
	if err != nil {
		return err
	}

	var currentLinkStatePtr *state.LinkState
	links := d.stateContainer.Links()
	currentLinkState, err := links.GetByName(
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

	resourceAChanges := changeStagingState.GetResourceChanges(resourceAInfo.ResourceName)
	resourceBChanges := changeStagingState.GetResourceChanges(resourceBInfo.ResourceName)

	output, err := linkImpl.StageChanges(ctx, &provider.LinkStageChangesInput{
		ResourceAChanges: resourceAChanges,
		ResourceBChanges: resourceBChanges,
		CurrentLinkState: currentLinkStatePtr,
		Params:           params,
	})
	if err != nil {
		return err
	}

	changeStagingState.MarkLinkAsNoLongerPending(
		readyToStage.resourceANode,
		readyToStage.resourceBNode,
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

func (d *defaultLinkChangeStager) getResourceInfoForLink(
	ctx context.Context,
	node *links.ChainLinkNode,
	currentResourceInfo *provider.ResourceInfo,
) (*provider.ResourceInfo, error) {
	if node.ResourceName != currentResourceInfo.ResourceName {
		resourceInfo, _, err := getResourceInfo(
			ctx,
			&stageResourceChangeInfo{
				node:       node,
				instanceID: currentResourceInfo.InstanceID,
			},
			d.substitutionResolver,
			d.resourceCache,
			d.stateContainer,
		)
		return resourceInfo, err
	}

	return currentResourceInfo, nil
}
