package container

import (
	"context"
	"fmt"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/internal"
	"github.com/two-hundred/celerity/libs/blueprint/linkhelpers"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

func createParams() core.BlueprintParams {
	return core.NewDefaultParams(
		map[string]map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
	)
}

func newTestAWSProvider() provider.Provider {
	return &internal.ProviderMock{
		NamespaceValue: "aws",
		Resources: map[string]provider.Resource{
			"aws/dynamodb/table":  &internal.DynamoDBTableResource{},
			"aws/dynamodb/stream": &internal.DynamoDBStreamResource{},
			"aws/lambda/function": &internal.LambdaFunctionResource{},
		},
		Links: map[string]provider.Link{
			"aws/apigateway/api::aws/lambda/function":  &testApiGatewayLambdaLink{},
			"aws/lambda/function::aws/dynamodb/table":  &testLambdaDynamoDBTableLink{},
			"aws/dynamodb/table::aws/dynamodb/stream":  &testDynamoDBTableStreamLink{},
			"aws/dynamodb/stream::aws/lambda/function": &testDynamoDBStreamLambdaLink{},
			"aws/lambda/function::aws/lambda/function": &testLambdaLambdaLink{},
			"aws/dynamodb/table::aws/lambda/function":  &testDynamoDBTableLambdaLink{},
			"aws/ec2/subnet::aws/ec2/vpc":              &testSubnetVPCLink{},
			"aws/ec2/securityGroup::aws/ec2/vpc":       &testSecurityGroupVPCLink{},
			"aws/ec2/routeTable::aws/ec2/vpc":          &testRouteTableVPCLink{},
			"aws/ec2/route::aws/ec2/routeTable":        &testRouteRouteTableLink{},
			"aws/ec2/route::aws/ec2/internetGateway":   &testRouteInternetGatewayLink{},
		},
		CustomVariableTypes: map[string]provider.CustomVariableType{
			"aws/ec2/instanceType": &internal.InstanceTypeCustomVariableType{},
		},
		DataSources: map[string]provider.DataSource{
			"aws/vpc": &internal.VPCDataSource{},
		},
	}
}

func newTestExampleProvider() provider.Provider {
	return &internal.ProviderMock{
		NamespaceValue: "example",
		Resources: map[string]provider.Resource{
			"example/complex": &internal.ExampleComplexResource{},
		},
		Links:               map[string]provider.Link{},
		CustomVariableTypes: map[string]provider.CustomVariableType{},
		DataSources:         map[string]provider.DataSource{},
	}
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

func (l *testApiGatewayLambdaLink) HandleResourceAError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testApiGatewayLambdaLink) HandleResourceBError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testApiGatewayLambdaLink) UpdateResourceA(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testApiGatewayLambdaLink) UpdateResourceB(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testApiGatewayLambdaLink) UpdateIntermediaryResources(
	ctx context.Context,
	input *provider.LinkUpdateIntermediaryResourcesInput,
) (*provider.LinkUpdateIntermediaryResourcesOutput, error) {
	return &provider.LinkUpdateIntermediaryResourcesOutput{}, nil
}

type testLambdaDynamoDBTableLink struct{}

func (l *testLambdaDynamoDBTableLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	changes := &provider.LinkChanges{}

	functionResourceName := linkhelpers.GetResourceNameFromChanges(input.ResourceAChanges)
	tableResourceName := linkhelpers.GetResourceNameFromChanges(input.ResourceBChanges)

	currentLinkData := linkhelpers.GetLinkDataFromState(input.CurrentLinkState)

	tableFieldPath := fmt.Sprintf(
		"$.%s.environmentVariables.TABLE_NAME_%s",
		functionResourceName,
		tableResourceName,
	)
	err := linkhelpers.CollectChanges(
		"$.spec.tableName",
		tableFieldPath,
		currentLinkData,
		input.ResourceBChanges,
		changes,
	)
	if err != nil {
		return nil, err
	}

	regionFieldPath := fmt.Sprintf(
		"$.%s.environmentVariables.TABLE_REGION_%s",
		functionResourceName,
		tableResourceName,
	)
	err = linkhelpers.CollectChanges(
		"$.spec.region",
		regionFieldPath,
		currentLinkData,
		input.ResourceBChanges,
		changes,
	)
	if err != nil {
		return nil, err
	}

	accessType := linkhelpers.GetAnnotation(
		input.ResourceAChanges,
		"aws.lambda.dynamodb.accessType",
		core.MappingNodeFromString("read"),
	)
	actionArray := l.policyStatementActionFromAccessType(accessType)
	actionFieldPath := fmt.Sprintf(
		"$.%s[\"iam.policyStatements\"].0.action",
		functionResourceName,
	)
	err = linkhelpers.CollectLinkDataChanges(
		actionFieldPath,
		currentLinkData,
		changes,
		actionArray,
	)
	if err != nil {
		return nil, err
	}

	effect := core.MappingNodeFromString("Allow")
	effectFieldPath := fmt.Sprintf(
		"$.%s[\"iam.policyStatements\"].0.effect",
		functionResourceName,
	)
	err = linkhelpers.CollectLinkDataChanges(
		effectFieldPath,
		currentLinkData,
		changes,
		effect,
	)
	if err != nil {
		return nil, err
	}

	resourceARNFieldPath := fmt.Sprintf(
		"$.%s[\"iam.policyStatements\"].0.resource",
		functionResourceName,
	)
	err = linkhelpers.CollectChanges(
		"$.spec.id",
		resourceARNFieldPath,
		currentLinkData,
		input.ResourceBChanges,
		changes,
	)
	if err != nil {
		return nil, err
	}

	return &provider.LinkStageChangesOutput{
		Changes: changes,
	}, nil
}

func (l *testLambdaDynamoDBTableLink) policyStatementActionFromAccessType(accessType *core.MappingNode) *core.MappingNode {
	switch *accessType.Scalar.StringValue {
	case "write":
		return &core.MappingNode{
			Items: []*core.MappingNode{
				core.MappingNodeFromString("dynamodb:PutItem"),
				core.MappingNodeFromString("dynamodb:DeleteItem"),
				core.MappingNodeFromString("dynamodb:UpdateItem"),
			},
		}
	case "readwrite":
		return &core.MappingNode{
			Items: []*core.MappingNode{
				core.MappingNodeFromString("dynamodb:GetItem"),
				core.MappingNodeFromString("dynamodb:PutItem"),
				core.MappingNodeFromString("dynamodb:DeleteItem"),
				core.MappingNodeFromString("dynamodb:UpdateItem"),
			},
		}
	default:
		return &core.MappingNode{
			Items: []*core.MappingNode{
				core.MappingNodeFromString("dynamodb:GetItem"),
			},
		}
	}
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

func (l *testLambdaDynamoDBTableLink) HandleResourceAError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testLambdaDynamoDBTableLink) HandleResourceBError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testLambdaDynamoDBTableLink) UpdateResourceA(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testLambdaDynamoDBTableLink) UpdateResourceB(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testLambdaDynamoDBTableLink) UpdateIntermediaryResources(
	ctx context.Context,
	input *provider.LinkUpdateIntermediaryResourcesInput,
) (*provider.LinkUpdateIntermediaryResourcesOutput, error) {
	return &provider.LinkUpdateIntermediaryResourcesOutput{}, nil
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

func (l *testDynamoDBTableStreamLink) HandleResourceAError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testDynamoDBTableStreamLink) HandleResourceBError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testDynamoDBTableStreamLink) UpdateResourceA(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testDynamoDBTableStreamLink) UpdateResourceB(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testDynamoDBTableStreamLink) UpdateIntermediaryResources(
	ctx context.Context,
	input *provider.LinkUpdateIntermediaryResourcesInput,
) (*provider.LinkUpdateIntermediaryResourcesOutput, error) {
	return &provider.LinkUpdateIntermediaryResourcesOutput{}, nil
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

func (l *testDynamoDBStreamLambdaLink) HandleResourceAError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testDynamoDBStreamLambdaLink) HandleResourceBError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testDynamoDBStreamLambdaLink) UpdateResourceA(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testDynamoDBStreamLambdaLink) UpdateResourceB(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testDynamoDBStreamLambdaLink) UpdateIntermediaryResources(
	ctx context.Context,
	input *provider.LinkUpdateIntermediaryResourcesInput,
) (*provider.LinkUpdateIntermediaryResourcesOutput, error) {
	return &provider.LinkUpdateIntermediaryResourcesOutput{}, nil
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

func (l *testDynamoDBTableLambdaLink) HandleResourceAError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testDynamoDBTableLambdaLink) HandleResourceBError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testDynamoDBTableLambdaLink) UpdateResourceA(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testDynamoDBTableLambdaLink) UpdateResourceB(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testDynamoDBTableLambdaLink) UpdateIntermediaryResources(
	ctx context.Context,
	input *provider.LinkUpdateIntermediaryResourcesInput,
) (*provider.LinkUpdateIntermediaryResourcesOutput, error) {
	return &provider.LinkUpdateIntermediaryResourcesOutput{}, nil
}

type testLambdaLambdaLink struct{}

func (l *testLambdaLambdaLink) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	return &provider.LinkStageChangesOutput{}, nil
}

func (l *testLambdaLambdaLink) GetPriorityResourceType(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceTypeInput,
) (*provider.LinkGetPriorityResourceTypeOutput, error) {
	return &provider.LinkGetPriorityResourceTypeOutput{
		PriorityResourceType: "aws/lambda/function",
	}, nil
}

func (l *testLambdaLambdaLink) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return &provider.LinkGetTypeOutput{
		Type: "aws/lambda/function::aws/lambda/function",
	}, nil
}

func (l *testLambdaLambdaLink) GetKind(ctx context.Context, input *provider.LinkGetKindInput) (*provider.LinkGetKindOutput, error) {
	return &provider.LinkGetKindOutput{
		Kind: provider.LinkKindHard,
	}, nil
}

func (l *testLambdaLambdaLink) HandleResourceAError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testLambdaLambdaLink) HandleResourceBError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testLambdaLambdaLink) UpdateResourceA(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testLambdaLambdaLink) UpdateResourceB(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testLambdaLambdaLink) UpdateIntermediaryResources(
	ctx context.Context,
	input *provider.LinkUpdateIntermediaryResourcesInput,
) (*provider.LinkUpdateIntermediaryResourcesOutput, error) {
	return &provider.LinkUpdateIntermediaryResourcesOutput{}, nil
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

func (l *testSubnetVPCLink) HandleResourceAError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testSubnetVPCLink) HandleResourceBError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testSubnetVPCLink) UpdateResourceA(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testSubnetVPCLink) UpdateResourceB(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testSubnetVPCLink) UpdateIntermediaryResources(
	ctx context.Context,
	input *provider.LinkUpdateIntermediaryResourcesInput,
) (*provider.LinkUpdateIntermediaryResourcesOutput, error) {
	return &provider.LinkUpdateIntermediaryResourcesOutput{}, nil
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

func (l *testSecurityGroupVPCLink) HandleResourceAError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testSecurityGroupVPCLink) HandleResourceBError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testSecurityGroupVPCLink) UpdateResourceA(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testSecurityGroupVPCLink) UpdateResourceB(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testSecurityGroupVPCLink) UpdateIntermediaryResources(
	ctx context.Context,
	input *provider.LinkUpdateIntermediaryResourcesInput,
) (*provider.LinkUpdateIntermediaryResourcesOutput, error) {
	return &provider.LinkUpdateIntermediaryResourcesOutput{}, nil
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

func (l *testRouteTableVPCLink) HandleResourceAError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testRouteTableVPCLink) HandleResourceBError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testRouteTableVPCLink) UpdateResourceA(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testRouteTableVPCLink) UpdateResourceB(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testRouteTableVPCLink) UpdateIntermediaryResources(
	ctx context.Context,
	input *provider.LinkUpdateIntermediaryResourcesInput,
) (*provider.LinkUpdateIntermediaryResourcesOutput, error) {
	return &provider.LinkUpdateIntermediaryResourcesOutput{}, nil
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

func (l *testRouteRouteTableLink) HandleResourceAError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testRouteRouteTableLink) HandleResourceBError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testRouteRouteTableLink) UpdateResourceA(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testRouteRouteTableLink) UpdateResourceB(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testRouteRouteTableLink) UpdateIntermediaryResources(
	ctx context.Context,
	input *provider.LinkUpdateIntermediaryResourcesInput,
) (*provider.LinkUpdateIntermediaryResourcesOutput, error) {
	return &provider.LinkUpdateIntermediaryResourcesOutput{}, nil
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

func (l *testRouteInternetGatewayLink) HandleResourceAError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testRouteInternetGatewayLink) HandleResourceBError(
	ctx context.Context,
	input *provider.LinkHandleResourceErrorInput,
) error {
	return nil
}

func (l *testRouteInternetGatewayLink) UpdateResourceA(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testRouteInternetGatewayLink) UpdateResourceB(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return &provider.LinkUpdateResourceOutput{}, nil
}

func (l *testRouteInternetGatewayLink) UpdateIntermediaryResources(
	ctx context.Context,
	input *provider.LinkUpdateIntermediaryResourcesInput,
) (*provider.LinkUpdateIntermediaryResourcesOutput, error) {
	return &provider.LinkUpdateIntermediaryResourcesOutput{}, nil
}
