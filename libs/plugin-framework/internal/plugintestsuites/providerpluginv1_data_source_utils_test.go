package plugintestsuites

import (
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	"github.com/two-hundred/celerity/libs/plugin-framework/internal/testutils"
)

func dataSourceValidateInput() *provider.DataSourceValidateInput {
	exampleAnnotationValue := "example-annotation-value"
	searchFor := "search-for"
	description := "example field description"
	return &provider.DataSourceValidateInput{
		SchemaDataSource: &schema.DataSource{
			Type: &schema.DataSourceTypeWrapper{
				Value: vpcDataSourceType,
			},
			DataSourceMetadata: &schema.DataSourceMetadata{
				Annotations: &schema.StringOrSubstitutionsMap{
					Values: map[string]*substitutions.StringOrSubstitutions{
						"example.annotation": {
							Values: []*substitutions.StringOrSubstitution{
								{
									StringValue: &exampleAnnotationValue,
								},
							},
						},
					},
				},
			},
			Filter: &schema.DataSourceFilter{
				Field: core.ScalarFromString("examplefield"),
				Operator: &schema.DataSourceFilterOperatorWrapper{
					Value: schema.DataSourceFilterOperatorEquals,
				},
				Search: &schema.DataSourceFilterSearch{
					Values: []*substitutions.StringOrSubstitutions{
						{
							Values: []*substitutions.StringOrSubstitution{
								{
									StringValue: &searchFor,
								},
							},
						},
					},
				},
			},
			Exports: &schema.DataSourceFieldExportMap{
				Values: map[string]*schema.DataSourceFieldExport{
					"example": {
						Type: &schema.DataSourceFieldTypeWrapper{
							Value: schema.DataSourceFieldTypeString,
						},
						AliasFor: core.ScalarFromString("examplefield"),
						Description: &substitutions.StringOrSubstitutions{
							Values: []*substitutions.StringOrSubstitution{
								{
									StringValue: &description,
								},
							},
						},
					},
				},
			},
		},
		ProviderContext: testutils.CreateTestProviderContext("aws"),
	}
}

func dataSourceGetTypeInput() *provider.DataSourceGetTypeInput {
	return &provider.DataSourceGetTypeInput{
		ProviderContext: testutils.CreateTestProviderContext("aws"),
	}
}
