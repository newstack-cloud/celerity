package blueprint

import (
	"context"
	"errors"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

type celerityProvider struct {
	resources   map[string]provider.Resource
	dataSources map[string]provider.DataSource
}

func NewCelerityProvider() provider.Provider {
	return &celerityProvider{
		resources: map[string]provider.Resource{
			"celerity/handler": &celerityHandlerResource{},
		},
		dataSources: map[string]provider.DataSource{
			"celerity/vpc": &celerityVPCDataSource{},
		},
	}
}

func (p *celerityProvider) Namespace(ctx context.Context) (string, error) {
	return "celerity", nil
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
	return nil, errors.New("custom var types not implemented")
}

func (p *celerityProvider) ListFunctions(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (p *celerityProvider) Function(ctx context.Context, functionName string) (provider.Function, error) {
	return nil, errors.New("functions not implemented")
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

func (r *celerityHandlerResource) StageChanges(
	ctx context.Context,
	input *provider.ResourceStageChangesInput,
) (*provider.ResourceStageChangesOutput, error) {
	return &provider.ResourceStageChangesOutput{}, nil
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
					"handlerName": {
						Description: "The name of the handler.",
						Type:        provider.ResourceDefinitionsSchemaTypeString,
					},
					"runtime": {
						Description: "The runtime that the handler uses.",
						Type:        provider.ResourceDefinitionsSchemaTypeString,
					},
				},
			},
		},
	}, nil
}

func (r *celerityHandlerResource) GetStateDefinition(
	ctx context.Context,
	input *provider.ResourceGetStateDefinitionInput,
) (*provider.ResourceGetStateDefinitionOutput, error) {
	return &provider.ResourceGetStateDefinitionOutput{
		StateDefinition: &provider.ResourceStateDefinition{
			Schema: &provider.ResourceDefinitionsSchema{
				Description: "The output state of a deployed handler for a Celerity application.",
				Type:        provider.ResourceDefinitionsSchemaTypeObject,
				Attributes: map[string]*provider.ResourceDefinitionsSchema{
					"id": {
						Description: "The ID of the handler in the deployed environment.",
						Type:        provider.ResourceDefinitionsSchemaTypeString,
					},
				},
			},
		},
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
		Data: map[string]interface{}{},
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
		Fields: []string{},
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
