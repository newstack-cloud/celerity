package testprovider

// func lambdaFunction() provider.Resource {
// 	return providerv1.ResourceFromDefinition(
// 		providerv1.ResourceDefinition{
// 			Type:       "aws/lambda/function",
// 			Schema:     map[string]*schema.Schema{},
// 			DeployFunc: providerv1.RetryableReturnValue(deployLambdaFunction),
// 		},
// 	)
// }

// func deployLambdaFunction(
// 	ctx context.Context,
// 	params *provider.ResourceDeployParams,
// ) (*state.ResourceState, error) {
// 	return &state.ResourceState{}, nil
// }

// // LambdaFunction is the resource type implementation for AWS Lambda
// // functions.
// type LambdaFunction struct {
// 	resourceTypeSchema map[string]*schema.Schema
// }

// func (l *LambdaFunction) GetType() string {
// 	return "aws/lambda/function"
// }

// func (l *LambdaFunction) CanLinkTo() []string {
// 	return []string{}
// }

// func (l *LambdaFunction) Validate(
// 	ctx context.Context,
// 	schemaResource *bpschema.Resource,
// 	params core.BlueprintParams,
// ) ([]*core.Diagnostic, error) {
// 	// Example of using the schema validation helper here.
// 	// This is not required, but it is recommended to use the helper
// 	// to ensure that the resource is correctly defined.
// 	// This is more of a helper to be used as a library instead of a
// 	// a framework requirement.
// 	diagnostics, err := schema.ValidateResourceSchema(
// 		l.resourceTypeSchema,
// 		schemaResource,
// 		params,
// 	)
// 	if err != nil {
// 		return diagnostics, err
// 	}

// 	return nil, nil
// }

// func (l *LambdaFunction) IsCommonTerminal() bool {
// 	return false
// }

// // todo: add custom timeouts for each operation.
// // todo: add retryable wrappper util?
// func (l *LambdaFunction) StageChanges(
// 	ctx context.Context,
// 	resourceInfo *provider.ResourceInfo,
// 	params core.BlueprintParams,
// ) (provider.Changes, error) {
// 	return provider.Changes{}, nil
// }

// func (l *LambdaFunction) Deploy(
// 	ctx context.Context,
// 	changes provider.Changes,
// 	params core.BlueprintParams,
// ) (state.ResourceState, error) {
// 	return state.ResourceState{}, nil
// }

// func (l *LambdaFunction) GetExternalState(
// 	ctx context.Context,
// 	instanceID string,
// 	revisionID string,
// 	resourceID string,
// ) (state.ResourceState, error) {
// 	return state.ResourceState{}, nil
// }

// func (l *LambdaFunction) Destroy(
// 	ctx context.Context,
// 	instanceID string,
// 	revisionID string,
// 	resourceID string,
// ) error {
// 	return nil
// }
