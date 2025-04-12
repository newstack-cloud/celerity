package plugintestsuites

import (
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

func createDeployResourceChanges() *provider.Changes {
	return &provider.Changes{
		AppliedResourceInfo: createDeployResourceInfo(),
		MustRecreate:        false,
		ModifiedFields: []provider.FieldChange{
			{
				FieldPath: "spec.functionName",
				PrevValue: core.MappingNodeFromString("Process-Order-Function-0"),
				NewValue:  core.MappingNodeFromString("Process-Order-Function-0--Updated"),
			},
		},
		NewFields:                 []provider.FieldChange{},
		RemovedFields:             []string{},
		UnchangedFields:           []string{},
		ComputedFields:            []string{"spec.arn"},
		FieldChangesKnownOnDeploy: []string{},
		ConditionKnownOnDeploy:    false,
		NewOutboundLinks:          map[string]provider.LinkChanges{},
		OutboundLinkChanges:       map[string]provider.LinkChanges{},
		RemovedOutboundLinks:      []string{},
	}
}

func createDeployResourceInfo() provider.ResourceInfo {
	lastDriftDetected := 1742389743
	configCompleteDuration := 8.5
	totalDuration := 69.5
	return provider.ResourceInfo{
		ResourceID:   testResource1ID,
		ResourceName: testResource1Name,
		InstanceID:   testInstance1ID,
		CurrentResourceState: &state.ResourceState{
			ResourceID:                 testResource1ID,
			Name:                       testResource1Name,
			Type:                       lambdaFunctionResourceType,
			TemplateName:               "processOrderFunction",
			InstanceID:                 testInstance1ID,
			Status:                     core.ResourceStatusCreated,
			PreciseStatus:              core.PreciseResourceStatusCreated,
			LastStatusUpdateTimestamp:  1742389743,
			LastDeployedTimestamp:      1742389743,
			LastDeployAttemptTimestamp: 1742389743,
			Description:                "Lambda function for processing orders",
			SpecData: &core.MappingNode{
				Fields: map[string]*core.MappingNode{
					"arn":          core.MappingNodeFromString("arn:aws:lambda:us-west-2:123456789012:function:processOrderFunction_0"),
					"functionName": core.MappingNodeFromString("Process-Order-Function-0"),
				},
			},
			Metadata: &state.ResourceMetadataState{
				DisplayName: "Process Order Function",
				Annotations: map[string]*core.MappingNode{
					"example.annotation.v1": core.MappingNodeFromString("example-value"),
				},
				Labels: map[string]string{
					"app": "orders",
				},
				Custom: &core.MappingNode{
					Fields: map[string]*core.MappingNode{
						"customField": core.MappingNodeFromString("customValue"),
					},
				},
			},
			DependsOnResources:         []string{"orderQueue"},
			DependsOnChildren:          []string{"coreInfra"},
			Drifted:                    true,
			LastDriftDetectedTimestamp: &lastDriftDetected,
			Durations: &state.ResourceCompletionDurations{
				ConfigCompleteDuration: &configCompleteDuration,
				TotalDuration:          &totalDuration,
				AttemptDurations:       []float64{10.5, 20.5, 30.5},
			},
		},
		ResourceWithResolvedSubs: &provider.ResolvedResource{
			Type: &schema.ResourceTypeWrapper{
				Value: lambdaFunctionResourceType,
			},
			Description: core.MappingNodeFromString("Lambda function for processing orders"),
			Spec: &core.MappingNode{
				Fields: map[string]*core.MappingNode{
					"functionName": core.MappingNodeFromString("Process-Order-Function-0--Updated"),
				},
			},
			Metadata: &provider.ResolvedResourceMetadata{
				DisplayName: core.MappingNodeFromString("Process Order Function"),
				Annotations: &core.MappingNode{
					Fields: map[string]*core.MappingNode{
						"example.annotation.v1": core.MappingNodeFromString("example-value-updated"),
					},
				},
				Labels: &schema.StringMap{
					Values: map[string]string{
						"app": "orders",
					},
				},
				Custom: &core.MappingNode{
					Fields: map[string]*core.MappingNode{
						"customField": core.MappingNodeFromString("customValueUpdated"),
					},
				},
			},
			Condition: &provider.ResolvedResourceCondition{
				StringValue: core.MappingNodeFromBool(true),
			},
			LinkSelector: &schema.LinkSelector{
				ByLabel: &schema.StringMap{
					Values: map[string]string{
						"app": "orders",
					},
				},
			},
		},
	}
}
