package container

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/links"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
)

// ResourceChangeStager provides an interface for a service that stages changes for
// a resource in a blueprint.
type ResourceChangeStager interface {
	StageChanges(
		ctx context.Context,
		instanceID string,
		stagingState ChangeStagingState,
		node *links.ChainLinkNode,
		channels *ChangeStagingChannels,
		resourceProviders map[string]provider.Provider,
		params core.BlueprintParams,
	)
}

// NewDefaultResourceChangeStager creates a new instance of the
// default resource change stager.
func NewDefaultResourceChangeStager(
	substitutionResolver subengine.SubstitutionResolver,
	resourceCache *core.Cache[*provider.ResolvedResource],
	stateContainer state.Container,
	changeGenerator ResourceChangeGenerator,
	linkChangeStager LinkChangeStager,
) ResourceChangeStager {
	return &defaultResourceChangeStager{
		substitutionResolver: substitutionResolver,
		resourceCache:        resourceCache,
		stateContainer:       stateContainer,
		changeGenerator:      changeGenerator,
		linkChangeStager:     linkChangeStager,
	}
}

type defaultResourceChangeStager struct {
	substitutionResolver subengine.SubstitutionResolver
	resourceCache        *core.Cache[*provider.ResolvedResource]
	stateContainer       state.Container
	changeGenerator      ResourceChangeGenerator
	linkChangeStager     LinkChangeStager
}

func (s *defaultResourceChangeStager) StageChanges(
	ctx context.Context,
	instanceID string,
	stagingState ChangeStagingState,
	node *links.ChainLinkNode,
	channels *ChangeStagingChannels,
	resourceProviders map[string]provider.Provider,
	params core.BlueprintParams,
) {
	resourceImplementation, err := getProviderResourceImplementation(
		ctx,
		node.ResourceName,
		node.Resource.Type.Value,
		resourceProviders,
	)
	if err != nil {
		channels.ErrChan <- err
		return
	}

	err = s.stageChanges(
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

func (s *defaultResourceChangeStager) stageChanges(
	ctx context.Context,
	stageResourceInfo *stageResourceChangeInfo,
	resourceImplementation provider.Resource,
	changesChan chan ResourceChangesMessage,
	linkChangesChan chan LinkChangesMessage,
	stagingState ChangeStagingState,
	params core.BlueprintParams,
) error {

	resourceInfo, resolveResourceResult, err := getResourceInfo(
		ctx,
		stageResourceInfo,
		s.substitutionResolver,
		s.resourceCache,
		s.stateContainer,
	)
	if err != nil {
		return err
	}

	changes, err := s.changeGenerator.GenerateChanges(
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
		changes.MustRecreate = stagingState.MustRecreateResourceOnRemovedDependencies(
			resourceInfo.ResourceName,
		)
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
	stagingState.ApplyResourceChanges(changesMsg)
	linksReadyToBeStaged := stagingState.UpdateLinkStagingState(stageResourceInfo.node)

	err = s.prepareAndStageLinkChanges(
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

func (s *defaultResourceChangeStager) prepareAndStageLinkChanges(
	ctx context.Context,
	currentResourceInfo *provider.ResourceInfo,
	linksReadyToBeStaged []*LinkPendingCompletion,
	linkChangesChan chan LinkChangesMessage,
	stagingState ChangeStagingState,
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
		err = s.linkChangeStager.StageChanges(
			ctx,
			linkImpl,
			currentResourceInfo,
			readyToStage,
			stagingState,
			linkChangesChan,
			params,
		)
		if err != nil {
			return err
		}
	}

	return nil
}
