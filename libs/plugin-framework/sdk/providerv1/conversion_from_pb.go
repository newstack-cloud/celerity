package providerv1

import (
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/serialisation"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/plugin-framework/convertv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/pbutils"
	"github.com/two-hundred/celerity/libs/plugin-framework/providerserverv1"
)

func fromPBCustomValidateResourceRequest(
	req *providerserverv1.CustomValidateResourceRequest,
) (*provider.ResourceValidateInput, error) {
	resource, err := serialisation.FromResourcePB(req.SchemaResource)
	if err != nil {
		return nil, err
	}

	providerCtx, err := convertv1.FromPBProviderContext(req.Context)
	if err != nil {
		return nil, err
	}

	return &provider.ResourceValidateInput{
		SchemaResource:  resource,
		ProviderContext: providerCtx,
	}, nil
}

func fromPBGetResourceExternalStateRequest(
	req *providerserverv1.GetResourceExternalStateRequest,
) (*provider.ResourceGetExternalStateInput, error) {
	providerCtx, err := convertv1.FromPBProviderContext(req.Context)
	if err != nil {
		return nil, err
	}

	currentResourceSpec, err := serialisation.FromMappingNodePB(
		req.CurrentResourceSpec,
		/* optional */ true,
	)
	if err != nil {
		return nil, err
	}

	currentResourceMetadata, err := convertv1.FromPBResourceMetadataState(req.CurrentResourceMetadata)
	if err != nil {
		return nil, err
	}

	return &provider.ResourceGetExternalStateInput{
		InstanceID:              req.InstanceId,
		ResourceID:              req.ResourceId,
		CurrentResourceSpec:     currentResourceSpec,
		CurrentResourceMetadata: currentResourceMetadata,
		ProviderContext:         providerCtx,
	}, nil
}

func fromPBStageLinkChangesRequest(
	req *providerserverv1.StageLinkChangesRequest,
) (*provider.LinkStageChangesInput, error) {

	resourceAChanges, err := convertv1.FromPBResourceChanges(req.ResourceAChanges)
	if err != nil {
		return nil, err
	}

	resourceBChanges, err := convertv1.FromPBResourceChanges(req.ResourceBChanges)
	if err != nil {
		return nil, err
	}

	currentLinkState, err := fromPBLinkState(req.CurrentLinkState)
	if err != nil {
		return nil, err
	}

	linkContext, err := fromPBLinkContext(req.Context)
	if err != nil {
		return nil, err
	}

	return &provider.LinkStageChangesInput{
		ResourceAChanges: resourceAChanges,
		ResourceBChanges: resourceBChanges,
		CurrentLinkState: currentLinkState,
		LinkContext:      linkContext,
	}, nil
}

func fromPBLinkState(
	pbLinkState *providerserverv1.LinkState,
) (*state.LinkState, error) {
	intermediaryResourceStates, err := fromPBLinkIntermediaryResourceStates(
		pbLinkState.IntermediaryResourceStates,
	)
	if err != nil {
		return nil, err
	}

	data, err := convertv1.FromPBMappingNodeMap(pbLinkState.Data)
	if err != nil {
		return nil, err
	}

	return &state.LinkState{
		LinkID:     pbLinkState.Id,
		Name:       pbLinkState.Name,
		InstanceID: pbLinkState.InstanceId,
		Status:     core.LinkStatus(pbLinkState.Status),
		PreciseStatus: core.PreciseLinkStatus(
			pbLinkState.PreciseStatus,
		),
		LastStatusUpdateTimestamp:  int(pbLinkState.LastStatusUpdateTimestamp),
		LastDeployedTimestamp:      int(pbLinkState.LastDeployedTimestamp),
		LastDeployAttemptTimestamp: int(pbLinkState.LastDeployAttemptTimestamp),
		IntermediaryResourceStates: intermediaryResourceStates,
		Data:                       data,
		FailureReasons:             pbLinkState.FailureReasons,
		Durations:                  fromPBLinkCompletionDurations(pbLinkState.Durations),
	}, nil
}

func fromPBLinkIntermediaryResourceStates(
	pbIntermediaryResourceStates []*providerserverv1.LinkIntermediaryResourceState,
) ([]*state.LinkIntermediaryResourceState, error) {
	intermediaryResourceStates := make(
		[]*state.LinkIntermediaryResourceState,
		0,
		len(pbIntermediaryResourceStates),
	)
	for _, pbIntermediaryResourceState := range pbIntermediaryResourceStates {
		intermediaryResourceState, err := fromPBLinkIntermediaryResourceState(
			pbIntermediaryResourceState,
		)
		if err != nil {
			return nil, err
		}
		intermediaryResourceStates = append(
			intermediaryResourceStates,
			intermediaryResourceState,
		)
	}
	return intermediaryResourceStates, nil
}

func fromPBLinkIntermediaryResourceState(
	pbIntermediaryResourceState *providerserverv1.LinkIntermediaryResourceState,
) (*state.LinkIntermediaryResourceState, error) {
	resourceSpecData, err := serialisation.FromMappingNodePB(
		pbIntermediaryResourceState.ResourceSpecData,
		/* optional */ true,
	)
	if err != nil {
		return nil, err
	}
	return &state.LinkIntermediaryResourceState{
		ResourceID: pbIntermediaryResourceState.ResourceId,
		InstanceID: pbIntermediaryResourceState.InstanceId,
		Status:     core.ResourceStatus(pbIntermediaryResourceState.Status),
		PreciseStatus: core.PreciseResourceStatus(
			pbIntermediaryResourceState.PreciseStatus,
		),
		LastDeployedTimestamp:      int(pbIntermediaryResourceState.LastDeployedTimestamp),
		LastDeployAttemptTimestamp: int(pbIntermediaryResourceState.LastDeployAttemptTimestamp),
		ResourceSpecData:           resourceSpecData,
		FailureReasons:             pbIntermediaryResourceState.FailureReasons,
	}, nil
}

func fromPBLinkCompletionDurations(
	pbDurations *providerserverv1.LinkCompletionDurations,
) *state.LinkCompletionDurations {
	if pbDurations == nil {
		return nil
	}

	return &state.LinkCompletionDurations{
		ResourceAUpdate: fromPBLinkComponentCompletionDurations(pbDurations.ResourceAUpdate),
		ResourceBUpdate: fromPBLinkComponentCompletionDurations(pbDurations.ResourceBUpdate),
		IntermediaryResources: fromPBLinkComponentCompletionDurations(
			pbDurations.IntermediaryResources,
		),
		TotalDuration: pbutils.DoublePtrFromPBWrapper(pbDurations.TotalDuration),
	}
}

func fromPBLinkComponentCompletionDurations(
	pbComponentDurations *providerserverv1.LinkComponentCompletionDurations,
) *state.LinkComponentCompletionDurations {
	if pbComponentDurations == nil {
		return nil
	}

	return &state.LinkComponentCompletionDurations{
		TotalDuration: pbutils.DoublePtrFromPBWrapper(
			pbComponentDurations.TotalDuration,
		),
		AttemptDurations: pbComponentDurations.AttemptDurations,
	}
}

func fromPBLinkContext(
	pbLinkContext *providerserverv1.LinkContext,
) (provider.LinkContext, error) {
	providerConfigVars, err := convertv1.FromPBScalarMap(pbLinkContext.ProviderConfigVariables)
	if err != nil {
		return nil, err
	}

	contextVars, err := convertv1.FromPBScalarMap(pbLinkContext.ContextVariables)
	if err != nil {
		return nil, err
	}

	return createLinkContextFromVarMaps(providerConfigVars, contextVars)
}
