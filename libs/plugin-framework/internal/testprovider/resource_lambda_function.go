package testprovider

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	"github.com/two-hundred/celerity/libs/plugin-framework/sdk/providerv1"
)

func resourceLambdaFunction() provider.Resource {
	return &providerv1.ResourceDefinition{
		Type:              "aws/lambda/function",
		Label:             "AWS Lambda Function",
		Schema:            ResourceLambdaFunctionSchema(),
		IDField:           "arn",
		ResourceCanLinkTo: []string{"aws/dynamodb/table"},
		DeployFunc: providerv1.RetryableReturnValue(
			deployLambdaFunction,
			func(err error) bool {
				return true
			},
		),
		CustomValidateFunc: customValidateLambdaFunction,
	}
}

func deployLambdaFunction(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

func customValidateLambdaFunction(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return ResourceLambdaFunctionValidateOutput(), nil
}

// ResourceLambdaFunctionValidateOutput returns a stub validation output
// for the LambdaFunction resource.
func ResourceLambdaFunctionValidateOutput() *provider.ResourceValidateOutput {
	colAccuracy := substitutions.ColumnAccuracyExact
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{
			{
				Level:   core.DiagnosticLevelWarning,
				Message: "This is a warning about an invalid lambda function spec",
				Range: &core.DiagnosticRange{
					Start: &source.Meta{
						Position: source.Position{
							Line:   110,
							Column: 40,
						},
					},
					End: &source.Meta{
						Position: source.Position{
							Line:   110,
							Column: 80,
						},
					},
					ColumnAccuracy: &colAccuracy,
				},
			},
		},
	}
}

// ResourceLambdaFunctionSchema returns a stub spec definition
// for the LambdaFunction resource.
func ResourceLambdaFunctionSchema() *provider.ResourceDefinitionsSchema {
	return &provider.ResourceDefinitionsSchema{
		Type: provider.ResourceDefinitionsSchemaTypeObject,
		Attributes: map[string]*provider.ResourceDefinitionsSchema{
			"functionName": {
				Type:        provider.ResourceDefinitionsSchemaTypeString,
				Label:       "Function Name",
				Description: "The name of the Lambda function",
				Examples: []*core.MappingNode{
					core.MappingNodeFromString("example-function"),
				},
			},
			"arn": {
				Type:     provider.ResourceDefinitionsSchemaTypeString,
				Computed: true,
			},
		},
	}
}

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
