package container

import (
	"context"
	"fmt"
	"testing"

	"github.com/freshwebio/celerity/libs/blueprint/pkg/core"
	"github.com/freshwebio/celerity/libs/blueprint/pkg/provider"
	"github.com/freshwebio/celerity/libs/blueprint/pkg/state"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) {
	TestingT(t)
}

type testAWSProvider struct {
	resources map[string]provider.Resource[any]
	links     map[string]provider.Link[any, any]
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
