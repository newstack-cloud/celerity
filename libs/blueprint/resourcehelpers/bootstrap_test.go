package resourcehelpers

import (
	"context"
	"errors"
	"testing"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) {
	TestingT(t)
}

type testProvider struct {
	functions   map[string]provider.Function
	resources   map[string]provider.Resource
	dataSources map[string]provider.DataSource
	namespace   string
}

func (p *testProvider) Namespace(ctx context.Context) (string, error) {
	return p.namespace, nil
}

func (p *testProvider) Resource(ctx context.Context, resourceType string) (provider.Resource, error) {
	resource, ok := p.resources[resourceType]
	if !ok {
		return nil, errors.New("resource not found")
	}
	return resource, nil
}

func (p *testProvider) DataSource(ctx context.Context, dataSourceType string) (provider.DataSource, error) {
	dataSource, ok := p.dataSources[dataSourceType]
	if !ok {
		return nil, errors.New("data source not found")
	}
	return dataSource, nil
}

func (p *testProvider) Link(ctx context.Context, resourceTypeA string, resourceTypeB string) (provider.Link, error) {
	return nil, nil
}

func (p *testProvider) CustomVariableType(ctx context.Context, customVariableType string) (provider.CustomVariableType, error) {
	return nil, nil
}

func (p *testProvider) ListFunctions(ctx context.Context) ([]string, error) {
	functionNames := []string{}
	for name := range p.functions {
		functionNames = append(functionNames, name)
	}
	return functionNames, nil
}

func (p *testProvider) Function(ctx context.Context, functionName string) (provider.Function, error) {
	function, ok := p.functions[functionName]
	if !ok {
		return nil, errors.New("function not found")
	}
	return function, nil
}

type testExampleResource struct {
	definition      *provider.ResourceSpecDefinition
	stateDefinition *provider.ResourceStateDefinition
}

func newTestExampleResource() provider.Resource {
	return &testExampleResource{
		definition: &provider.ResourceSpecDefinition{
			Schema: &provider.ResourceDefinitionsSchema{
				Type: provider.ResourceDefinitionsSchemaTypeObject,
				Attributes: map[string]*provider.ResourceDefinitionsSchema{
					"name": {
						Type: provider.ResourceDefinitionsSchemaTypeString,
					},
					"ids": {
						Type: provider.ResourceDefinitionsSchemaTypeArray,
						Items: &provider.ResourceDefinitionsSchema{
							Type: provider.ResourceDefinitionsSchemaTypeObject,
							Attributes: map[string]*provider.ResourceDefinitionsSchema{
								"name": {
									Type: provider.ResourceDefinitionsSchemaTypeString,
								},
							},
						},
					},
				},
			},
		},
		stateDefinition: &provider.ResourceStateDefinition{
			Schema: &provider.ResourceDefinitionsSchema{},
		},
	}
}

// CanLinkTo is not used for validation!
func (r *testExampleResource) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{}, nil
}

// IsCommonTerminal is not used for validation!
func (r *testExampleResource) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: false,
	}, nil
}

func (r *testExampleResource) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "celerity/exampleResource",
	}, nil
}

// StageChanges is not used for validation!
func (r *testExampleResource) StageChanges(
	ctx context.Context,
	input *provider.ResourceStageChangesInput,
) (*provider.ResourceStageChangesOutput, error) {
	return &provider.ResourceStageChangesOutput{}, nil
}

func (r *testExampleResource) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

func (r *testExampleResource) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return &provider.ResourceGetSpecDefinitionOutput{
		SpecDefinition: r.definition,
	}, nil
}

func (r *testExampleResource) GetStateDefinition(
	ctx context.Context,
	input *provider.ResourceGetStateDefinitionInput,
) (*provider.ResourceGetStateDefinitionOutput, error) {
	return &provider.ResourceGetStateDefinitionOutput{
		StateDefinition: r.stateDefinition,
	}, nil
}

// Deploy is not used for validation!
func (r *testExampleResource) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

// GetExternalState is not used for validation!
func (r *testExampleResource) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

// Destroy is not used for validation!
func (r *testExampleResource) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}
