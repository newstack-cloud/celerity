package awsrunner

import (
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/core"
	"github.com/newstack-cloud/bluelink/libs/blueprint/state"
	"github.com/newstack-cloud/celerity/apps/cli/internal/maintenancerunner"
)

func runnerResourceWithARN(arn string) *state.ResourceState {
	return &state.ResourceState{
		Type: lambdaFunctionType,
		Metadata: &state.ResourceMetadataState{
			Annotations: map[string]*core.MappingNode{
				maintenancerunner.RunnerAnnotation: core.MappingNodeFromString(
					maintenancerunner.RunnerAnnotationValue,
				),
			},
		},
		SpecData: core.MappingNodeFields("arn", core.MappingNodeFromString(arn)),
	}
}

// A deploy config naming a region used to be ignored, so `celerity schema apply` failed
// with "Missing Region" unless AWS_REGION happened to be exported. The runner's own ARN
// records where it is deployed.
func Test_region_comes_from_the_runners_arn(t *testing.T) {
	instance := &state.InstanceState{
		Resources: map[string]*state.ResourceState{
			"runner": runnerResourceWithARN(
				"arn:aws:lambda:eu-west-2:027653366630:function:audit-maintenance",
			),
		},
	}

	if got := runnerRegion(instance); got != "eu-west-2" {
		t.Errorf("got %q, want eu-west-2", got)
	}
	if opts := runnerRegionOption(instance); len(opts) != 1 {
		t.Errorf("expected one load option, got %d", len(opts))
	}
}

// Without a usable ARN the default chain must be left alone rather than pinned to "".
func Test_no_region_leaves_the_default_chain_untouched(t *testing.T) {
	cases := map[string]*state.InstanceState{
		"nil instance": nil,
		"no runner":    {Resources: map[string]*state.ResourceState{}},
		"empty arn": {Resources: map[string]*state.ResourceState{
			"runner": runnerResourceWithARN(""),
		}},
		"malformed arn": {Resources: map[string]*state.ResourceState{
			"runner": runnerResourceWithARN("audit-maintenance"),
		}},
	}

	for name, instance := range cases {
		if got := runnerRegion(instance); got != "" {
			t.Errorf("%s: got %q, want empty", name, got)
		}
		if opts := runnerRegionOption(instance); opts != nil {
			t.Errorf("%s: expected no load options", name)
		}
	}
}
