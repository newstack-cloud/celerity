package testutils

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

type DynamoDBTableResource struct{}

func (r *DynamoDBTableResource) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{
		CanLinkTo: []string{"aws/dynamodb/stream"},
	}, nil
}

func (r *DynamoDBTableResource) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: true,
	}, nil
}

func (r *DynamoDBTableResource) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "aws/dynamodb/table",
	}, nil
}

func (r *DynamoDBTableResource) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		PlainTextDescription: "",
		MarkdownDescription:  "# DynamoDB Table\n\nA table in DynamoDB.",
	}, nil
}

func (r *DynamoDBTableResource) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{
		StabilisedDependencies: []string{},
	}, nil
}

func (r *DynamoDBTableResource) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

func (r *DynamoDBTableResource) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return &provider.ResourceGetSpecDefinitionOutput{
		SpecDefinition: &provider.ResourceSpecDefinition{
			Schema: &provider.ResourceDefinitionsSchema{
				Type: provider.ResourceDefinitionsSchemaTypeObject,
				Attributes: map[string]*provider.ResourceDefinitionsSchema{
					"id": {
						Type:     provider.ResourceDefinitionsSchemaTypeString,
						Computed: true,
					},
					"tableName": {
						Type: provider.ResourceDefinitionsSchemaTypeString,
					},
				},
			},
		},
	}, nil
}

func (r *DynamoDBTableResource) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

func (r *DynamoDBTableResource) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

func (r *DynamoDBTableResource) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}
