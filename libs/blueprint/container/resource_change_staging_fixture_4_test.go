package container

import (
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

func (s *ResourceChangeStagerTestSuite) resourceInfoFixture4() *provider.ResourceInfo {

	return &provider.ResourceInfo{
		ResourceID:           "test-resource-4",
		InstanceID:           "test-instance-1",
		ResourceName:         "complexResource",
		CurrentResourceState: s.resourceInfoFixture4CurrentState(),
		// Reuse the example complex resource as the new spec for the resource.
		ResourceWithResolvedSubs: s.resourceInfoFixture1NewResolvedResource(),
	}
}

func (s *ResourceChangeStagerTestSuite) resourceInfoFixture4CurrentState() *state.ResourceState {
	return &state.ResourceState{
		ResourceID:   "test-resource-1",
		ResourceName: "complexResource",
		// Resource type is being updated from "example/old-complex" to "example/complex"
		ResourceType:               "example/old-complex",
		Status:                     core.ResourceStatusCreated,
		PreciseStatus:              core.PreciseResourceStatusCreated,
		LastDeployedTimestamp:      1732969676,
		LastDeployAttemptTimestamp: 1732969676,
		ResourceSpecData:           &core.MappingNode{},
		Metadata:                   &state.ResourceMetadataState{},
	}
}
