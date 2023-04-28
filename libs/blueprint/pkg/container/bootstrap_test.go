package container

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/provider"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/state"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) {
	TestingT(t)
}

type testAWSProvider struct {
	resources           map[string]provider.Resource[any]
	links               map[string]provider.Link[any, any]
	customVariableTypes map[string]provider.CustomVariableType
}

func newTestAWSProvider() provider.Provider {
	return &testAWSProvider{
		resources: map[string]provider.Resource[any]{},
		links: map[string]provider.Link[any, any]{
			"aws/apigateway/api::aws/lambda/function":  &testApiGatewayLambdaLink{},
			"aws/lambda/function::aws/dynamodb/table":  &testLambdaDynamoDBTableLink{},
			"aws/dynamodb/table::aws/dynamodb/stream":  &testDynamoDBTableStreamLink{},
			"aws/dynamodb/stream::aws/lambda/function": &testDynamoDBStreamLambdaLink{},
			"aws/ec2/subnet::aws/ec2/vpc":              &testSubnetVPCLink{},
			"aws/ec2/securityGroup::aws/ec2/vpc":       &testSecurityGroupVPCLink{},
			"aws/ec2/routeTable::aws/ec2/vpc":          &testRouteTableVPCLink{},
			"aws/ec2/route::aws/ec2/routeTable":        &testRouteRouteTableLink{},
			"aws/ec2/route::aws/ec2/internetGateway":   &testRouteInternetGatewayLink{},
		},
		customVariableTypes: map[string]provider.CustomVariableType{},
	}
}

func (p *testAWSProvider) Resource(resourceType string) provider.Resource[any] {
	return p.resources[resourceType]
}

func (p *testAWSProvider) Link(resourceTypeA string, resourceTypeB string) provider.Link[any, any] {
	linkKey := fmt.Sprintf("%s::%s", resourceTypeA, resourceTypeB)
	return p.links[linkKey]
}

func (p *testAWSProvider) DataSource(dataSourceType string) provider.DataSource {
	return nil
}

func (p *testAWSProvider) CustomVariableType(customVariableType string) provider.CustomVariableType {
	return nil
}

type testApiGatewayLambdaLink struct{}

func (l *testApiGatewayLambdaLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo[any],
	resourceBInfo *provider.ResourceInfo[any],
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

func (l *testApiGatewayLambdaLink) PriorityResourceType() string {
	return "aws/lambda/function"
}

func (l *testApiGatewayLambdaLink) Type() provider.LinkType {
	return provider.LinkTypeSoft
}

func (l *testApiGatewayLambdaLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo[any]) error {
	return nil
}

func (l *testApiGatewayLambdaLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo[any]) error {
	return nil
}

func (l *testApiGatewayLambdaLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo[any],
	resourceBInfo *provider.ResourceInfo[any],
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

type testLambdaDynamoDBTableLink struct{}

func (l *testLambdaDynamoDBTableLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo[any],
	resourceBInfo *provider.ResourceInfo[any],
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

func (l *testLambdaDynamoDBTableLink) PriorityResourceType() string {
	return "aws/dynamodb/table"
}

func (l *testLambdaDynamoDBTableLink) Type() provider.LinkType {
	return provider.LinkTypeSoft
}

func (l *testLambdaDynamoDBTableLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo[any]) error {
	return nil
}

func (l *testLambdaDynamoDBTableLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo[any]) error {
	return nil
}

func (l *testLambdaDynamoDBTableLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo[any],
	resourceBInfo *provider.ResourceInfo[any],
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

type testDynamoDBTableStreamLink struct{}

func (l *testDynamoDBTableStreamLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo[any],
	resourceBInfo *provider.ResourceInfo[any],
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

func (l *testDynamoDBTableStreamLink) PriorityResourceType() string {
	return "aws/dynamodb/table"
}

func (l *testDynamoDBTableStreamLink) Type() provider.LinkType {
	return provider.LinkTypeHard
}

func (l *testDynamoDBTableStreamLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo[any]) error {
	return nil
}

func (l *testDynamoDBTableStreamLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo[any]) error {
	return nil
}

func (l *testDynamoDBTableStreamLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo[any],
	resourceBInfo *provider.ResourceInfo[any],
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

type testDynamoDBStreamLambdaLink struct{}

func (l *testDynamoDBStreamLambdaLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo[any],
	resourceBInfo *provider.ResourceInfo[any],
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

func (l *testDynamoDBStreamLambdaLink) PriorityResourceType() string {
	return "aws/lambda/function"
}

func (l *testDynamoDBStreamLambdaLink) Type() provider.LinkType {
	return provider.LinkTypeSoft
}

func (l *testDynamoDBStreamLambdaLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo[any]) error {
	return nil
}

func (l *testDynamoDBStreamLambdaLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo[any]) error {
	return nil
}

func (l *testDynamoDBStreamLambdaLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo[any],
	resourceBInfo *provider.ResourceInfo[any],
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

type testSubnetVPCLink struct{}

func (l *testSubnetVPCLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo[any],
	resourceBInfo *provider.ResourceInfo[any],
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

func (l *testSubnetVPCLink) PriorityResourceType() string {
	return "aws/ec2/vpc"
}

func (l *testSubnetVPCLink) Type() provider.LinkType {
	return provider.LinkTypeHard
}

func (l *testSubnetVPCLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo[any]) error {
	return nil
}

func (l *testSubnetVPCLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo[any]) error {
	return nil
}

func (l *testSubnetVPCLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo[any],
	resourceBInfo *provider.ResourceInfo[any],
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

type testSecurityGroupVPCLink struct{}

func (l *testSecurityGroupVPCLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo[any],
	resourceBInfo *provider.ResourceInfo[any],
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

func (l *testSecurityGroupVPCLink) PriorityResourceType() string {
	return "aws/ec2/vpc"
}

func (l *testSecurityGroupVPCLink) Type() provider.LinkType {
	return provider.LinkTypeHard
}

func (l *testSecurityGroupVPCLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo[any]) error {
	return nil
}

func (l *testSecurityGroupVPCLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo[any]) error {
	return nil
}

func (l *testSecurityGroupVPCLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo[any],
	resourceBInfo *provider.ResourceInfo[any],
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

type testRouteTableVPCLink struct{}

func (l *testRouteTableVPCLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo[any],
	resourceBInfo *provider.ResourceInfo[any],
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

func (l *testRouteTableVPCLink) PriorityResourceType() string {
	return "aws/ec2/vpc"
}

func (l *testRouteTableVPCLink) Type() provider.LinkType {
	return provider.LinkTypeHard
}

func (l *testRouteTableVPCLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo[any]) error {
	return nil
}

func (l *testRouteTableVPCLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo[any]) error {
	return nil
}

func (l *testRouteTableVPCLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo[any],
	resourceBInfo *provider.ResourceInfo[any],
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

type testRouteRouteTableLink struct{}

func (l *testRouteRouteTableLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo[any],
	resourceBInfo *provider.ResourceInfo[any],
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

func (l *testRouteRouteTableLink) PriorityResourceType() string {
	return "aws/ec2/routeTable"
}

func (l *testRouteRouteTableLink) Type() provider.LinkType {
	return provider.LinkTypeHard
}

func (l *testRouteRouteTableLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo[any]) error {
	return nil
}

func (l *testRouteRouteTableLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo[any]) error {
	return nil
}

func (l *testRouteRouteTableLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo[any],
	resourceBInfo *provider.ResourceInfo[any],
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

type testRouteInternetGatewayLink struct{}

func (l *testRouteInternetGatewayLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo[any],
	resourceBInfo *provider.ResourceInfo[any],
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

func (l *testRouteInternetGatewayLink) PriorityResourceType() string {
	return "aws/ec2/internetGateway"
}

func (l *testRouteInternetGatewayLink) Type() provider.LinkType {
	return provider.LinkTypeHard
}

func (l *testRouteInternetGatewayLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo[any]) error {
	return nil
}

func (l *testRouteInternetGatewayLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo[any]) error {
	return nil
}

func (l *testRouteInternetGatewayLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo[any],
	resourceBInfo *provider.ResourceInfo[any],
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

////////////////////////////////////////////////////////////////////////////////
// Test blueprint params implementing the core.BlueprintParams interface.
////////////////////////////////////////////////////////////////////////////////

type testBlueprintParams struct {
	providerConfig     map[string]map[string]*core.ScalarValue
	contextVariables   map[string]*core.ScalarValue
	blueprintVariables map[string]*core.ScalarValue
}

func (p *testBlueprintParams) ProviderConfig(namespace string) map[string]*core.ScalarValue {
	return p.providerConfig[namespace]
}

func (p *testBlueprintParams) ContextVariable(name string) *core.ScalarValue {
	return p.contextVariables[name]
}

func (p *testBlueprintParams) BlueprintVariable(name string) *core.ScalarValue {
	return p.blueprintVariables[name]
}

////////////////////////////////////////////////////////////////////////////////
// Test custom variable types implementing the provider.CustomVariableType interface.
////////////////////////////////////////////////////////////////////////////////

type testEC2InstanceTypeCustomVariableType struct{}

func (t *testEC2InstanceTypeCustomVariableType) Options(
	ctx context.Context,
	params core.BlueprintParams,
) (map[string]*core.ScalarValue, error) {
	t2nano := "t2.nano"
	t2micro := "t2.micro"
	t2small := "t2.small"
	t2medium := "t2.medium"
	t2large := "t2.large"
	t2xlarge := "t2.xlarge"
	t22xlarge := "t2.2xlarge"
	return map[string]*core.ScalarValue{
		t2nano: {
			StringValue: &t2nano,
		},
		t2micro: {
			StringValue: &t2micro,
		},
		t2small: {
			StringValue: &t2small,
		},
		t2medium: {
			StringValue: &t2medium,
		},
		t2large: {
			StringValue: &t2large,
		},
		t2xlarge: {
			StringValue: &t2xlarge,
		},
		t22xlarge: {
			StringValue: &t22xlarge,
		},
	}, nil
}

type testInvalidEC2InstanceTypeCustomVariableType struct{}

func (t *testInvalidEC2InstanceTypeCustomVariableType) Options(
	ctx context.Context,
	params core.BlueprintParams,
) (map[string]*core.ScalarValue, error) {
	// Invalid due to mixed scalar types.
	t2nano := "t2.nano"
	t2micro := 54039
	t2small := "t2.small"
	t2medium := "t2.medium"
	t2large := 32192.49
	t2xlarge := "t2.xlarge"
	t22xlarge := true
	return map[string]*core.ScalarValue{
		t2nano: {
			StringValue: &t2nano,
		},
		"t2.micro": {
			IntValue: &t2micro,
		},
		t2small: {
			StringValue: &t2small,
		},
		t2medium: {
			StringValue: &t2medium,
		},
		"t2.large": {
			FloatValue: &t2large,
		},
		t2xlarge: {
			StringValue: &t2xlarge,
		},
		"t2.2xlarge": {
			BoolValue: &t22xlarge,
		},
	}, nil
}

type testFailToLoadOptionsCustomVariableType struct{}

func (t *testFailToLoadOptionsCustomVariableType) Options(
	ctx context.Context,
	params core.BlueprintParams,
) (map[string]*core.ScalarValue, error) {
	return nil, errors.New("failed to load options")
}

type testRegionCustomVariableType struct{}

func (t *testRegionCustomVariableType) Options(
	ctx context.Context,
	params core.BlueprintParams,
) (map[string]*core.ScalarValue, error) {
	usEast1 := "us-east-1"
	usEast2 := "us-east-2"
	usWest1 := "us-west-1"
	usWest2 := "us-west-2"
	euWest1 := "eu-west-1"
	euWest2 := "eu-west-2"
	euCentral1 := "eu-central-1"

	return map[string]*core.ScalarValue{
		usEast1: {
			StringValue: &usEast1,
		},
		usEast2: {
			StringValue: &usEast2,
		},
		usWest1: {
			StringValue: &usWest1,
		},
		usWest2: {
			StringValue: &usWest2,
		},
		euWest1: {
			StringValue: &euWest1,
		},
		euWest2: {
			StringValue: &euWest2,
		},
		euCentral1: {
			StringValue: &euCentral1,
		},
	}, nil
}
