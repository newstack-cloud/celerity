package plugintestsuites

import (
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/plugin-framework/internal/testutils"
)

func customVarTypeGetTypeInput() *provider.CustomVariableTypeGetTypeInput {
	return &provider.CustomVariableTypeGetTypeInput{
		ProviderContext: testutils.CreateTestProviderContext("aws"),
	}
}

func customVarTypeGetDescriptionInput() *provider.CustomVariableTypeGetDescriptionInput {
	return &provider.CustomVariableTypeGetDescriptionInput{
		ProviderContext: testutils.CreateTestProviderContext("aws"),
	}
}
