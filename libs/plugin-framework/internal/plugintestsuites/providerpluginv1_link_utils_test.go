package plugintestsuites

import (
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/plugin-framework/internal/testutils"
)

func linkStageChangesInput() *provider.LinkStageChangesInput {
	return &provider.LinkStageChangesInput{
		ResourceAChanges: createDeployResourceChanges(),
		ResourceBChanges: createDeployResourceChanges(),
		CurrentLinkState: createCurrentLinkState(),
		LinkContext:      testutils.CreateTestLinkContext(),
	}
}

func createCurrentLinkState() *state.LinkState {
	resourceAUpdateDuration := 8.5
	resourceBUpdateDuration := 12.5
	intermediaryResourcesUpdateDuraition := 18.2
	return &state.LinkState{
		LinkID:                     testLinkID,
		Name:                       testLinkName,
		InstanceID:                 testInstance1ID,
		Status:                     core.LinkStatusCreated,
		PreciseStatus:              core.PreciseLinkStatusIntermediaryResourcesUpdated,
		LastStatusUpdateTimestamp:  1742389743,
		LastDeployedTimestamp:      1742389743,
		LastDeployAttemptTimestamp: 1742389743,
		IntermediaryResourceStates: []*state.LinkIntermediaryResourceState{},
		Data: map[string]*core.MappingNode{
			testResource1Name: {
				Fields: map[string]*core.MappingNode{
					"environmentVariables": {
						Fields: map[string]*core.MappingNode{
							"TABLE_NAME_ordersTable": core.MappingNodeFromString("orders"),
						},
					},
				},
			},
		},
		Durations: &state.LinkCompletionDurations{
			ResourceAUpdate: &state.LinkComponentCompletionDurations{
				TotalDuration:    &resourceAUpdateDuration,
				AttemptDurations: []float64{resourceAUpdateDuration},
			},
			ResourceBUpdate: &state.LinkComponentCompletionDurations{
				TotalDuration:    &resourceBUpdateDuration,
				AttemptDurations: []float64{resourceBUpdateDuration},
			},
			IntermediaryResources: &state.LinkComponentCompletionDurations{
				TotalDuration:    &intermediaryResourcesUpdateDuraition,
				AttemptDurations: []float64{intermediaryResourcesUpdateDuraition},
			},
		},
	}
}
