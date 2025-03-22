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
	descriptionInfo := DataSourceVPCTypeDescriptionOutput()
	examples := DataSourceVPCExamplesOutput()
	specDefOutput := DataSourceVPCSpecDefinitionOutput()
	filterFieldsOutput := DataSourceVPCFilterFieldsOutput()
	return &providerv1.DataSourceDefinition{
		Type:                 "aws/vpc",
		Label:                "AWS Virtual Private Cloud",
		CustomValidateFunc:   customValidateDataSourceVPC,
		PlainTextSummary:     descriptionInfo.PlainTextSummary,
		FormattedSummary:     descriptionInfo.MarkdownSummary,
		PlainTextDescription: descriptionInfo.PlainTextDescription,
		FormattedDescription: descriptionInfo.MarkdownDescription,
		PlainTextExamples:    examples.PlainTextExamples,
		MarkdownExamples:     examples.MarkdownExamples,
		FieldSchemas:         specDefOutput.Fields,
		FilterFields:         filterFieldsOutput.Fields,
		FetchFunc:            fetchDataSourceVPC,
	}
}

func DataSourceVPCTypeDescriptionOutput() *provider.DataSourceGetTypeDescriptionOutput {
	return &provider.DataSourceGetTypeDescriptionOutput{
		PlainTextSummary:     "This is a plain text summary of the vpc data source",
		MarkdownSummary:      "This is a **formatted** summary of the vpc data source",
		PlainTextDescription: "This is a plain text description of the vpc data source",
		MarkdownDescription:  "This is a **formatted** description of the vpc data source",
	}
}

func DataSourceVPCExamplesOutput() *provider.DataSourceGetExamplesOutput {
	return &provider.DataSourceGetExamplesOutput{
		PlainTextExamples: []string{
			"This is a plain text example of the vpc data source",
		},
		MarkdownExamples: []string{
			"This is a **formatted** example of the vpc data source",
		},
	}
}

func DataSourceVPCSpecDefinitionOutput() *provider.DataSourceSpecDefinition {
	return &provider.DataSourceSpecDefinition{
		Fields: map[string]*provider.DataSourceSpecSchema{
			"example": {
				Type:                 provider.DataSourceSpecTypeString,
				Label:                "Example Field",
				Description:          "This is an example field",
				FormattedDescription: "This is a **formatted** description of the example field",
				Nullable:             true,
			},
			"exampleArray": {
				Type:                 provider.DataSourceSpecTypeArray,
				Label:                "Example Array Field",
				Description:          "This is an example array field",
				FormattedDescription: "This is a **formatted** description of the example array field",
				Nullable:             true,
				Items: &provider.DataSourceSpecSchema{
					Type: provider.DataSourceSpecTypeString,
				},
			},
		},
	}
}

func DataSourceVPCFilterFieldsOutput() *provider.DataSourceGetFilterFieldsOutput {
	return &provider.DataSourceGetFilterFieldsOutput{
		Fields: []string{
			"example",
			"exampleArray",
		},
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

func fetchDataSourceVPC(
	ctx context.Context,
	input *provider.DataSourceFetchInput,
) (*provider.DataSourceFetchOutput, error) {
	return DataSourceVPCFetchOutput(), nil
}

func DataSourceVPCFetchOutput() *provider.DataSourceFetchOutput {
	return &provider.DataSourceFetchOutput{
		Data: map[string]*core.MappingNode{
			"exampleSource": core.MappingNodeFromString("exampleSourceValue"),
		},
	}
}
