package docgen

import (
	"context"
	"slices"
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

func getProviderDataSourceDocs(
	ctx context.Context,
	namespace string,
	providerPlugin provider.Provider,
	dataSourceType string,
) (*PluginDocsDataSource, error) {
	dataSource, err := providerPlugin.DataSource(ctx, dataSourceType)
	if err != nil {
		return nil, err
	}

	typeInfo, err := dataSource.GetType(
		ctx,
		&provider.DataSourceGetTypeInput{
			ProviderContext: createProviderContext(namespace),
		},
	)
	if err != nil {
		return nil, err
	}

	typeDescriptionOutput, err := dataSource.GetTypeDescription(
		ctx,
		&provider.DataSourceGetTypeDescriptionInput{
			ProviderContext: createProviderContext(namespace),
		},
	)
	if err != nil {
		return nil, err
	}

	examplesOutput, err := dataSource.GetExamples(
		ctx,
		&provider.DataSourceGetExamplesInput{
			ProviderContext: createProviderContext(namespace),
		},
	)
	if err != nil {
		return nil, err
	}

	dataSourceSpec, err := getProviderDataSourceSpecDocs(
		ctx,
		namespace,
		dataSource,
	)
	if err != nil {
		return nil, err
	}

	return &PluginDocsDataSource{
		Type:    typeInfo.Type,
		Label:   typeInfo.Label,
		Summary: getProviderDataSourceSummary(typeDescriptionOutput),
		Description: getProviderDataSourceDescription(
			typeDescriptionOutput,
		),
		Specification: dataSourceSpec,
		Examples: getProviderDataSourceExamples(
			examplesOutput,
		),
	}, nil
}

func getProviderDataSourceSpecDocs(
	ctx context.Context,
	namespace string,
	dataSource provider.DataSource,
) (*PluginDocsDataSourceSpec, error) {
	dataSourceSpecOutput, err := dataSource.GetSpecDefinition(
		ctx,
		&provider.DataSourceGetSpecDefinitionInput{
			ProviderContext: createProviderContext(namespace),
		},
	)
	if err != nil {
		return nil, err
	}

	filterableFieldsOutput, err := dataSource.GetFilterFields(
		ctx,
		&provider.DataSourceGetFilterFieldsInput{
			ProviderContext: createProviderContext(namespace),
		},
	)
	if err != nil {
		return nil, err
	}

	dataSourceSpecFieldDocs := make(
		map[string]*PluginDocsDataSourceFieldSpec,
		len(dataSourceSpecOutput.SpecDefinition.Fields),
	)
	for key, field := range dataSourceSpecOutput.SpecDefinition.Fields {
		dataSourceSpecFieldDocs[key] = toDocsDataSourceFieldSpec(
			key,
			field,
			filterableFieldsOutput.Fields,
		)
	}

	return &PluginDocsDataSourceSpec{
		Fields: dataSourceSpecFieldDocs,
	}, nil
}

func toDocsDataSourceFieldSpec(
	fieldName string,
	field *provider.DataSourceSpecSchema,
	filterableFields []string,
) *PluginDocsDataSourceFieldSpec {
	return &PluginDocsDataSourceFieldSpec{
		Type:        string(field.Type),
		Description: field.Description,
		Nullable:    field.Nullable,
		Filterable:  slices.Contains(filterableFields, fieldName),
	}
}

func getProviderDataSourceSummary(
	output *provider.DataSourceGetTypeDescriptionOutput,
) string {
	if strings.TrimSpace(output.MarkdownSummary) != "" {
		return output.MarkdownSummary
	}

	if strings.TrimSpace(output.PlainTextSummary) != "" {
		return output.PlainTextSummary
	}

	return truncateDescription(getProviderDataSourceDescription(output), 120)
}

func getProviderDataSourceDescription(
	output *provider.DataSourceGetTypeDescriptionOutput,
) string {
	if strings.TrimSpace(output.MarkdownDescription) != "" {
		return output.MarkdownDescription
	}

	return output.PlainTextDescription
}

func getProviderDataSourceExamples(
	output *provider.DataSourceGetExamplesOutput,
) []string {
	if len(output.MarkdownExamples) > 0 {
		return output.MarkdownExamples
	}

	return output.PlainTextExamples
}
