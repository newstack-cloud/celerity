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
	descriptionInfo := ResourceLambdaFunctionTypeDescription()
	examples := ResourceLambdaFunctionExamples()
	return &providerv1.ResourceDefinition{
		Type:                   "aws/lambda/function",
		Label:                  "AWS Lambda Function",
		Schema:                 ResourceLambdaFunctionSchema(),
		PlainTextDescription:   descriptionInfo.PlainTextDescription,
		FormattedDescription:   descriptionInfo.MarkdownDescription,
		PlainTextSummary:       descriptionInfo.PlainTextSummary,
		FormattedSummary:       descriptionInfo.MarkdownSummary,
		PlainTextExamples:      examples.PlainTextExamples,
		FormattedExamples:      examples.MarkdownExamples,
		IDField:                "arn",
		ResourceCanLinkTo:      []string{"aws/dynamodb/table"},
		StabilisedDependencies: []string{"aws/sqs/queue"},
		DeployFunc: providerv1.RetryableReturnValue(
			deployLambdaFunction,
			func(err error) bool {
				return true
			},
		),
		CustomValidateFunc:   customValidateLambdaFunction,
		GetExternalStateFunc: getLambdaFunctionExternalState,
	}
}

func ResourceLambdaFunctionTypeDescription() *provider.ResourceGetTypeDescriptionOutput {
	return &provider.ResourceGetTypeDescriptionOutput{
		PlainTextDescription: "An AWS Lambda Function for running code in the cloud",
		MarkdownDescription:  "An **AWS** Lambda Function for running code in the cloud",
		PlainTextSummary:     "An AWS Lambda Function",
		MarkdownSummary:      "An **AWS** Lambda Function",
	}
}

func ResourceLambdaFunctionExamples() *provider.ResourceGetExamplesOutput {
	return &provider.ResourceGetExamplesOutput{
		MarkdownExamples: []string{
			"```yaml\nresources:\n  - type: aws/lambda/function\n    name: example-function\n```",
		},
		PlainTextExamples: []string{
			"resources:\n  - type: aws/lambda/function\n    name: example-function\n",
		},
	}
}

func ResourceLambdaDeployOutput() *provider.ResourceDeployOutput {
	return &provider.ResourceDeployOutput{
		ComputedFieldValues: map[string]*core.MappingNode{
			"spec.arn": core.MappingNodeFromString(
				"arn:aws:lambda:us-west-2:123456789012:function:processOrderFunction_0",
			),
		},
	}
}

func deployLambdaFunction(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return ResourceLambdaDeployOutput(), nil
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

func getLambdaFunctionExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{
		ResourceSpecState: ResourceLambdaFunctionExternalState(),
	}, nil
}

func ResourceLambdaFunctionExternalState() *core.MappingNode {
	return &core.MappingNode{
		Fields: map[string]*core.MappingNode{
			"arn": core.MappingNodeFromString(
				"arn:aws:lambda:us-west-2:123456789012:function:processOrderFunction_0",
			),
			"functionName": core.MappingNodeFromString("Process-Order-Function-0"),
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
