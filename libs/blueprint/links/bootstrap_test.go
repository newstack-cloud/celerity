package links

import (
	"context"
	"fmt"
	"testing"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) {
	TestingT(t)
}

type testBlueprintSpec struct {
	schema *schema.Blueprint
}

func (s *testBlueprintSpec) ResourceConcreteSpec(resourceName string) interface{} {
	return nil
}

func (s *testBlueprintSpec) ResourceSchema(resourceName string) *schema.Resource {
	return nil
}

func (s *testBlueprintSpec) Schema() *schema.Blueprint {
	return s.schema
}

type testAWSProvider struct {
	resources           map[string]provider.Resource
	links               map[string]provider.Link
	customVariableTypes map[string]provider.CustomVariableType
}

func newTestAWSProvider() provider.Provider {
	return &testAWSProvider{
		resources: map[string]provider.Resource{
			"aws/apigateway/api":         &testApiGatewayResource{},
			"aws/sqs/queue":              &testSQSQueueResource{},
			"aws/lambda/function":        &testLambdaFunctionResource{},
			"stratosaws/lambda/function": &testStratosLambdaFunctionResource{},
			"aws/dynamodb/table":         &testDynamoDBTableResource{},
			"aws/dynamodb/stream":        &testDynamoDBStreamResource{},
			"aws/iam/role":               &testIAMRoleResource{},
			"stratosaws/iam/role":        &testStratosIAMRoleResource{},
		},
		links: map[string]provider.Link{
			"aws/apigateway/api::aws/lambda/function":         &testApiGatewayLambdaLink{},
			"aws/sqs/queue::aws/lambda/function":              &testSQSQueueLambdaLink{},
			"aws/lambda/function::aws/dynamodb/table":         &testLambdaDynamoDBTableLink{},
			"aws/iam/role::aws/lambda/function":               &testIAMRoleLambdaLink{},
			"aws/lambda/function::aws/iam/role":               &testLambdaIAMRoleLink{},
			"aws/lambda/function::aws/sqs/queue":              &testLambdaSQSQueueLink{},
			"aws/dynamodb/table::aws/dynamodb/stream":         &testDynamoDBTableStreamLink{},
			"aws/dynamodb/stream::aws/lambda/function":        &testDynamoDBStreamLambdaLink{},
			"aws/dynamodb/stream::stratosaws/lambda/function": &testDynamoDBStreamStratosLambdaLink{},
			"aws/lambda/function::stratosaws/iam/role":        &testLambdaStratosIAMRoleLink{},
			"stratosaws/iam/role::aws/lambda/function":        &testStratosIAMRoleLambdaLink{},
			"stratosaws/lambda/function::aws/dynamodb/table":  &testStratosLambdaDynamoDBTableLink{},
		},
		customVariableTypes: map[string]provider.CustomVariableType{},
	}
}

func (p *testAWSProvider) Namespace(ctx context.Context) (string, error) {
	return "aws", nil
}

func (p *testAWSProvider) Resource(ctx context.Context, resourceType string) (provider.Resource, error) {
	return p.resources[resourceType], nil
}

func (p *testAWSProvider) Link(ctx context.Context, resourceTypeA string, resourceTypeB string) (provider.Link, error) {
	linkKey := fmt.Sprintf("%s::%s", resourceTypeA, resourceTypeB)
	return p.links[linkKey], nil
}

// DataSource is not used for spec link info!
func (p *testAWSProvider) DataSource(ctx context.Context, dataSourceType string) (provider.DataSource, error) {
	return nil, nil
}

// CustomVariableType is not used for spec link info!
func (p *testAWSProvider) CustomVariableType(ctx context.Context, customVariableType string) (provider.CustomVariableType, error) {
	return nil, nil
}

// ListResourceTypes is not used for spec link info!
func (p *testAWSProvider) ListResourceTypes(ctx context.Context) ([]string, error) {
	return nil, nil
}

// ListDataSourceTypes is not used for spec link info!
func (p *testAWSProvider) ListDataSourceTypes(ctx context.Context) ([]string, error) {
	return nil, nil
}

// ListCustomVariableTypes is not used for spec link info!
func (p *testAWSProvider) ListCustomVariableTypes(ctx context.Context) ([]string, error) {
	return nil, nil
}

// ListFunctions is not used for spec link info!
func (p *testAWSProvider) ListFunctions(ctx context.Context) ([]string, error) {
	return nil, nil
}

// Function is not used for spec link info!
func (p *testAWSProvider) Function(ctx context.Context, functionName string) (provider.Function, error) {
	return nil, nil
}

type testApiGatewayResource struct{}

func (r *testApiGatewayResource) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{
		CanLinkTo: []string{"aws/lambda/function"},
	}, nil
}

func (r *testApiGatewayResource) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{}, nil
}

func (r *testApiGatewayResource) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: false,
	}, nil
}

func (r *testApiGatewayResource) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "aws/apigateway/api",
	}, nil
}

func (r *testApiGatewayResource) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		PlainTextDescription: "",
		MarkdownDescription:  "",
	}, nil
}

// StageChanges is not used for spec link info!
func (r *testApiGatewayResource) StageChanges(
	ctx context.Context,
	input *provider.ResourceStageChangesInput,
) (*provider.ResourceStageChangesOutput, error) {
	return &provider.ResourceStageChangesOutput{}, nil
}

// CustomValidate is not used for spec link info!
func (r *testApiGatewayResource) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

// GetSpecDefinition is not used for spec link info!
func (r *testApiGatewayResource) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return &provider.ResourceGetSpecDefinitionOutput{}, nil
}

// GetStateDefinition is not used for spec link info!
func (r *testApiGatewayResource) GetStateDefinition(
	ctx context.Context,
	input *provider.ResourceGetStateDefinitionInput,
) (*provider.ResourceGetStateDefinitionOutput, error) {
	return &provider.ResourceGetStateDefinitionOutput{}, nil
}

// Deploy is not used for spec link info!
func (r *testApiGatewayResource) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

// GetExternalState is not used for spec link info!
func (r *testApiGatewayResource) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

// Destroy is not used for spec link info!
func (r *testApiGatewayResource) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}

type testSQSQueueResource struct{}

func (r *testSQSQueueResource) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{
		CanLinkTo: []string{"aws/lambda/function"},
	}, nil
}

func (r *testSQSQueueResource) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{}, nil
}

func (r *testSQSQueueResource) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: false,
	}, nil
}

func (r *testSQSQueueResource) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "aws/sqs/queue",
	}, nil
}

func (r *testSQSQueueResource) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		PlainTextDescription: "",
		MarkdownDescription:  "",
	}, nil
}

// StageChanges is not used for spec link info!
func (r *testSQSQueueResource) StageChanges(
	ctx context.Context,
	input *provider.ResourceStageChangesInput,
) (*provider.ResourceStageChangesOutput, error) {
	return &provider.ResourceStageChangesOutput{}, nil
}

// CustomValidate is not used for spec link info!
func (r *testSQSQueueResource) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

// GetSpecDefinition is not used for spec link info!
func (r *testSQSQueueResource) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return &provider.ResourceGetSpecDefinitionOutput{}, nil
}

// GetStateDefinition is not used for spec link info!
func (r *testSQSQueueResource) GetStateDefinition(
	ctx context.Context,
	input *provider.ResourceGetStateDefinitionInput,
) (*provider.ResourceGetStateDefinitionOutput, error) {
	return &provider.ResourceGetStateDefinitionOutput{}, nil
}

// Deploy is not used for spec link info!
func (r *testSQSQueueResource) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

// GetExternalState is not used for spec link info!
func (r *testSQSQueueResource) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

// Destroy is not used for spec link info!
func (r *testSQSQueueResource) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}

type testLambdaFunctionResource struct{}

func (r *testLambdaFunctionResource) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{
		// The inclusion of "aws/lambda/function" accounts for the case when
		// a resource is reported to be able to link to another where there is
		// no link implementation to catch a missing link implementation.
		CanLinkTo: []string{"aws/dynamodb/table", "aws/iam/role", "aws/lambda/function", "stratosaws/iam/role"},
	}, nil
}

func (r *testLambdaFunctionResource) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{}, nil
}

func (r *testLambdaFunctionResource) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: false,
	}, nil
}

func (r *testLambdaFunctionResource) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "aws/lambda/function",
	}, nil
}

func (r *testLambdaFunctionResource) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		PlainTextDescription: "",
		MarkdownDescription:  "",
	}, nil
}

// StageChanges is not used for spec link info!
func (r *testLambdaFunctionResource) StageChanges(
	ctx context.Context,
	input *provider.ResourceStageChangesInput,
) (*provider.ResourceStageChangesOutput, error) {
	return &provider.ResourceStageChangesOutput{}, nil
}

// CustomValidate is not used for spec link info!
func (r *testLambdaFunctionResource) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

// GetSpecDefinition is not used for spec link info!
func (r *testLambdaFunctionResource) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return &provider.ResourceGetSpecDefinitionOutput{}, nil
}

// GetStateDefinition is not used for spec link info!
func (r *testLambdaFunctionResource) GetStateDefinition(
	ctx context.Context,
	input *provider.ResourceGetStateDefinitionInput,
) (*provider.ResourceGetStateDefinitionOutput, error) {
	return &provider.ResourceGetStateDefinitionOutput{}, nil
}

// Deploy is not used for spec link info!
func (r *testLambdaFunctionResource) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

// GetExternalState is not used for spec link info!
func (r *testLambdaFunctionResource) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

// Destroy is not used for spec link info!
func (r *testLambdaFunctionResource) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}

type testStratosLambdaFunctionResource struct{}

func (r *testStratosLambdaFunctionResource) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{
		CanLinkTo: []string{"aws/dynamodb/table"},
	}, nil
}

func (r *testStratosLambdaFunctionResource) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{}, nil
}

func (r *testStratosLambdaFunctionResource) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: false,
	}, nil
}

func (r *testStratosLambdaFunctionResource) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "stratosaws/lambda/function",
	}, nil
}

func (r *testStratosLambdaFunctionResource) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		PlainTextDescription: "",
		MarkdownDescription:  "",
	}, nil
}

// StageChanges is not used for spec link info!
func (r *testStratosLambdaFunctionResource) StageChanges(
	ctx context.Context,
	input *provider.ResourceStageChangesInput,
) (*provider.ResourceStageChangesOutput, error) {
	return &provider.ResourceStageChangesOutput{}, nil
}

// CustomValidate is not used for spec link info!
func (r *testStratosLambdaFunctionResource) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

// GetSpecDefinition is not used for spec link info!
func (r *testStratosLambdaFunctionResource) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return &provider.ResourceGetSpecDefinitionOutput{}, nil
}

// GetStateDefinition is not used for spec link info!
func (r *testStratosLambdaFunctionResource) GetStateDefinition(
	ctx context.Context,
	input *provider.ResourceGetStateDefinitionInput,
) (*provider.ResourceGetStateDefinitionOutput, error) {
	return &provider.ResourceGetStateDefinitionOutput{}, nil
}

// Deploy is not used for spec link info!
func (r *testStratosLambdaFunctionResource) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

// GetExternalState is not used for spec link info!
func (r *testStratosLambdaFunctionResource) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

// Destroy is not used for spec link info!
func (r *testStratosLambdaFunctionResource) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}

type testDynamoDBTableResource struct{}

func (r *testDynamoDBTableResource) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{
		CanLinkTo: []string{"aws/dynamodb/stream"},
	}, nil
}

func (r *testDynamoDBTableResource) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{}, nil
}

func (r *testDynamoDBTableResource) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: true,
	}, nil
}

func (r *testDynamoDBTableResource) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "aws/dynamodb/table",
	}, nil
}

func (r *testDynamoDBTableResource) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		PlainTextDescription: "",
		MarkdownDescription:  "",
	}, nil
}

// StageChanges is not used for spec link info!
func (r *testDynamoDBTableResource) StageChanges(
	ctx context.Context,
	input *provider.ResourceStageChangesInput,
) (*provider.ResourceStageChangesOutput, error) {
	return &provider.ResourceStageChangesOutput{}, nil
}

// CustomValidate is not used for spec link info!
func (r *testDynamoDBTableResource) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

// GetSpecDefinition is not used for spec link info!
func (r *testDynamoDBTableResource) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return &provider.ResourceGetSpecDefinitionOutput{}, nil
}

// GetStateDefinition is not used for spec link info!
func (r *testDynamoDBTableResource) GetStateDefinition(
	ctx context.Context,
	input *provider.ResourceGetStateDefinitionInput,
) (*provider.ResourceGetStateDefinitionOutput, error) {
	return &provider.ResourceGetStateDefinitionOutput{}, nil
}

// Deploy is not used for spec link info!
func (r *testDynamoDBTableResource) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

// GetExternalState is not used for spec link info!
func (r *testDynamoDBTableResource) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

// Destroy is not used for spec link info!
func (r *testDynamoDBTableResource) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}

type testDynamoDBStreamResource struct{}

func (r *testDynamoDBStreamResource) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{
		CanLinkTo: []string{"aws/lambda/function", "stratosaws/lambda/function"},
	}, nil
}

func (r *testDynamoDBStreamResource) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{}, nil
}

func (r *testDynamoDBStreamResource) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: false,
	}, nil
}

func (r *testDynamoDBStreamResource) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "aws/dynamodb/stream",
	}, nil
}

func (r *testDynamoDBStreamResource) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		PlainTextDescription: "",
		MarkdownDescription:  "",
	}, nil
}

// StageChanges is not used for spec link info!
func (r *testDynamoDBStreamResource) StageChanges(
	ctx context.Context,
	input *provider.ResourceStageChangesInput,
) (*provider.ResourceStageChangesOutput, error) {
	return &provider.ResourceStageChangesOutput{}, nil
}

// CustomValidate is not used for spec link info!
func (r *testDynamoDBStreamResource) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

// GetSpecDefinition is not used for spec link info!
func (r *testDynamoDBStreamResource) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return &provider.ResourceGetSpecDefinitionOutput{}, nil
}

// GetStateDefinition is not used for spec link info!
func (r *testDynamoDBStreamResource) GetStateDefinition(
	ctx context.Context,
	input *provider.ResourceGetStateDefinitionInput,
) (*provider.ResourceGetStateDefinitionOutput, error) {
	return &provider.ResourceGetStateDefinitionOutput{}, nil
}

// Deploy is not used for spec link info!
func (r *testDynamoDBStreamResource) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

// GetExternalState is not used for spec link info!
func (r *testDynamoDBStreamResource) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

// Destroy is not used for spec link info!
func (r *testDynamoDBStreamResource) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}

type testIAMRoleResource struct{}

func (r *testIAMRoleResource) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{
		// "aws/lambda/function" is included here to test catching circular links.
		CanLinkTo: []string{"aws/iam/policy", "aws/lambda/function"},
	}, nil
}

func (r *testIAMRoleResource) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{}, nil
}

func (r *testIAMRoleResource) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: false,
	}, nil
}

func (r *testIAMRoleResource) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "aws/iam/role",
	}, nil
}

func (r *testIAMRoleResource) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		PlainTextDescription: "",
		MarkdownDescription:  "",
	}, nil
}

// StageChanges is not used for spec link info!
func (r *testIAMRoleResource) StageChanges(
	ctx context.Context,
	input *provider.ResourceStageChangesInput,
) (*provider.ResourceStageChangesOutput, error) {
	return &provider.ResourceStageChangesOutput{}, nil
}

// CustomValidate is not used for spec link info!
func (r *testIAMRoleResource) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

// GetSpecDefinition is not used for spec link info!
func (r *testIAMRoleResource) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return &provider.ResourceGetSpecDefinitionOutput{}, nil
}

// GetStateDefinition is not used for spec link info!
func (r *testIAMRoleResource) GetStateDefinition(
	ctx context.Context,
	input *provider.ResourceGetStateDefinitionInput,
) (*provider.ResourceGetStateDefinitionOutput, error) {
	return &provider.ResourceGetStateDefinitionOutput{}, nil
}

// Deploy is not used for spec link info!
func (r *testIAMRoleResource) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

// GetExternalState is not used for spec link info!
func (r *testIAMRoleResource) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

// Destroy is not used for spec link info!
func (r *testIAMRoleResource) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}

type testStratosIAMRoleResource struct{}

func (r *testStratosIAMRoleResource) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{
		// "aws/lambda/function" is included here to test catching circular links.
		CanLinkTo: []string{"aws/iam/policy", "aws/lambda/function"},
	}, nil
}

func (r *testStratosIAMRoleResource) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{}, nil
}

func (r *testStratosIAMRoleResource) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: false,
	}, nil
}

func (r *testStratosIAMRoleResource) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "stratosaws/iam/role",
	}, nil
}

func (r *testStratosIAMRoleResource) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		PlainTextDescription: "",
		MarkdownDescription:  "",
	}, nil
}

// StageChanges is not used for spec link info!
func (r *testStratosIAMRoleResource) StageChanges(
	ctx context.Context,
	input *provider.ResourceStageChangesInput,
) (*provider.ResourceStageChangesOutput, error) {
	return &provider.ResourceStageChangesOutput{}, nil
}

// CustomValidate is not used for spec link info!
func (r *testStratosIAMRoleResource) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

// GetSpecDefinition is not used for spec link info!
func (r *testStratosIAMRoleResource) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return &provider.ResourceGetSpecDefinitionOutput{}, nil
}

// GetStateDefinition is not used for spec link info!
func (r *testStratosIAMRoleResource) GetStateDefinition(
	ctx context.Context,
	input *provider.ResourceGetStateDefinitionInput,
) (*provider.ResourceGetStateDefinitionOutput, error) {
	return &provider.ResourceGetStateDefinitionOutput{}, nil
}

// Deploy is not used for spec link info!
func (r *testStratosIAMRoleResource) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

// GetExternalState is not used for spec link info!
func (r *testStratosIAMRoleResource) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

// Destroy is not used for spec link info!
func (r *testStratosIAMRoleResource) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testApiGatewayLambdaLink struct{}

// StageChanges is not used for spec link info!
func (l *testApiGatewayLambdaLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

// GetPriorityResourceType is not used for spec link info!
func (l *testApiGatewayLambdaLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{}, nil
}

// GetType is not used for spec link info!
func (l *testApiGatewayLambdaLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{}, nil
}

func (l *testApiGatewayLambdaLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		Kind: provider.LinkKindSoft,
	}, nil
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testApiGatewayLambdaLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testApiGatewayLambdaLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testApiGatewayLambdaLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testSQSQueueLambdaLink struct{}

// StageChanges is not used for spec link info!
func (l *testSQSQueueLambdaLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

// GetPriorityResourceType is not used for spec link info!
func (l *testSQSQueueLambdaLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{}, nil
}

// GetType is not used for spec link info!
func (l *testSQSQueueLambdaLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{}, nil
}

func (l *testSQSQueueLambdaLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		Kind: provider.LinkKindSoft,
	}, nil
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testSQSQueueLambdaLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testSQSQueueLambdaLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testSQSQueueLambdaLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testLambdaDynamoDBTableLink struct{}

// StageChanges is not used for spec link info!
func (l *testLambdaDynamoDBTableLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

// GetPriorityResourceType is not used for spec link info!
func (l *testLambdaDynamoDBTableLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{}, nil
}

// GetType is not used for spec link info!
func (l *testLambdaDynamoDBTableLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{}, nil
}

func (l *testLambdaDynamoDBTableLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		// For test purposes only, does not reflect reality!
		Kind: provider.LinkKindHard,
	}, nil
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testLambdaDynamoDBTableLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testLambdaDynamoDBTableLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testLambdaDynamoDBTableLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testStratosLambdaDynamoDBTableLink struct{}

// StageChanges is not used for spec link info!
func (l *testStratosLambdaDynamoDBTableLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

// GetPriorityResourceType is not used for spec link info!
func (l *testStratosLambdaDynamoDBTableLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{}, nil
}

// GetType is not used for spec link info!
func (l *testStratosLambdaDynamoDBTableLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{}, nil
}

func (l *testStratosLambdaDynamoDBTableLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		Kind: provider.LinkKindSoft,
	}, nil
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testStratosLambdaDynamoDBTableLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testStratosLambdaDynamoDBTableLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testStratosLambdaDynamoDBTableLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testLambdaSQSQueueLink struct{}

// StageChanges is not used for spec link info!
func (l *testLambdaSQSQueueLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

// GetPriorityResourceType is not used for spec link info!
func (l *testLambdaSQSQueueLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{}, nil
}

// GetType is not used for spec link info!
func (l *testLambdaSQSQueueLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{}, nil
}

func (l *testLambdaSQSQueueLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		Kind: provider.LinkKindSoft,
	}, nil
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testLambdaSQSQueueLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testLambdaSQSQueueLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testLambdaSQSQueueLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testDynamoDBTableStreamLink struct{}

// StageChanges is not used for spec link info!
func (l *testDynamoDBTableStreamLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

// GetPriorityResourceType is not used for spec link info!
func (l *testDynamoDBTableStreamLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{}, nil
}

// GetType is not used for spec link info!
func (l *testDynamoDBTableStreamLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{}, nil
}

func (l *testDynamoDBTableStreamLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		// The DynamoDB table must exist before the stream.
		Kind: provider.LinkKindHard,
	}, nil
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testDynamoDBTableStreamLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testDynamoDBTableStreamLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testDynamoDBTableStreamLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testDynamoDBStreamLambdaLink struct{}

// StageChanges is not used for spec link info!
func (l *testDynamoDBStreamLambdaLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

// GetPriorityResourceType is not used for spec link info!
func (l *testDynamoDBStreamLambdaLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{}, nil
}

// GetType is not used for spec link info!
func (l *testDynamoDBStreamLambdaLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{}, nil
}

func (l *testDynamoDBStreamLambdaLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		// For test purposes only, does not reflect reality!
		Kind: provider.LinkKindHard,
	}, nil
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testDynamoDBStreamLambdaLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testDynamoDBStreamLambdaLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testDynamoDBStreamLambdaLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testDynamoDBStreamStratosLambdaLink struct{}

// StageChanges is not used for spec link info!
func (l *testDynamoDBStreamStratosLambdaLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

// GetPriorityResourceType is not used for spec link info!
func (l *testDynamoDBStreamStratosLambdaLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{}, nil
}

// GetType is not used for spec link info!
func (l *testDynamoDBStreamStratosLambdaLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{}, nil
}

func (l *testDynamoDBStreamStratosLambdaLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		// For test purposes only, does not reflect reality!
		Kind: provider.LinkKindHard,
	}, nil
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testDynamoDBStreamStratosLambdaLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testDynamoDBStreamStratosLambdaLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testDynamoDBStreamStratosLambdaLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testIAMRoleLambdaLink struct{}

// StageChanges is not used for spec link info!
func (l *testIAMRoleLambdaLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

// GetPriorityResourceType is not used for spec link info!
func (l *testIAMRoleLambdaLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{}, nil
}

// GetType is not used for spec link info!
func (l *testIAMRoleLambdaLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{}, nil
}

func (l *testIAMRoleLambdaLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		// For test purposes only, does not reflect reality!
		Kind: provider.LinkKindHard,
	}, nil
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testIAMRoleLambdaLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testIAMRoleLambdaLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testIAMRoleLambdaLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testStratosIAMRoleLambdaLink struct{}

// StageChanges is not used for spec link info!
func (l *testStratosIAMRoleLambdaLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

// GetPriorityResourceType is not used for spec link info!
func (l *testStratosIAMRoleLambdaLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{}, nil
}

// GetType is not used for spec link info!
func (l *testStratosIAMRoleLambdaLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{}, nil
}

func (l *testStratosIAMRoleLambdaLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		Kind: provider.LinkKindSoft,
	}, nil
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testStratosIAMRoleLambdaLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testStratosIAMRoleLambdaLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testStratosIAMRoleLambdaLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testLambdaIAMRoleLink struct{}

// StageChanges is not used for spec link info!
func (l *testLambdaIAMRoleLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

// GetPriorityResourceType is not used for spec link info!
func (l *testLambdaIAMRoleLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{}, nil
}

// GetType is not used for spec link info!
func (l *testLambdaIAMRoleLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{}, nil
}

func (l *testLambdaIAMRoleLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		// For test purposes only, does not reflect reality!
		Kind: provider.LinkKindHard,
	}, nil
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testLambdaIAMRoleLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testLambdaIAMRoleLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testLambdaIAMRoleLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testLambdaStratosIAMRoleLink struct{}

// StageChanges is not used for spec link info!
func (l *testLambdaStratosIAMRoleLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

// GetPriorityResourceType is not used for spec link info!
func (l *testLambdaStratosIAMRoleLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{}, nil
}

// GetType is not used for spec link info!
func (l *testLambdaStratosIAMRoleLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{}, nil
}

func (l *testLambdaStratosIAMRoleLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		Kind: provider.LinkKindSoft,
	}, nil
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testLambdaStratosIAMRoleLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testLambdaStratosIAMRoleLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testLambdaStratosIAMRoleLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

// Provides a version of a chain link purely for snapshot tests.
// this simplifies the linked from references to a slice of resource names.
type snapshotChainLinkNode struct {
	// ResourceName is the unique name in the spec for the current
	// resource in the chain.
	ResourceName string
	// Resource holds the information about a resource at the blueprint spec schema-level,
	// most importantly the resource type that allows us to efficiently get a resource type
	// provider implementation for a link in a chain.
	Resource *schema.Resource
	// Selectors provides a mapping of the selector attribute to the resources
	// the current resource links to.
	// (e.g. "label::app:orderApi" -> ["createOrderFunction", "removeOrderFunction"])
	Selectors map[string][]string
	// LinkImplementations holds the link provider implementations keyed by resource name
	// for all the resources the current resource in the chain links
	// to.
	LinkImplementations map[string]provider.Link
	// LinksTo holds the chain link nodes for the resources
	// that the curent resource links to.
	LinksTo []*snapshotChainLinkNode
	// LinkedFrom holds the resource names for the chain link nodes that link to the current resource.
	// This information is important to allow backtracking when the blueprint container
	// is deciding the order in which resources should be deployed.
	LinkedFrom []string
	// Paths holds all the different "routes" to get to the current link in a set of chains.
	// These are known as materialised paths in the context of tree data structures.
	// Having this information here allows us to efficiently find out if
	// there is a relationship between two links at any depth in the chain.
	Paths []string
}
