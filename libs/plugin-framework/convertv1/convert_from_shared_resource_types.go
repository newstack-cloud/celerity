package convertv1

import (
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/plugin-framework/sharedtypesv1"
)

// FromPBDeployResourceCompleteResponse converts a DeployResourceCompleteResponse
// from a protobuf message to a type comptabile with the blueprint framework.
func FromPBDeployResourceCompleteResponse(
	response *sharedtypesv1.DeployResourceCompleteResponse,
) (*provider.ResourceDeployOutput, error) {
	if response == nil {
		return nil, nil
	}

	computedFieldValues, err := FromPBMappingNodeMap(response.ComputedFieldValues)
	if err != nil {
		return nil, err
	}

	return &provider.ResourceDeployOutput{
		ComputedFieldValues: computedFieldValues,
	}, nil
}
