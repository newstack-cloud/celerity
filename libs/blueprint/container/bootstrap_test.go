package container

import (
	"context"
	"fmt"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/internal"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

func createParams() core.BlueprintParams {
	return internal.NewParams(
		map[string]map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
	)
}

type testAWSProvider struct {
	resources           map[string]provider.Resource
	dataSources         map[string]provider.DataSource
	links               map[string]provider.Link
	customVariableTypes map[string]provider.CustomVariableType
}

func newTestAWSProvider() provider.Provider {
	return &testAWSProvider{
		resources: map[string]provider.Resource{
			"aws/dynamodb/table":  &internal.DynamoDBTableResource{},
			"aws/lambda/function": &internal.LambdaFunctionResource{},
		},
		links: map[string]provider.Link{
			"aws/apigateway/api::aws/lambda/function":  &testApiGatewayLambdaLink{},
			"aws/lambda/function::aws/dynamodb/table":  &testLambdaDynamoDBTableLink{},
			"aws/dynamodb/table::aws/dynamodb/stream":  &testDynamoDBTableStreamLink{},
			"aws/dynamodb/stream::aws/lambda/function": &testDynamoDBStreamLambdaLink{},
			"aws/dynamodb/table::aws/lambda/function":  &testDynamoDBTableLambdaLink{},
			"aws/ec2/subnet::aws/ec2/vpc":              &testSubnetVPCLink{},
			"aws/ec2/securityGroup::aws/ec2/vpc":       &testSecurityGroupVPCLink{},
			"aws/ec2/routeTable::aws/ec2/vpc":          &testRouteTableVPCLink{},
			"aws/ec2/route::aws/ec2/routeTable":        &testRouteRouteTableLink{},
			"aws/ec2/route::aws/ec2/internetGateway":   &testRouteInternetGatewayLink{},
		},
		customVariableTypes: map[string]provider.CustomVariableType{
			"aws/ec2/instanceType": &internal.InstanceTypeCustomVariableType{},
		},
		dataSources: map[string]provider.DataSource{
			"aws/vpc": &internal.VPCDataSource{},
		},
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

func (p *testAWSProvider) DataSource(ctx context.Context, dataSourceType string) (provider.DataSource, error) {
	return p.dataSources[dataSourceType], nil
}

func (p *testAWSProvider) CustomVariableType(ctx context.Context, customVariableType string) (provider.CustomVariableType, error) {
	return p.customVariableTypes[customVariableType], nil
}

func (p *testAWSProvider) ListFunctions(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (p *testAWSProvider) ListResourceTypes(ctx context.Context) ([]string, error) {
	resourceTypes := make([]string, 0, len(p.resources))
	for resourceType := range p.resources {
		resourceTypes = append(resourceTypes, resourceType)
	}
	return resourceTypes, nil
}

func (p *testAWSProvider) ListDataSourceTypes(ctx context.Context) ([]string, error) {
	dataSourceTypes := make([]string, 0, len(p.dataSources))
	for dataSourceType := range p.dataSources {
		dataSourceTypes = append(dataSourceTypes, dataSourceType)
	}
	return dataSourceTypes, nil
}

func (p *testAWSProvider) ListCustomVariableTypes(ctx context.Context) ([]string, error) {
	customVariableTypes := make([]string, 0, len(p.customVariableTypes))
	for customVariableType := range p.customVariableTypes {
		customVariableTypes = append(customVariableTypes, customVariableType)
	}
	return customVariableTypes, nil
}

func (p *testAWSProvider) Function(ctx context.Context, functionName string) (provider.Function, error) {
	return nil, nil
}

type testApiGatewayLambdaLink struct{}

func (l *testApiGatewayLambdaLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

func (l *testApiGatewayLambdaLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{
		PriorityResourceType: "aws/lambda/function",
	}, nil
}

func (l *testApiGatewayLambdaLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{
		Type: "aws/apigateway/api::aws/lambda/function",
	}, nil
}

func (l *testApiGatewayLambdaLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		Kind: provider.LinkKindSoft,
	}, nil
}

func (l *testApiGatewayLambdaLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

func (l *testApiGatewayLambdaLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

func (l *testApiGatewayLambdaLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

type testLambdaDynamoDBTableLink struct{}

func (l *testLambdaDynamoDBTableLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

func (l *testLambdaDynamoDBTableLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{
		PriorityResourceType: "aws/dynamodb/table",
	}, nil
}

func (l *testLambdaDynamoDBTableLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{
		Type: "aws/lambda/function::aws/dynamodb/table",
	}, nil
}

func (l *testLambdaDynamoDBTableLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		Kind: provider.LinkKindHard,
	}, nil
}

func (l *testLambdaDynamoDBTableLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

func (l *testLambdaDynamoDBTableLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

func (l *testLambdaDynamoDBTableLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

type testDynamoDBTableStreamLink struct{}

func (l *testDynamoDBTableStreamLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

func (l *testDynamoDBTableStreamLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{
		PriorityResourceType: "aws/dynamodb/table",
	}, nil
}

func (l *testDynamoDBTableStreamLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{
		Type: "aws/dynamodb/table::aws/dynamodb/stream",
	}, nil
}

func (l *testDynamoDBTableStreamLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		Kind: provider.LinkKindHard,
	}, nil
}

func (l *testDynamoDBTableStreamLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

func (l *testDynamoDBTableStreamLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

func (l *testDynamoDBTableStreamLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

type testDynamoDBStreamLambdaLink struct{}

func (l *testDynamoDBStreamLambdaLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

func (l *testDynamoDBStreamLambdaLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{
		PriorityResourceType: "aws/lambda/function",
	}, nil
}

func (l *testDynamoDBStreamLambdaLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{
		Type: "aws/dynamodb/stream::aws/lambda/function",
	}, nil
}

func (l *testDynamoDBStreamLambdaLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		Kind: provider.LinkKindSoft,
	}, nil
}

func (l *testDynamoDBStreamLambdaLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

func (l *testDynamoDBStreamLambdaLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

func (l *testDynamoDBStreamLambdaLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

type testDynamoDBTableLambdaLink struct{}

func (l *testDynamoDBTableLambdaLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

func (l *testDynamoDBTableLambdaLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{
		PriorityResourceType: "aws/dynamodb/table",
	}, nil
}

func (l *testDynamoDBTableLambdaLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{
		Type: "aws/dynamodb/table::aws/lambda/function",
	}, nil
}

func (l *testDynamoDBTableLambdaLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		Kind: provider.LinkKindHard,
	}, nil
}

func (l *testDynamoDBTableLambdaLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

func (l *testDynamoDBTableLambdaLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

func (l *testDynamoDBTableLambdaLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

type testSubnetVPCLink struct{}

func (l *testSubnetVPCLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

func (l *testSubnetVPCLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{
		PriorityResourceType: "aws/ec2/vpc",
	}, nil
}

func (l *testSubnetVPCLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{
		Type: "aws/ec2/subnet::aws/ec2/vpc",
	}, nil
}

func (l *testSubnetVPCLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		Kind: provider.LinkKindHard,
	}, nil
}

func (l *testSubnetVPCLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

func (l *testSubnetVPCLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

func (l *testSubnetVPCLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

type testSecurityGroupVPCLink struct{}

func (l *testSecurityGroupVPCLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

func (l *testSecurityGroupVPCLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{
		PriorityResourceType: "aws/ec2/vpc",
	}, nil
}

func (l *testSecurityGroupVPCLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{
		Type: "aws/ec2/securityGroup::aws/ec2/vpc",
	}, nil
}

func (l *testSecurityGroupVPCLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		Kind: provider.LinkKindHard,
	}, nil
}

func (l *testSecurityGroupVPCLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

func (l *testSecurityGroupVPCLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

func (l *testSecurityGroupVPCLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

type testRouteTableVPCLink struct{}

func (l *testRouteTableVPCLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

func (l *testRouteTableVPCLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{
		PriorityResourceType: "aws/ec2/vpc",
	}, nil
}

func (l *testRouteTableVPCLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{
		Type: "aws/ec2/routeTable::aws/ec2/vpc",
	}, nil
}

func (l *testRouteTableVPCLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		Kind: provider.LinkKindHard,
	}, nil
}

func (l *testRouteTableVPCLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

func (l *testRouteTableVPCLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

func (l *testRouteTableVPCLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

type testRouteRouteTableLink struct{}

func (l *testRouteRouteTableLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

func (l *testRouteRouteTableLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{
		PriorityResourceType: "aws/ec2/routeTable",
	}, nil
}

func (l *testRouteRouteTableLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{
		Type: "aws/ec2/route::aws/ec2/routeTable",
	}, nil
}

func (l *testRouteRouteTableLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		Kind: provider.LinkKindHard,
	}, nil
}

func (l *testRouteRouteTableLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

func (l *testRouteRouteTableLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

func (l *testRouteRouteTableLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}

type testRouteInternetGatewayLink struct{}

func (l *testRouteInternetGatewayLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

func (l *testRouteInternetGatewayLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{
		PriorityResourceType: "aws/ec2/internetGateway",
	}, nil
}

func (l *testRouteInternetGatewayLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{
		Type: "aws/ec2/route::aws/ec2/internetGateway",
	}, nil
}

func (l *testRouteInternetGatewayLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		Kind: provider.LinkKindHard,
	}, nil
}

func (l *testRouteInternetGatewayLink) HandleResourceTypeAError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

func (l *testRouteInternetGatewayLink) HandleResourceTypeBError(
	ctx context.Context,
	input *provider.LinkHandleResourceTypeErrorInput,
) error {
	return nil
}

func (l *testRouteInternetGatewayLink) Deploy(
	ctx context.Context,
	input *provider.LinkDeployInput,
) (*provider.LinkDeployOutput, error) {
	return &provider.LinkDeployOutput{}, nil
}
