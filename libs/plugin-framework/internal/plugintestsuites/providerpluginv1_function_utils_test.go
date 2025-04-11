package plugintestsuites

import (
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/plugin-framework/internal/testutils"
)

func functionGetDefinitionInput() *provider.FunctionGetDefinitionInput {
	return &provider.FunctionGetDefinitionInput{
		Params: testutils.CreateEmptyTestParams(),
	}
}
