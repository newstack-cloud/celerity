package providerserverv1

import (
	"fmt"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	schemapb "github.com/two-hundred/celerity/libs/blueprint/schemapb"
	"github.com/two-hundred/celerity/libs/blueprint/serialisation"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/plugin-framework/convertv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/pbutils"
	sharedtypesv1 "github.com/two-hundred/celerity/libs/plugin-framework/sharedtypesv1"
)

func toPBLinkState(linkState *state.LinkState) (*LinkState, error) {
	intermediaryResourceStates, err := toPBLinkIntermediaryResourceStates(
		linkState.IntermediaryResourceStates,
	)
	if err != nil {
		return nil, err
	}

	pbData, err := convertv1.ToPBMappingNodeMap(linkState.Data)
	if err != nil {
		return nil, err
	}

	return &LinkState{
		Id:         linkState.LinkID,
		Name:       linkState.Name,
		InstanceId: linkState.InstanceID,
		Status:     LinkStatus(linkState.Status),
		PreciseStatus: PreciseLinkStatus(
			linkState.PreciseStatus,
		),
		LastStatusUpdateTimestamp:  int64(linkState.LastStatusUpdateTimestamp),
		LastDeployedTimestamp:      int64(linkState.LastDeployedTimestamp),
		LastDeployAttemptTimestamp: int64(linkState.LastDeployAttemptTimestamp),
		IntermediaryResourceStates: intermediaryResourceStates,
		Data:                       pbData,
		FailureReasons:             linkState.FailureReasons,
		Durations:                  toPBLinkCompletionDurations(linkState.Durations),
	}, nil
}

func toPBLinkCompletionDurations(
	durations *state.LinkCompletionDurations,
) *LinkCompletionDurations {
	if durations == nil {
		return nil
	}

	return &LinkCompletionDurations{
		ResourceAUpdate: toPBLinkComponentCompletionDurations(
			durations.ResourceAUpdate,
		),
		ResourceBUpdate: toPBLinkComponentCompletionDurations(
			durations.ResourceBUpdate,
		),
		IntermediaryResources: toPBLinkComponentCompletionDurations(
			durations.IntermediaryResources,
		),
		TotalDuration: pbutils.DoublePtrToPBWrapper(
			durations.TotalDuration,
		),
	}
}

func toPBLinkComponentCompletionDurations(
	componentDurations *state.LinkComponentCompletionDurations,
) *LinkComponentCompletionDurations {
	if componentDurations == nil {
		return nil
	}

	return &LinkComponentCompletionDurations{
		TotalDuration: pbutils.DoublePtrToPBWrapper(
			componentDurations.TotalDuration,
		),
		AttemptDurations: componentDurations.AttemptDurations,
	}
}

func toPBLinkIntermediaryResourceStates(
	intermediaryResourceStates []*state.LinkIntermediaryResourceState,
) ([]*LinkIntermediaryResourceState, error) {
	pbIntermediaryResourceStates := make([]*LinkIntermediaryResourceState, 0, len(intermediaryResourceStates))
	for _, intermediaryResourceState := range intermediaryResourceStates {
		pbIntermediaryResourceState, err := toPBLinkIntermediaryResourceState(
			intermediaryResourceState,
		)
		if err != nil {
			return nil, err
		}

		pbIntermediaryResourceStates = append(
			pbIntermediaryResourceStates,
			pbIntermediaryResourceState,
		)
	}

	return pbIntermediaryResourceStates, nil
}

func toPBLinkIntermediaryResourceState(
	intermediaryResourceState *state.LinkIntermediaryResourceState,
) (*LinkIntermediaryResourceState, error) {
	pbResourceSpecData, err := serialisation.ToMappingNodePB(
		intermediaryResourceState.ResourceSpecData,
		/* optional */ true,
	)
	if err != nil {
		return nil, err
	}

	return &LinkIntermediaryResourceState{
		ResourceId: intermediaryResourceState.ResourceID,
		InstanceId: intermediaryResourceState.InstanceID,
		Status:     sharedtypesv1.ResourceStatus(intermediaryResourceState.Status),
		PreciseStatus: sharedtypesv1.PreciseResourceStatus(
			intermediaryResourceState.PreciseStatus,
		),
		LastDeployedTimestamp: int64(intermediaryResourceState.LastDeployedTimestamp),
		LastDeployAttemptTimestamp: int64(
			intermediaryResourceState.LastDeployAttemptTimestamp,
		),
		ResourceSpecData: pbResourceSpecData,
	}, nil
}

func toPBLinkChanges(
	changes *provider.LinkChanges,
) (*sharedtypesv1.LinkChanges, error) {
	if changes == nil {
		return nil, nil
	}

	changesPB, err := convertv1.ToPBLinkChanges(*changes)
	if err != nil {
		return nil, err
	}

	return changesPB, nil
}

func toPBLinkContext(linkCtx provider.LinkContext) (*LinkContext, error) {
	providerConfigVars, err := toPBLinkContextProviderConfigVars(
		linkCtx.AllProviderConfigVariables(),
	)
	if err != nil {
		return nil, err
	}

	contextVars, err := convertv1.ToPBScalarMap(linkCtx.ContextVariables())
	if err != nil {
		return nil, err
	}

	return &LinkContext{
		ProviderConfigVariables: providerConfigVars,
		ContextVariables:        contextVars,
	}, nil
}

func toPBLinkContextProviderConfigVars(
	allProviderConfigVars map[string]map[string]*core.ScalarValue,
) (map[string]*schemapb.ScalarValue, error) {
	providerConfigVars := make(map[string]*schemapb.ScalarValue)

	for providerName, configVars := range allProviderConfigVars {
		for key, value := range configVars {
			pbValue, err := serialisation.ToScalarValuePB(
				value,
				/* optional */ true,
			)
			if err != nil {
				return nil, err
			}

			namespacedKey := fmt.Sprintf("%s::%s", providerName, key)
			providerConfigVars[namespacedKey] = pbValue
		}
	}

	return providerConfigVars, nil
}
