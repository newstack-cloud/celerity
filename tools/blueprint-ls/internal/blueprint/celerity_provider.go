package blueprint

import (
	"context"
	"errors"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

type celerityProvider struct {
	resources       map[string]provider.Resource
	dataSources     map[string]provider.DataSource
	customVariables map[string]provider.CustomVariableType
}

func NewCelerityProvider() provider.Provider {
	return &celerityProvider{
		resources: map[string]provider.Resource{
			"celerity/handler": &celerityHandlerResource{},
		},
		dataSources: map[string]provider.DataSource{
			"celerity/vpc": &celerityVPCDataSource{},
		},
		customVariables: map[string]provider.CustomVariableType{
			"celerity/customVariable": &celerityCustomVariableType{},
		},
	}
}

func (p *celerityProvider) Namespace(ctx context.Context) (string, error) {
	return "celerity", nil
}

func (p *celerityProvider) ConfigDefinition(ctx context.Context) (*core.ConfigDefinition, error) {
	return nil, nil
}

func (p *celerityProvider) Resource(ctx context.Context, resourceType string) (provider.Resource, error) {
	resource, hasResource := p.resources[resourceType]
	if !hasResource {
		return nil, errors.New("resource not found")
	}

	return resource, nil
}

func (p *celerityProvider) DataSource(ctx context.Context, dataSourceType string) (provider.DataSource, error) {
	dataSource, hasDataSource := p.dataSources[dataSourceType]
	if !hasDataSource {
		return nil, errors.New("data source not found")
	}

	return dataSource, nil
}

func (p *celerityProvider) Link(ctx context.Context, resourceTypeA string, resourceTypeB string) (provider.Link, error) {
	return nil, errors.New("links not implemented")
}

func (p *celerityProvider) CustomVariableType(ctx context.Context, customVariableType string) (provider.CustomVariableType, error) {
	customVarType, hasCustomVarType := p.customVariables[customVariableType]
	if !hasCustomVarType {
		return nil, errors.New("custom variable type not found")
	}

	return customVarType, nil
}

func (p *celerityProvider) ListResourceTypes(ctx context.Context) ([]string, error) {
	resourceTypes := []string{}
	for resourceType := range p.resources {
		resourceTypes = append(resourceTypes, resourceType)
	}

	return resourceTypes, nil
}

func (p *celerityProvider) ListLinkTypes(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (p *celerityProvider) ListDataSourceTypes(ctx context.Context) ([]string, error) {
	dataSourceTypes := []string{}
	for dataSourceType := range p.dataSources {
		dataSourceTypes = append(dataSourceTypes, dataSourceType)
	}

	return dataSourceTypes, nil
}

func (p *celerityProvider) ListCustomVariableTypes(ctx context.Context) ([]string, error) {
	customVariableTypes := []string{}
	for customVariableType := range p.customVariables {
		customVariableTypes = append(customVariableTypes, customVariableType)
	}

	return customVariableTypes, nil
}

func (p *celerityProvider) ListFunctions(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (p *celerityProvider) Function(ctx context.Context, functionName string) (provider.Function, error) {
	return nil, errors.New("functions not implemented")
}

func (p *celerityProvider) RetryPolicy(ctx context.Context) (*provider.RetryPolicy, error) {
	return nil, nil
}

type celerityHandlerResource struct{}

func (r *celerityHandlerResource) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{
		CanLinkTo: []string{},
	}, nil
}

func (r *celerityHandlerResource) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: false,
	}, nil
}

func (r *celerityHandlerResource) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "celerity/handler",
	}, nil
}

func (d *celerityHandlerResource) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		MarkdownDescription: "A resource that represents a handler for a Celerity application.\n\n" +
			"[`celerity/handler` resource docs](https://www.celerityframework.com/docs/resources/celerity-handler)",
		PlainTextDescription: "A resource that represents a handler for a Celerity application.",
	}, nil
}

func (r *celerityHandlerResource) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

func (r *celerityHandlerResource) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return &provider.ResourceGetSpecDefinitionOutput{
		SpecDefinition: &provider.ResourceSpecDefinition{
			Schema: &provider.ResourceDefinitionsSchema{
				Description: "A resource that represents a handler for a Celerity application.",
				Type:        provider.ResourceDefinitionsSchemaTypeObject,
				Attributes: map[string]*provider.ResourceDefinitionsSchema{
					"id": {
						Description: "The ID of the handler in the deployed environment.",
						Type:        provider.ResourceDefinitionsSchemaTypeString,
						Computed:    true,
					},
					"handlerName": {
						Description: "The name of the handler.",
						Type:        provider.ResourceDefinitionsSchemaTypeString,
					},
					"runtime": {
						Description: "The runtime that the handler uses.",
						Type:        provider.ResourceDefinitionsSchemaTypeString,
					},
					"info": {
						Description: "Additional information about the handler.",
						Type:        provider.ResourceDefinitionsSchemaTypeObject,
						Attributes: map[string]*provider.ResourceDefinitionsSchema{
							"applicationId": {
								Description: "The ID of the application that the handler is part of.",
								Type:        provider.ResourceDefinitionsSchemaTypeString,
							},
						},
					},
				},
			},
		},
	}, nil
}

func (r *celerityHandlerResource) GetStabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{
		StabilisedDependencies: []string{},
	}, nil
}

func (r *celerityHandlerResource) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

func (r *celerityHandlerResource) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

func (r *celerityHandlerResource) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}

func (r *celerityHandlerResource) HasStabilised(
	ctx context.Context,
	input *provider.ResourceHasStabilisedInput,
) (*provider.ResourceHasStabilisedOutput, error) {
	return &provider.ResourceHasStabilisedOutput{
		Stabilised: true,
	}, nil
}

func (r *celerityHandlerResource) GetExamples(
	ctx context.Context,
	input *provider.ResourceGetExamplesInput,
) (*provider.ResourceGetExamplesOutput, error) {
	return &provider.ResourceGetExamplesOutput{
		PlainTextExamples: []string{},
		MarkdownExamples:  []string{},
	}, nil
}

type celerityVPCDataSource struct{}

func (d *celerityVPCDataSource) GetSpecDefinition(
	ctx context.Context,
	input *provider.DataSourceGetSpecDefinitionInput,
) (*provider.DataSourceGetSpecDefinitionOutput, error) {
	return &provider.DataSourceGetSpecDefinitionOutput{
		SpecDefinition: &provider.DataSourceSpecDefinition{
			Fields: map[string]*provider.DataSourceSpecSchema{
				"vpcId": {
					Description: "The ID of the VPC.",
					Type:        provider.DataSourceSpecTypeString,
				},
				"subnetIds": {
					Description: "The IDs of subnets in the VPC.",
					Type:        provider.DataSourceSpecTypeArray,
					Items: &provider.DataSourceSpecSchema{
						Description: "The ID of a subnet.",
						Type:        provider.DataSourceSpecTypeString,
					},
				},
			},
		},
	}, nil
}

func (d *celerityVPCDataSource) Fetch(
	ctx context.Context,
	input *provider.DataSourceFetchInput,
) (*provider.DataSourceFetchOutput, error) {
	return &provider.DataSourceFetchOutput{
		Data: map[string]*core.MappingNode{},
	}, nil
}

func (d *celerityVPCDataSource) GetType(
	ctx context.Context,
	input *provider.DataSourceGetTypeInput,
) (*provider.DataSourceGetTypeOutput, error) {
	return &provider.DataSourceGetTypeOutput{
		Type: "test/exampleDataSource",
	}, nil
}

func (d *celerityVPCDataSource) GetTypeDescription(
	ctx context.Context,
	input *provider.DataSourceGetTypeDescriptionInput,
) (*provider.DataSourceGetTypeDescriptionOutput, error) {
	return &provider.DataSourceGetTypeDescriptionOutput{
		MarkdownDescription:  "A data source that pulls in celerity network information.",
		PlainTextDescription: "A data source that pulls in celerity network information.",
	}, nil
}

func (d *celerityVPCDataSource) GetFilterFields(
	ctx context.Context,
	input *provider.DataSourceGetFilterFieldsInput,
) (*provider.DataSourceGetFilterFieldsOutput, error) {
	return &provider.DataSourceGetFilterFieldsOutput{
		Fields: []string{"tags", "vpcId"},
	}, nil
}

func (d *celerityVPCDataSource) CustomValidate(
	ctx context.Context,
	input *provider.DataSourceValidateInput,
) (*provider.DataSourceValidateOutput, error) {
	return &provider.DataSourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

func (r *celerityVPCDataSource) GetExamples(
	ctx context.Context,
	input *provider.DataSourceGetExamplesInput,
) (*provider.DataSourceGetExamplesOutput, error) {
	return &provider.DataSourceGetExamplesOutput{
		PlainTextExamples: []string{},
		MarkdownExamples:  []string{},
	}, nil
}

type celerityCustomVariableType struct{}

func (t *celerityCustomVariableType) Options(
	ctx context.Context,
	input *provider.CustomVariableTypeOptionsInput,
) (*provider.CustomVariableTypeOptionsOutput, error) {
	t2nano := "t2.nano"
	t2micro := "t2.micro"
	t2small := "t2.small"
	t2medium := "t2.medium"
	t2large := "t2.large"
	t2xlarge := "t2.xlarge"
	t22xlarge := "t2.2xlarge"
	return &provider.CustomVariableTypeOptionsOutput{
		Options: map[string]*provider.CustomVariableTypeOption{
			t2nano: {
				Value: &core.ScalarValue{
					StringValue: &t2nano,
				},
			},
			t2micro: {
				Value: &core.ScalarValue{
					StringValue: &t2micro,
				},
			},
			t2small: {
				Value: &core.ScalarValue{
					StringValue: &t2small,
				},
			},
			t2medium: {
				Value: &core.ScalarValue{
					StringValue: &t2medium,
				},
			},
			t2large: {
				Value: &core.ScalarValue{
					StringValue: &t2large,
				},
			},
			t2xlarge: {
				Value: &core.ScalarValue{
					StringValue: &t2xlarge,
				},
			},
			t22xlarge: {
				Value: &core.ScalarValue{
					StringValue: &t22xlarge,
				},
			},
		},
	}, nil
}

func (t *celerityCustomVariableType) GetType(
	ctx context.Context,
	input *provider.CustomVariableTypeGetTypeInput,
) (*provider.CustomVariableTypeGetTypeOutput, error) {
	return &provider.CustomVariableTypeGetTypeOutput{
		Type: "celerity/customVariable",
	}, nil
}

func (t *celerityCustomVariableType) GetDescription(
	ctx context.Context,
	input *provider.CustomVariableTypeGetDescriptionInput,
) (*provider.CustomVariableTypeGetDescriptionOutput, error) {
	return &provider.CustomVariableTypeGetDescriptionOutput{
		MarkdownDescription:  "### Celerity Custom Variable\n\nA custom variable type for Celerity.",
		PlainTextDescription: "Celerity Custom Variable\n\nA custom variable type for Celerity.",
	}, nil
}

func (t *celerityCustomVariableType) GetExamples(
	ctx context.Context,
	input *provider.CustomVariableTypeGetExamplesInput,
) (*provider.CustomVariableTypeGetExamplesOutput, error) {
	return &provider.CustomVariableTypeGetExamplesOutput{
		PlainTextExamples: []string{},
		MarkdownExamples:  []string{},
	}, nil
}
