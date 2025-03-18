package testprovider

import (
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/plugin-framework/sdk/providerv1"
)

func dataSourceVPC() provider.DataSource {
	return &providerv1.DataSourceDefinition{
		Type:  "aws/vpc",
		Label: "AWS Virtual Private Cloud",
	}
}
