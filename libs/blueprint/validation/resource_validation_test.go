package validation

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/corefunctions"
	"github.com/two-hundred/celerity/libs/blueprint/errors"
	"github.com/two-hundred/celerity/libs/blueprint/internal"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/resourcehelpers"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	. "gopkg.in/check.v1"
)

var (
	// True represents a boolean true value
	// that can be used as a reference in tests.
	True = true
	// False represents a boolean false value
	// that can be used as a reference in tests.
	False = false
)

type ResourceValidationTestSuite struct {
	funcRegistry      provider.FunctionRegistry
	refChainCollector RefChainCollector
	resourceRegistry  resourcehelpers.Registry
}

var _ = Suite(&ResourceValidationTestSuite{})

func (s *ResourceValidationTestSuite) SetUpTest(c *C) {
	s.funcRegistry = &internal.FunctionRegistryMock{
		Functions: map[string]provider.Function{
			"trim":       corefunctions.NewTrimFunction(),
			"trimprefix": corefunctions.NewTrimPrefixFunction(),
			"list":       corefunctions.NewListFunction(),
			"object":     corefunctions.NewObjectFunction(),
			"jsondecode": corefunctions.NewJSONDecodeFunction(),
			"split":      corefunctions.NewSplitFunction(),
		},
	}
	s.refChainCollector = NewRefChainCollector()
	s.resourceRegistry = &internal.ResourceRegistryMock{
		Resources: map[string]provider.Resource{
			"aws/ecs/service": newTestECSServiceResource(),
		},
	}
}

func (s *ResourceValidationTestSuite) Test_reports_error_when_substitution_provided_in_resource_name(c *C) {
	description := "EC2 instance for the application"
	resourceSchema := &schema.Resource{
		Type: &schema.ResourceTypeWrapper{Value: "${variables.awsEC2InstanceName}"},
		Description: &substitutions.StringOrSubstitutions{
			Values: []*substitutions.StringOrSubstitution{
				{
					StringValue: &description,
				},
			},
		},
	}
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"${variables.awsEC2InstanceName}": resourceSchema,
		},
		SourceMeta: map[string]*source.Meta{
			"${variables.awsEC2InstanceName}": {
				Line:   1,
				Column: 1,
			},
		},
	}
	err := ValidateResourceName("${variables.awsEC2InstanceName}", resourceMap)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := err.(*errors.LoadError)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidResource)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: ${..} substitutions can not be used in resource names, "+
			"found in resource \"${variables.awsEC2InstanceName}\"",
	)
}

func (s *ResourceValidationTestSuite) Test_reports_errors_when_substitutions_used_in_spec_mapping_keys(c *C) {
	version := "1.0.0"
	resourceSchema := &schema.Resource{
		Type: &schema.ResourceTypeWrapper{Value: "celerity/api"},
		Spec: &core.MappingNode{
			Items: []*core.MappingNode{
				{
					Fields: map[string]*core.MappingNode{
						"${variables.version}": {
							Literal: &core.ScalarValue{
								StringValue: &version,
							},
						},
					},
					SourceMeta: &source.Meta{
						Line:   1,
						Column: 1,
					},
					FieldsSourceMeta: map[string]*source.Meta{
						"${variables.version}": {
							Line:   1,
							Column: 1,
						},
					},
				},
			},
		},
	}
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"api": resourceSchema,
		},
		SourceMeta: map[string]*source.Meta{
			"api": {
				Line:   1,
				Column: 1,
			},
		},
	}
	err := PreValidateResourceSpec(context.TODO(), "api", resourceSchema, resourceMap)
	c.Assert(err, NotNil)
}

func (s *ResourceValidationTestSuite) Test_reports_errors_when_resource_type_is_not_supported(c *C) {
	name := "Unknown Resource"
	handler := "unknown.handler"

	resource := &schema.Resource{
		Type: &schema.ResourceTypeWrapper{Value: "aws/lambda/unknown"},
		Metadata: &schema.Metadata{
			DisplayName: &substitutions.StringOrSubstitutions{
				Values: []*substitutions.StringOrSubstitution{
					{
						StringValue: &name,
					},
				},
			},
		},
		Spec: &core.MappingNode{
			Fields: map[string]*core.MappingNode{
				"handler": {
					Literal: &core.ScalarValue{
						StringValue: &handler,
					},
				},
			},
		},
	}
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"unknownHandler": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"unknownHandler",
		resource,
		resourceMap,
		blueprint,
		&testBlueprintParams{},
		s.funcRegistry,
		s.refChainCollector,
		s.resourceRegistry,
	)
	c.Assert(diagnostics, HasLen, 0)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := internal.UnpackLoadError(err)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidResource)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to resource \"unknownHandler\" having an "+
			"unsupported type \"aws/lambda/unknown\", this type is not made available by any of the loaded providers",
	)
}

func (s *ResourceValidationTestSuite) Test_reports_error_when_providing_a_display_name_with_wrong_sub_type(c *C) {
	resource := newTestInvalidDisplayNameResource()
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testService": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testService",
		resource,
		resourceMap,
		blueprint,
		&testBlueprintParams{},
		s.funcRegistry,
		s.refChainCollector,
		s.resourceRegistry,
	)
	c.Assert(diagnostics, HasLen, 0)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := internal.UnpackLoadError(err)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidSubstitution)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to an invalid substitution found in "+
			"\"resources.testService\", resolved type \"object\" is not supported by display names, "+
			"only values that resolve as strings are supported",
	)
}

func (s *ResourceValidationTestSuite) Test_reports_error_when_providing_a_description_with_wrong_sub_type(c *C) {
	resource := newTestInvalidDescriptionResource()
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testService": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testService",
		resource,
		resourceMap,
		blueprint,
		&testBlueprintParams{},
		s.funcRegistry,
		s.refChainCollector,
		s.resourceRegistry,
	)
	c.Assert(diagnostics, HasLen, 0)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := internal.UnpackLoadError(err)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidSubstitution)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to an invalid substitution found in "+
			"\"resources.testService\", resolved type \"object\" is not supported by descriptions, "+
			"only values that resolve as strings are supported",
	)
}

func (s *ResourceValidationTestSuite) Test_reports_error_when_metadata_label_key_has_substitution(c *C) {
	resource := newTestValidResource()
	resource.Metadata.Labels.Values["${variables.labelKey}"] = "test"
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testService": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testService",
		resource,
		resourceMap,
		blueprint,
		&testBlueprintParams{},
		s.funcRegistry,
		s.refChainCollector,
		s.resourceRegistry,
	)
	c.Assert(diagnostics, HasLen, 0)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := internal.UnpackLoadError(err)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidResource)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to a label key containing a substitution in resource \"testService\", "+
			"the label key \"${variables.labelKey}\" can not contain substitutions",
	)
}

func (s *ResourceValidationTestSuite) Test_reports_error_when_metadata_label_value_has_substitution(c *C) {
	resource := newTestValidResource()
	resource.Metadata.Labels.Values["app"] = "${variables.labelValue}"
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testService": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testService",
		resource,
		resourceMap,
		blueprint,
		&testBlueprintParams{},
		s.funcRegistry,
		s.refChainCollector,
		s.resourceRegistry,
	)
	c.Assert(diagnostics, HasLen, 0)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := internal.UnpackLoadError(err)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidResource)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to a label value containing a substitution in resource \"testService\", "+
			"the label \"app\" with value \"${variables.labelValue}\" can not contain substitutions",
	)
}

func (s *ResourceValidationTestSuite) Test_reports_error_when_annotation_key_has_substitution(c *C) {
	resource := newTestValidResource()
	annotationValue := "v1"
	resource.Metadata.Annotations.Values["${variables.annotationKey}"] = &substitutions.StringOrSubstitutions{
		Values: []*substitutions.StringOrSubstitution{
			{
				StringValue: &annotationValue,
			},
		},
	}
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testService": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testService",
		resource,
		resourceMap,
		blueprint,
		&testBlueprintParams{},
		s.funcRegistry,
		s.refChainCollector,
		s.resourceRegistry,
	)
	c.Assert(diagnostics, HasLen, 0)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := internal.UnpackLoadError(err)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidResource)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to an annotation key containing a substitution in resource \"testService\", "+
			"the annotation key \"${variables.annotationKey}\" can not contain substitutions",
	)
}

func (s *ResourceValidationTestSuite) Test_reports_error_when_nested_condition_is_empty(c *C) {
	resource := newTestValidResource()
	// Empty nested condition added to "and" list.
	resource.Condition.And = append(resource.Condition.And, &schema.Condition{})
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testService": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testService",
		resource,
		resourceMap,
		blueprint,
		&testBlueprintParams{},
		s.funcRegistry,
		s.refChainCollector,
		s.resourceRegistry,
	)
	c.Assert(diagnostics, HasLen, 0)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := internal.UnpackLoadError(err)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidResource)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to a nested condition for resource \"testService\" "+
			"being empty, all nested conditions must have a value defined",
	)
}

func (s *ResourceValidationTestSuite) Test_reports_error_when_condition_resolves_incorrect_type(c *C) {
	resource := newTestValidResource()
	resource.Condition.And = append(resource.Condition.And, &schema.Condition{
		StringValue: &substitutions.StringOrSubstitutions{
			Values: []*substitutions.StringOrSubstitution{
				{
					SubstitutionValue: &substitutions.Substitution{
						Function: &substitutions.SubstitutionFunctionExpr{
							FunctionName: "object",
							Arguments:    []*substitutions.SubstitutionFunctionArg{},
						},
					},
				},
			},
		},
	})
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testService": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testService",
		resource,
		resourceMap,
		blueprint,
		&testBlueprintParams{},
		s.funcRegistry,
		s.refChainCollector,
		s.resourceRegistry,
	)
	c.Assert(diagnostics, HasLen, 0)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := internal.UnpackLoadError(err)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidSubstitution)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to an invalid substitution found in "+
			"\"resources.testService\", resolved type \"object\" is not supported by conditions, "+
			"only values that resolve as booleans are supported",
	)
}

func (s *ResourceValidationTestSuite) Test_produces_warning_when_condition_resolves_any_type(c *C) {
	resource := newTestValidResource()
	boolJSON := "true"
	resource.Condition.And = append(resource.Condition.And, &schema.Condition{
		StringValue: &substitutions.StringOrSubstitutions{
			Values: []*substitutions.StringOrSubstitution{
				{
					SubstitutionValue: &substitutions.Substitution{
						Function: &substitutions.SubstitutionFunctionExpr{
							FunctionName: "jsondecode",
							Arguments: []*substitutions.SubstitutionFunctionArg{
								{
									Value: &substitutions.Substitution{
										StringValue: &boolJSON,
									},
								},
							},
						},
					},
				},
			},
		},
	})
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testService": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testService",
		resource,
		resourceMap,
		blueprint,
		&testBlueprintParams{},
		s.funcRegistry,
		s.refChainCollector,
		s.resourceRegistry,
	)

	c.Assert(err, IsNil)
	c.Assert(diagnostics, HasLen, 1)
	c.Assert(diagnostics[0].Level, Equals, core.DiagnosticLevelWarning)
	c.Assert(
		diagnostics[0].Message,
		Equals,
		"Substitution returns \"any\" type, this may produce unexpected output "+
			"in the condition, conditions are expected to be boolean values",
	)
}

func (s *ResourceValidationTestSuite) Test_produces_warning_when_each_resolves_any_type(c *C) {
	resource := newTestValidResource()
	arrJSON := "[]"
	resource.Each = &substitutions.StringOrSubstitutions{
		Values: []*substitutions.StringOrSubstitution{
			{
				SubstitutionValue: &substitutions.Substitution{
					Function: &substitutions.SubstitutionFunctionExpr{
						FunctionName: "jsondecode",
						Arguments: []*substitutions.SubstitutionFunctionArg{
							{
								Value: &substitutions.Substitution{
									StringValue: &arrJSON,
								},
							},
						},
					},
				},
			},
		},
	}
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testService": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testService",
		resource,
		resourceMap,
		blueprint,
		&testBlueprintParams{},
		s.funcRegistry,
		s.refChainCollector,
		s.resourceRegistry,
	)

	c.Assert(err, IsNil)
	c.Assert(diagnostics, HasLen, 1)
	c.Assert(diagnostics[0].Level, Equals, core.DiagnosticLevelWarning)
	c.Assert(
		diagnostics[0].Message,
		Equals,
		"Substitution returns \"any\" type, this may produce unexpected output "+
			"in each, an array is expected",
	)
}

func (s *ResourceValidationTestSuite) Test_reports_error_when_each_resolves_incorrect_type(c *C) {
	resource := newTestValidResource()
	resource.Each = &substitutions.StringOrSubstitutions{
		Values: []*substitutions.StringOrSubstitution{
			{
				SubstitutionValue: &substitutions.Substitution{
					BoolValue: &True,
				},
			},
		},
	}
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testService": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testService",
		resource,
		resourceMap,
		blueprint,
		&testBlueprintParams{},
		s.funcRegistry,
		s.refChainCollector,
		s.resourceRegistry,
	)
	c.Assert(diagnostics, HasLen, 0)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := internal.UnpackLoadError(err)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidSubstitution)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to an invalid substitution found in "+
			"\"resources.testService\", resolved type \"boolean\" is not supported in each, "+
			"only values that resolve as arrays are supported",
	)
}

func (s *ResourceValidationTestSuite) Test_reports_error_when_link_selector_label_key_has_substitution(c *C) {
	resource := newTestValidResource()
	resource.LinkSelector.ByLabel.Values["${variables.labelKey}"] = "test"
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testService": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testService",
		resource,
		resourceMap,
		blueprint,
		&testBlueprintParams{},
		s.funcRegistry,
		s.refChainCollector,
		s.resourceRegistry,
	)
	c.Assert(diagnostics, HasLen, 0)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := internal.UnpackLoadError(err)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidResource)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to a link selector \"byLabel\" key containing a substitution in resource \"testService\", "+
			"the link selector label key \"${variables.labelKey}\" can not contain substitutions",
	)
}

func (s *ResourceValidationTestSuite) Test_reports_error_when_link_selector_label_value_has_substitution(c *C) {
	resource := newTestValidResource()
	resource.LinkSelector.ByLabel.Values["app"] = "${variables.labelValue}"
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testService": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testService",
		resource,
		resourceMap,
		blueprint,
		&testBlueprintParams{},
		s.funcRegistry,
		s.refChainCollector,
		s.resourceRegistry,
	)
	c.Assert(diagnostics, HasLen, 0)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := internal.UnpackLoadError(err)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidResource)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to a link selector \"byLabel\" value containing a substitution in resource \"testService\", "+
			"the link selector label \"app\" with value \"${variables.labelValue}\" can not contain substitutions",
	)
}

func newTestValidResource() *schema.Resource {
	serviceName := "testService"
	displayNamePrefix := "Service-"
	serviceAnnotationPrefix := "service.v2024."
	serviceNo := "1"
	strToSplit := "a,b,c,d"
	delimiter := ","
	return &schema.Resource{
		Type: &schema.ResourceTypeWrapper{Value: "aws/ecs/service"},
		Metadata: &schema.Metadata{
			DisplayName: &substitutions.StringOrSubstitutions{
				Values: []*substitutions.StringOrSubstitution{
					{
						SubstitutionValue: &substitutions.Substitution{
							StringValue: &displayNamePrefix,
						},
					},
					{
						SubstitutionValue: &substitutions.Substitution{
							StringValue: &serviceNo,
						},
					},
				},
			},
			Labels: &schema.StringMap{
				Values: map[string]string{
					"service": "test",
				},
			},
			Annotations: &schema.StringOrSubstitutionsMap{
				Values: map[string]*substitutions.StringOrSubstitutions{
					"service.v1": {
						Values: []*substitutions.StringOrSubstitution{
							{
								StringValue: &serviceAnnotationPrefix,
							},
							{
								SubstitutionValue: &substitutions.Substitution{
									StringValue: &serviceNo,
								},
							},
						},
					},
				},
			},
		},
		LinkSelector: &schema.LinkSelector{
			ByLabel: &schema.StringMap{
				Values: map[string]string{
					"service": "test",
				},
			},
		},
		Each: &substitutions.StringOrSubstitutions{
			Values: []*substitutions.StringOrSubstitution{
				{
					SubstitutionValue: &substitutions.Substitution{
						Function: &substitutions.SubstitutionFunctionExpr{
							FunctionName: "split",
							Arguments: []*substitutions.SubstitutionFunctionArg{
								{
									Value: &substitutions.Substitution{
										StringValue: &strToSplit,
									},
								},
								{
									Value: &substitutions.Substitution{
										StringValue: &delimiter,
									},
								},
							},
						},
					},
				},
			},
		},
		Condition: &schema.Condition{
			And: []*schema.Condition{
				{
					Or: []*schema.Condition{
						{
							Not: &schema.Condition{
								StringValue: &substitutions.StringOrSubstitutions{
									Values: []*substitutions.StringOrSubstitution{
										{
											SubstitutionValue: &substitutions.Substitution{
												BoolValue: &True,
											},
										},
									},
								},
							},
						},
						{
							StringValue: &substitutions.StringOrSubstitutions{
								Values: []*substitutions.StringOrSubstitution{
									{
										SubstitutionValue: &substitutions.Substitution{
											BoolValue: &False,
										},
									},
								},
							},
						},
					},
				},
				{
					StringValue: &substitutions.StringOrSubstitutions{
						Values: []*substitutions.StringOrSubstitution{
							{
								SubstitutionValue: &substitutions.Substitution{
									BoolValue: &True,
								},
							},
						},
					},
				},
			},
		},
		Spec: &core.MappingNode{
			Fields: map[string]*core.MappingNode{
				"serviceName": {
					Literal: &core.ScalarValue{
						StringValue: &serviceName,
					},
				},
			},
		},
	}
}

func newTestInvalidDisplayNameResource() *schema.Resource {
	serviceName := "testService"
	displayNamePrefix := "Service-"
	return &schema.Resource{
		Type: &schema.ResourceTypeWrapper{Value: "aws/ecs/service"},
		Metadata: &schema.Metadata{
			DisplayName: &substitutions.StringOrSubstitutions{
				Values: []*substitutions.StringOrSubstitution{
					{
						SubstitutionValue: &substitutions.Substitution{
							StringValue: &displayNamePrefix,
						},
					},
					{
						SubstitutionValue: &substitutions.Substitution{
							Function: &substitutions.SubstitutionFunctionExpr{
								FunctionName: "object",
								Arguments:    []*substitutions.SubstitutionFunctionArg{},
							},
						},
					},
				},
			},
		},
		Spec: &core.MappingNode{
			Fields: map[string]*core.MappingNode{
				"serviceName": {
					Literal: &core.ScalarValue{
						StringValue: &serviceName,
					},
				},
			},
		},
	}
}

func newTestInvalidDescriptionResource() *schema.Resource {
	serviceName := "testService"
	displayName := "Test Service"
	return &schema.Resource{
		Type: &schema.ResourceTypeWrapper{Value: "aws/ecs/service"},
		Metadata: &schema.Metadata{
			DisplayName: &substitutions.StringOrSubstitutions{
				Values: []*substitutions.StringOrSubstitution{
					{
						SubstitutionValue: &substitutions.Substitution{
							StringValue: &displayName,
						},
					},
				},
			},
		},
		Spec: &core.MappingNode{
			Fields: map[string]*core.MappingNode{
				"serviceName": {
					Literal: &core.ScalarValue{
						StringValue: &serviceName,
					},
				},
			},
		},
		Description: &substitutions.StringOrSubstitutions{
			Values: []*substitutions.StringOrSubstitution{
				{
					SubstitutionValue: &substitutions.Substitution{
						Function: &substitutions.SubstitutionFunctionExpr{
							FunctionName: "object",
							Arguments:    []*substitutions.SubstitutionFunctionArg{},
						},
					},
				},
			},
		},
	}
}
