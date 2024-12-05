// Resource implementations for tests.

package internal

import (
	"context"
	"fmt"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

type DynamoDBTableResource struct{}

func (r *DynamoDBTableResource) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{
		CanLinkTo: []string{"aws/dynamodb/stream", "aws/lambda/function"},
	}, nil
}

func (r *DynamoDBTableResource) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{}, nil
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
	defaultGlobal := false
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
					"region": {
						Type: provider.ResourceDefinitionsSchemaTypeString,
					},
					"global": {
						Type: provider.ResourceDefinitionsSchemaTypeBoolean,
						Default: &core.MappingNode{
							Scalar: &core.ScalarValue{
								BoolValue: &defaultGlobal,
							},
						},
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

type DynamoDBStreamResource struct{}

func (r *DynamoDBStreamResource) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{
		CanLinkTo: []string{"aws/lambda/function"},
	}, nil
}

func (r *DynamoDBStreamResource) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{}, nil
}

func (r *DynamoDBStreamResource) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: false,
	}, nil
}

func (r *DynamoDBStreamResource) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "aws/dynamodb/stream",
	}, nil
}

func (r *DynamoDBStreamResource) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		PlainTextDescription: "",
		MarkdownDescription:  "# DynamoDB Stream\n\nA table event stream in DynamoDB.",
	}, nil
}

func (r *DynamoDBStreamResource) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

func (r *DynamoDBStreamResource) GetSpecDefinition(
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
					"label": {
						Type: provider.ResourceDefinitionsSchemaTypeString,
					},
					"region": {
						Type: provider.ResourceDefinitionsSchemaTypeString,
					},
				},
			},
		},
	}, nil
}

func (r *DynamoDBStreamResource) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

func (r *DynamoDBStreamResource) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

func (r *DynamoDBStreamResource) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}

type LambdaFunctionResource struct{}

func (r *LambdaFunctionResource) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{
		CanLinkTo: []string{"aws/dynamodb/table", "aws/lambda/function"},
	}, nil
}

func (r *LambdaFunctionResource) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{}, nil
}

func (r *LambdaFunctionResource) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: true,
	}, nil
}

func (r *LambdaFunctionResource) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "aws/lambda/function",
	}, nil
}

func (r *LambdaFunctionResource) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		PlainTextDescription: "",
		MarkdownDescription:  "# AWS Lambda\n\nA Lambda function in AWS.",
	}, nil
}

func (r *LambdaFunctionResource) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

func (r *LambdaFunctionResource) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return &provider.ResourceGetSpecDefinitionOutput{
		SpecDefinition: &provider.ResourceSpecDefinition{
			Schema: &provider.ResourceDefinitionsSchema{
				Type: provider.ResourceDefinitionsSchemaTypeObject,
				Attributes: map[string]*provider.ResourceDefinitionsSchema{
					"id": {
						Type: provider.ResourceDefinitionsSchemaTypeString,
					},
					"handler": {
						Type: provider.ResourceDefinitionsSchemaTypeString,
					},
				},
			},
		},
	}, nil
}

func (r *LambdaFunctionResource) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

func (r *LambdaFunctionResource) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

func (r *LambdaFunctionResource) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}

type ExampleComplexResource struct{}

func (r *ExampleComplexResource) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{
		CanLinkTo: []string{},
	}, nil
}

func (r *ExampleComplexResource) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{}, nil
}

func (r *ExampleComplexResource) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: true,
	}, nil
}

func (r *ExampleComplexResource) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "example/complex",
	}, nil
}

func (r *ExampleComplexResource) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		PlainTextDescription: "",
		MarkdownDescription:  "# An example resource with a complex specification",
	}, nil
}

func (r *ExampleComplexResource) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

func (r *ExampleComplexResource) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	defaultPrimaryPort := 80
	defaultOtherItemConfigValue1 := "Contents of value 1"
	defaultOtherItemConfigValue2 := "Contents of value 2"
	defaultVendorId := "default-vendor-id"

	return &provider.ResourceGetSpecDefinitionOutput{
		SpecDefinition: &provider.ResourceSpecDefinition{
			Schema: &provider.ResourceDefinitionsSchema{
				Type: provider.ResourceDefinitionsSchemaTypeObject,
				Attributes: map[string]*provider.ResourceDefinitionsSchema{
					"id": {
						Type:     provider.ResourceDefinitionsSchemaTypeString,
						Computed: true,
					},
					"itemConfig": {
						Type: provider.ResourceDefinitionsSchemaTypeUnion,
						OneOf: []*provider.ResourceDefinitionsSchema{
							{
								Type: provider.ResourceDefinitionsSchemaTypeString,
							},
							{
								Type: provider.ResourceDefinitionsSchemaTypeObject,
								Attributes: map[string]*provider.ResourceDefinitionsSchema{
									"endpoints": {
										Type: provider.ResourceDefinitionsSchemaTypeArray,
										Items: &provider.ResourceDefinitionsSchema{
											Type: provider.ResourceDefinitionsSchemaTypeString,
										},
									},
									"primaryPort": {
										Type: provider.ResourceDefinitionsSchemaTypeInteger,
										Default: &core.MappingNode{
											Scalar: &core.ScalarValue{
												IntValue: &defaultPrimaryPort,
											},
										},
									},
									"score": {
										Type:     provider.ResourceDefinitionsSchemaTypeFloat,
										Nullable: true,
									},
									"ipv4": {
										Type: provider.ResourceDefinitionsSchemaTypeBoolean,
									},
									// Deep config exists to test out the max depth logic where changes
									// beyond a certain depth should not be staged.
									// See validation.MappingNodeMaxTraverseDepth for the max depth.
									"deepConfig": {
										Type:       provider.ResourceDefinitionsSchemaTypeObject,
										Attributes: createDeepObjectSchema(25, "item"),
										Nullable:   true,
									},
									"metadata": {
										Type: provider.ResourceDefinitionsSchemaTypeMap,
										MapValues: &provider.ResourceDefinitionsSchema{
											Type: provider.ResourceDefinitionsSchemaTypeString,
										},
									},
								},
							},
						},
						MustRecreate: true,
					},
					"otherItemConfig": {
						Type: provider.ResourceDefinitionsSchemaTypeUnion,
						OneOf: []*provider.ResourceDefinitionsSchema{
							{
								Type: provider.ResourceDefinitionsSchemaTypeString,
							},
							{
								Type: provider.ResourceDefinitionsSchemaTypeMap,
								MapValues: &provider.ResourceDefinitionsSchema{
									Type: provider.ResourceDefinitionsSchemaTypeObject,
									Attributes: map[string]*provider.ResourceDefinitionsSchema{
										"value1": {
											Type: provider.ResourceDefinitionsSchemaTypeString,
										},
										"value2": {
											Type: provider.ResourceDefinitionsSchemaTypeString,
										},
									},
								},
							},
						},
						Default: &core.MappingNode{
							Fields: map[string]*core.MappingNode{
								"default": {
									Fields: map[string]*core.MappingNode{
										"value1": {
											Scalar: &core.ScalarValue{
												StringValue: &defaultOtherItemConfigValue1,
											},
										},
										"value2": {
											Scalar: &core.ScalarValue{
												StringValue: &defaultOtherItemConfigValue2,
											},
										},
									},
								},
							},
						},
					},
					"vendorTags": {
						Type: provider.ResourceDefinitionsSchemaTypeUnion,
						OneOf: []*provider.ResourceDefinitionsSchema{
							{
								Type: provider.ResourceDefinitionsSchemaTypeString,
							},
							{
								Type: provider.ResourceDefinitionsSchemaTypeArray,
								Items: &provider.ResourceDefinitionsSchema{
									Type: provider.ResourceDefinitionsSchemaTypeString,
								},
							},
						},
					},
					"vendorConfig": {
						Type: provider.ResourceDefinitionsSchemaTypeUnion,
						OneOf: []*provider.ResourceDefinitionsSchema{
							{
								Type: provider.ResourceDefinitionsSchemaTypeString,
							},
							{
								Type: provider.ResourceDefinitionsSchemaTypeArray,
								Items: &provider.ResourceDefinitionsSchema{
									Type: provider.ResourceDefinitionsSchemaTypeObject,
									Attributes: map[string]*provider.ResourceDefinitionsSchema{
										"vendorNamespace": {
											Type: provider.ResourceDefinitionsSchemaTypeString,
										},
										"vendorId": {
											Type: provider.ResourceDefinitionsSchemaTypeUnion,
											OneOf: []*provider.ResourceDefinitionsSchema{
												{
													Type: provider.ResourceDefinitionsSchemaTypeString,
												},
												{
													Type: provider.ResourceDefinitionsSchemaTypeInteger,
												},
											},
											Default: &core.MappingNode{
												Scalar: &core.ScalarValue{
													StringValue: &defaultVendorId,
												},
											},
										},
									},
								},
							},
						},
						Nullable: true,
					},
				},
			},
		},
	}, nil
}

func createDeepObjectSchema(depth int, fieldPrefix string) map[string]*provider.ResourceDefinitionsSchema {
	if depth == 0 {
		return nil
	}

	fieldName := fmt.Sprintf("%s%d", fieldPrefix, depth)
	return map[string]*provider.ResourceDefinitionsSchema{
		fieldName: {
			Type:       provider.ResourceDefinitionsSchemaTypeObject,
			Attributes: createDeepObjectSchema(depth-1, fieldPrefix),
		},
	}
}

func (r *ExampleComplexResource) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

func (r *ExampleComplexResource) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

func (r *ExampleComplexResource) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}
