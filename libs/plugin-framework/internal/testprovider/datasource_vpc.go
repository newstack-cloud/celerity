package testprovider

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	"github.com/two-hundred/celerity/libs/plugin-framework/sdk/providerv1"
)

func dataSourceVPC() provider.DataSource {
	return &providerv1.DataSourceDefinition{
		Type:               "aws/vpc",
		Label:              "AWS Virtual Private Cloud",
		CustomValidateFunc: customValidateDataSourceVPC,
	}
}

func customValidateDataSourceVPC(
	ctx context.Context,
	input *provider.DataSourceValidateInput,
) (*provider.DataSourceValidateOutput, error) {
	return DataSourceVPCValidateOutput(), nil
}

func DataSourceVPCValidateOutput() *provider.DataSourceValidateOutput {
	colAccuracy := substitutions.ColumnAccuracyExact
	return &provider.DataSourceValidateOutput{
		Diagnostics: []*core.Diagnostic{
			{
				Level:   core.DiagnosticLevelWarning,
				Message: "This is a warning about an invalid vpc data source",
				Range: &core.DiagnosticRange{
					Start: &source.Meta{
						Position: source.Position{
							Line:   120,
							Column: 45,
						},
					},
					End: &source.Meta{
						Position: source.Position{
							Line:   140,
							Column: 89,
						},
					},
					ColumnAccuracy: &colAccuracy,
				},
			},
		},
	}
}
