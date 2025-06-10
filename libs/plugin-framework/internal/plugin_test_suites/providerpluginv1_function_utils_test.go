package plugintestsuites

import (
	"github.com/newstack-cloud/celerity/libs/blueprint/provider"
	"github.com/newstack-cloud/celerity/libs/plugin-framework/internal/testutils"
)

func functionGetDefinitionInput() *provider.FunctionGetDefinitionInput {
	return &provider.FunctionGetDefinitionInput{
		Params: testutils.CreateEmptyTestParams(),
	}
}
