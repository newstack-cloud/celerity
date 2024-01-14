package links

import (
	"context"
	"fmt"
	"testing"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/provider"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/state"
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

func (p *testAWSProvider) Resource(resourceType string) provider.Resource {
	return p.resources[resourceType]
}

func (p *testAWSProvider) Link(resourceTypeA string, resourceTypeB string) provider.Link {
	linkKey := fmt.Sprintf("%s::%s", resourceTypeA, resourceTypeB)
	return p.links[linkKey]
}

// DataSource is not used for spec link info!
func (p *testAWSProvider) DataSource(dataSourceType string) provider.DataSource {
	return nil
}

// CustomVariableType is not used for spec link info!
func (p *testAWSProvider) CustomVariableType(customVariableType string) provider.CustomVariableType {
	return nil
}

type testApiGatewayResource struct{}

func (r *testApiGatewayResource) CanLinkTo() []string {
	return []string{"aws/lambda/function"}
}

func (r *testApiGatewayResource) IsCommonTerminal() bool {
	return false
}

func (r *testApiGatewayResource) GetType() string {
	return "aws/apigateway/api"
}

// StageChanges is not used for spec link info!
func (r *testApiGatewayResource) StageChanges(
	ctx context.Context,
	resourceInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (provider.Changes, error) {
	return provider.Changes{}, nil
}

// Validate is not used for spec link info!
func (r *testApiGatewayResource) Validate(ctx context.Context, schemaResource *schema.Resource, params core.BlueprintParams) error {
	return nil
}

// Deploy is not used for spec link info!
func (r *testApiGatewayResource) Deploy(ctx context.Context, changes provider.Changes, params core.BlueprintParams) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// GetExternalState is not used for spec link info!
func (r *testApiGatewayResource) GetExternalState(ctx context.Context, instanceID string, revisionID string, resourceID string) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// Destroy is not used for spec link info!
func (r *testApiGatewayResource) Destroy(ctx context.Context, instanceID string, revisionID string, resourceID string) error {
	return nil
}

type testSQSQueueResource struct{}

func (r *testSQSQueueResource) CanLinkTo() []string {
	return []string{"aws/lambda/function"}
}

func (r *testSQSQueueResource) IsCommonTerminal() bool {
	return false
}

func (r *testSQSQueueResource) GetType() string {
	return "aws/sqs/queue"
}

// StageChanges is not used for spec link info!
func (r *testSQSQueueResource) StageChanges(
	ctx context.Context,
	resourceInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (provider.Changes, error) {
	return provider.Changes{}, nil
}

// Validate is not used for spec link info!
func (r *testSQSQueueResource) Validate(ctx context.Context, schemaResource *schema.Resource, params core.BlueprintParams) error {
	return nil
}

// Deploy is not used for spec link info!
func (r *testSQSQueueResource) Deploy(ctx context.Context, changes provider.Changes, params core.BlueprintParams) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// GetExternalState is not used for spec link info!
func (r *testSQSQueueResource) GetExternalState(ctx context.Context, instanceID string, revisionID string, resourceID string) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// Destroy is not used for spec link info!
func (r *testSQSQueueResource) Destroy(ctx context.Context, instanceID string, revisionID string, resourceID string) error {
	return nil
}

type testLambdaFunctionResource struct{}

func (r *testLambdaFunctionResource) CanLinkTo() []string {
	// The inclusion of "aws/lambda/function" accounts for the case when
	// a resource is reported to be able to link to another where there is
	// no link implementation to catch a missing link implementation.
	return []string{"aws/dynamodb/table", "aws/iam/role", "aws/lambda/function", "stratosaws/iam/role"}
}

func (r *testLambdaFunctionResource) IsCommonTerminal() bool {
	return false
}

func (r *testLambdaFunctionResource) GetType() string {
	return "aws/lambda/function"
}

// StageChanges is not used for spec link info!
func (r *testLambdaFunctionResource) StageChanges(
	ctx context.Context,
	resourceInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (provider.Changes, error) {
	return provider.Changes{}, nil
}

// Validate is not used for spec link info!
func (r *testLambdaFunctionResource) Validate(ctx context.Context, schemaResource *schema.Resource, params core.BlueprintParams) error {
	return nil
}

// Deploy is not used for spec link info!
func (r *testLambdaFunctionResource) Deploy(ctx context.Context, changes provider.Changes, params core.BlueprintParams) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// GetExternalState is not used for spec link info!
func (r *testLambdaFunctionResource) GetExternalState(ctx context.Context, instanceID string, revisionID string, resourceID string) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// Destroy is not used for spec link info!
func (r *testLambdaFunctionResource) Destroy(ctx context.Context, instanceID string, revisionID string, resourceID string) error {
	return nil
}

type testStratosLambdaFunctionResource struct{}

func (r *testStratosLambdaFunctionResource) CanLinkTo() []string {
	return []string{"aws/dynamodb/table"}
}

func (r *testStratosLambdaFunctionResource) IsCommonTerminal() bool {
	return false
}

func (r *testStratosLambdaFunctionResource) GetType() string {
	return "stratosaws/lambda/function"
}

// StageChanges is not used for spec link info!
func (r *testStratosLambdaFunctionResource) StageChanges(
	ctx context.Context,
	resourceInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (provider.Changes, error) {
	return provider.Changes{}, nil
}

// Validate is not used for spec link info!
func (r *testStratosLambdaFunctionResource) Validate(ctx context.Context, schemaResource *schema.Resource, params core.BlueprintParams) error {
	return nil
}

// Deploy is not used for spec link info!
func (r *testStratosLambdaFunctionResource) Deploy(ctx context.Context, changes provider.Changes, params core.BlueprintParams) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// GetExternalState is not used for spec link info!
func (r *testStratosLambdaFunctionResource) GetExternalState(ctx context.Context, instanceID string, revisionID string, resourceID string) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// Destroy is not used for spec link info!
func (r *testStratosLambdaFunctionResource) Destroy(ctx context.Context, instanceID string, revisionID string, resourceID string) error {
	return nil
}

type testDynamoDBTableResource struct{}

func (r *testDynamoDBTableResource) CanLinkTo() []string {
	return []string{"aws/dynamodb/stream"}
}

func (r *testDynamoDBTableResource) IsCommonTerminal() bool {
	return true
}

func (r *testDynamoDBTableResource) GetType() string {
	return "aws/dynamodb/table"
}

// StageChanges is not used for spec link info!
func (r *testDynamoDBTableResource) StageChanges(
	ctx context.Context,
	resourceInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (provider.Changes, error) {
	return provider.Changes{}, nil
}

// Validate is not used for spec link info!
func (r *testDynamoDBTableResource) Validate(ctx context.Context, schemaResource *schema.Resource, params core.BlueprintParams) error {
	return nil
}

// Deploy is not used for spec link info!
func (r *testDynamoDBTableResource) Deploy(ctx context.Context, changes provider.Changes, params core.BlueprintParams) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// GetExternalState is not used for spec link info!
func (r *testDynamoDBTableResource) GetExternalState(ctx context.Context, instanceID string, revisionID string, resourceID string) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// Destroy is not used for spec link info!
func (r *testDynamoDBTableResource) Destroy(ctx context.Context, instanceID string, revisionID string, resourceID string) error {
	return nil
}

type testDynamoDBStreamResource struct{}

func (r *testDynamoDBStreamResource) CanLinkTo() []string {
	return []string{"aws/lambda/function", "stratosaws/lambda/function"}
}

func (r *testDynamoDBStreamResource) IsCommonTerminal() bool {
	return false
}

func (r *testDynamoDBStreamResource) GetType() string {
	return "aws/dynamodb/stream"
}

// StageChanges is not used for spec link info!
func (r *testDynamoDBStreamResource) StageChanges(
	ctx context.Context,
	resourceInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (provider.Changes, error) {
	return provider.Changes{}, nil
}

// Validate is not used for spec link info!
func (r *testDynamoDBStreamResource) Validate(ctx context.Context, schemaResource *schema.Resource, params core.BlueprintParams) error {
	return nil
}

// Deploy is not used for spec link info!
func (r *testDynamoDBStreamResource) Deploy(ctx context.Context, changes provider.Changes, params core.BlueprintParams) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// GetExternalState is not used for spec link info!
func (r *testDynamoDBStreamResource) GetExternalState(ctx context.Context, instanceID string, revisionID string, resourceID string) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// Destroy is not used for spec link info!
func (r *testDynamoDBStreamResource) Destroy(ctx context.Context, instanceID string, revisionID string, resourceID string) error {
	return nil
}

type testIAMRoleResource struct{}

func (r *testIAMRoleResource) CanLinkTo() []string {
	// "aws/lambda/function" is included here to test catching circular links.
	return []string{"aws/iam/policy", "aws/lambda/function"}
}

func (r *testIAMRoleResource) IsCommonTerminal() bool {
	return false
}

func (r *testIAMRoleResource) GetType() string {
	return "aws/iam/role"
}

// StageChanges is not used for spec link info!
func (r *testIAMRoleResource) StageChanges(
	ctx context.Context,
	resourceInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (provider.Changes, error) {
	return provider.Changes{}, nil
}

// Validate is not used for spec link info!
func (r *testIAMRoleResource) Validate(ctx context.Context, schemaResource *schema.Resource, params core.BlueprintParams) error {
	return nil
}

// Deploy is not used for spec link info!
func (r *testIAMRoleResource) Deploy(ctx context.Context, changes provider.Changes, params core.BlueprintParams) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// GetExternalState is not used for spec link info!
func (r *testIAMRoleResource) GetExternalState(ctx context.Context, instanceID string, revisionID string, resourceID string) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// Destroy is not used for spec link info!
func (r *testIAMRoleResource) Destroy(ctx context.Context, instanceID string, revisionID string, resourceID string) error {
	return nil
}

type testStratosIAMRoleResource struct{}

func (r *testStratosIAMRoleResource) CanLinkTo() []string {
	return []string{"aws/iam/policy", "aws/lambda/function"}
}

func (r *testStratosIAMRoleResource) IsCommonTerminal() bool {
	return false
}

func (r *testStratosIAMRoleResource) GetType() string {
	return "stratosaws/iam/role"
}

// StageChanges is not used for spec link info!
func (r *testStratosIAMRoleResource) StageChanges(
	ctx context.Context,
	resourceInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (provider.Changes, error) {
	return provider.Changes{}, nil
}

// Validate is not used for spec link info!
func (r *testStratosIAMRoleResource) Validate(ctx context.Context, schemaResource *schema.Resource, params core.BlueprintParams) error {
	return nil
}

// Deploy is not used for spec link info!
func (r *testStratosIAMRoleResource) Deploy(ctx context.Context, changes provider.Changes, params core.BlueprintParams) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// GetExternalState is not used for spec link info!
func (r *testStratosIAMRoleResource) GetExternalState(ctx context.Context, instanceID string, revisionID string, resourceID string) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// Destroy is not used for spec link info!
func (r *testStratosIAMRoleResource) Destroy(ctx context.Context, instanceID string, revisionID string, resourceID string) error {
	return nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testApiGatewayLambdaLink struct{}

// StageChanges is not used for spec link info!
func (l *testApiGatewayLambdaLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

// PriorityResourceType is not used for spec link info!
func (l *testApiGatewayLambdaLink) PriorityResourceType() string {
	return ""
}

func (l *testApiGatewayLambdaLink) Type() provider.LinkType {
	return provider.LinkTypeSoft
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testApiGatewayLambdaLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testApiGatewayLambdaLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testApiGatewayLambdaLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testSQSQueueLambdaLink struct{}

// StageChanges is not used for spec link info!
func (l *testSQSQueueLambdaLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

// PriorityResourceType is not used for spec link info!
func (l *testSQSQueueLambdaLink) PriorityResourceType() string {
	return ""
}

func (l *testSQSQueueLambdaLink) Type() provider.LinkType {
	return provider.LinkTypeSoft
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testSQSQueueLambdaLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testSQSQueueLambdaLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testSQSQueueLambdaLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testLambdaDynamoDBTableLink struct{}

// StageChanges is not used for spec link info!
func (l *testLambdaDynamoDBTableLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

// PriorityResourceType is not used for spec link info!
func (l *testLambdaDynamoDBTableLink) PriorityResourceType() string {
	return ""
}

func (l *testLambdaDynamoDBTableLink) Type() provider.LinkType {
	// For test purposes only, does not reflect reality!
	return provider.LinkTypeHard
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testLambdaDynamoDBTableLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testLambdaDynamoDBTableLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testLambdaDynamoDBTableLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testStratosLambdaDynamoDBTableLink struct{}

// StageChanges is not used for spec link info!
func (l *testStratosLambdaDynamoDBTableLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

// PriorityResourceType is not used for spec link info!
func (l *testStratosLambdaDynamoDBTableLink) PriorityResourceType() string {
	return ""
}

func (l *testStratosLambdaDynamoDBTableLink) Type() provider.LinkType {
	return provider.LinkTypeSoft
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testStratosLambdaDynamoDBTableLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testStratosLambdaDynamoDBTableLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testStratosLambdaDynamoDBTableLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testLambdaSQSQueueLink struct{}

// StageChanges is not used for spec link info!
func (l *testLambdaSQSQueueLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

// PriorityResourceType is not used for spec link info!
func (l *testLambdaSQSQueueLink) PriorityResourceType() string {
	return ""
}

func (l *testLambdaSQSQueueLink) Type() provider.LinkType {
	return provider.LinkTypeSoft
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testLambdaSQSQueueLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testLambdaSQSQueueLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testLambdaSQSQueueLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testDynamoDBTableStreamLink struct{}

// StageChanges is not used for spec link info!
func (l *testDynamoDBTableStreamLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

// PriorityResourceType is not used for spec link info!
func (l *testDynamoDBTableStreamLink) PriorityResourceType() string {
	return ""
}

func (l *testDynamoDBTableStreamLink) Type() provider.LinkType {
	// The DynamoDB table must exist before the stream.
	return provider.LinkTypeHard
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testDynamoDBTableStreamLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testDynamoDBTableStreamLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testDynamoDBTableStreamLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testDynamoDBStreamLambdaLink struct{}

// StageChanges is not used for spec link info!
func (l *testDynamoDBStreamLambdaLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

// PriorityResourceType is not used for spec link info!
func (l *testDynamoDBStreamLambdaLink) PriorityResourceType() string {
	return ""
}

func (l *testDynamoDBStreamLambdaLink) Type() provider.LinkType {
	// For test purposes only, does not reflect reality!
	return provider.LinkTypeHard
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testDynamoDBStreamLambdaLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testDynamoDBStreamLambdaLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testDynamoDBStreamLambdaLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testDynamoDBStreamStratosLambdaLink struct{}

// StageChanges is not used for spec link info!
func (l *testDynamoDBStreamStratosLambdaLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

// PriorityResourceType is not used for spec link info!
func (l *testDynamoDBStreamStratosLambdaLink) PriorityResourceType() string {
	return ""
}

func (l *testDynamoDBStreamStratosLambdaLink) Type() provider.LinkType {
	// For test purposes only, does not reflect reality!
	return provider.LinkTypeHard
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testDynamoDBStreamStratosLambdaLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testDynamoDBStreamStratosLambdaLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testDynamoDBStreamStratosLambdaLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testIAMRoleLambdaLink struct{}

// StageChanges is not used for spec link info!
func (l *testIAMRoleLambdaLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

// PriorityResourceType is not used for spec link info!
func (l *testIAMRoleLambdaLink) PriorityResourceType() string {
	return ""
}

func (l *testIAMRoleLambdaLink) Type() provider.LinkType {
	// For test purposes only, does not reflect reality!
	return provider.LinkTypeHard
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testIAMRoleLambdaLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testIAMRoleLambdaLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testIAMRoleLambdaLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testStratosIAMRoleLambdaLink struct{}

// StageChanges is not used for spec link info!
func (l *testStratosIAMRoleLambdaLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

// PriorityResourceType is not used for spec link info!
func (l *testStratosIAMRoleLambdaLink) PriorityResourceType() string {
	return ""
}

func (l *testStratosIAMRoleLambdaLink) Type() provider.LinkType {
	return provider.LinkTypeSoft
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testStratosIAMRoleLambdaLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testStratosIAMRoleLambdaLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testStratosIAMRoleLambdaLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testLambdaIAMRoleLink struct{}

// StageChanges is not used for spec link info!
func (l *testLambdaIAMRoleLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

// PriorityResourceType is not used for spec link info!
func (l *testLambdaIAMRoleLink) PriorityResourceType() string {
	return ""
}

func (l *testLambdaIAMRoleLink) Type() provider.LinkType {
	// For test purposes only, does not reflect reality!
	return provider.LinkTypeHard
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testLambdaIAMRoleLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testLambdaIAMRoleLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testLambdaIAMRoleLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// The functionality provided by link implementations is not used for building
// chain links. The spec link info behaviour that builds out the chain links
// prepares link implementations so they can be used by the blueprint container.
type testLambdaStratosIAMRoleLink struct{}

// StageChanges is not used for spec link info!
func (l *testLambdaStratosIAMRoleLink) StageChanges(
	ctx context.Context,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (provider.LinkChanges, error) {
	return provider.LinkChanges{}, nil
}

// PriorityResourceType is not used for spec link info!
func (l *testLambdaStratosIAMRoleLink) PriorityResourceType() string {
	return ""
}

func (l *testLambdaStratosIAMRoleLink) Type() provider.LinkType {
	return provider.LinkTypeSoft
}

// HandleResourceTypeAError is not used for spec link info!
func (l *testLambdaStratosIAMRoleLink) HandleResourceTypeAError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// HandleResourceTypeBError is not used for spec link info!
func (l *testLambdaStratosIAMRoleLink) HandleResourceTypeBError(ctx context.Context, resourceInfo *provider.ResourceInfo) error {
	return nil
}

// Deploy is not used for spec link info!
func (l *testLambdaStratosIAMRoleLink) Deploy(
	ctx context.Context,
	changes provider.LinkChanges,
	resourceAInfo *provider.ResourceInfo,
	resourceBInfo *provider.ResourceInfo,
	params core.BlueprintParams,
) (state.ResourceState, error) {
	return state.ResourceState{}, nil
}

// Provides a version of a chain link purely for snapshot tests.
// this simplifies the linked from references to a slice of resource names.
type snapshotChainLink struct {
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
	LinksTo []*snapshotChainLink
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
