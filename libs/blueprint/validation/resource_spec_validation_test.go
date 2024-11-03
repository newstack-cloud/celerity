package validation

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/corefunctions"
	"github.com/two-hundred/celerity/libs/blueprint/internal"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/resourcehelpers"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	. "gopkg.in/check.v1"
)

type ResourceSpecValidationTestSuite struct {
	funcRegistry      provider.FunctionRegistry
	refChainCollector RefChainCollector
	resourceRegistry  resourcehelpers.Registry
}

var _ = Suite(&ResourceSpecValidationTestSuite{})

func (s *ResourceSpecValidationTestSuite) SetUpTest(c *C) {
	s.funcRegistry = &internal.FunctionRegistryMock{
		Functions: map[string]provider.Function{
			"trim":       corefunctions.NewTrimFunction(),
			"trimprefix": corefunctions.NewTrimPrefixFunction(),
			"list":       corefunctions.NewListFunction(),
			"object":     corefunctions.NewObjectFunction(),
			"jsondecode": corefunctions.NewJSONDecodeFunction(),
			"split":      corefunctions.NewSplitFunction(),
			"len":        corefunctions.NewLenFunction(),
		},
	}
	s.refChainCollector = NewRefChainCollector()
	s.resourceRegistry = &internal.ResourceRegistryMock{
		Resources: map[string]provider.Resource{
			"test/missingSpecDef":  newTestResourceMissingSpecDef(),
			"test/missingSchema":   newTestResourceMissingSchema(),
			"test/exampleResource": newSpecValidationTestExampleResource(),
		},
	}
}

func (s *ResourceSpecValidationTestSuite) Test_successfully_valid_resource(c *C) {
	resource := createTestValidResource()

	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testHandler": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}
	diagnostics, err := ValidateResource(
		context.Background(),
		"testHandler",
		resource,
		resourceMap,
		blueprint,
		&testBlueprintParams{},
		s.funcRegistry,
		s.refChainCollector,
		s.resourceRegistry,
	)
	c.Assert(diagnostics, HasLen, 0)
	c.Assert(err, IsNil)
}

func (s *ResourceSpecValidationTestSuite) Test_reports_error_for_resource_type_with_missing_spec_definition(c *C) {
	name := "Resource"
	handler := "test.handler"

	resource := &schema.Resource{
		Type: &schema.ResourceTypeWrapper{Value: "test/missingSpecDef"},
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
			"testHandler": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testHandler",
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
		"blueprint load error: validation failed due to a missing spec definition for resource \"testHandler\" "+
			"of type \"test/missingSpecDef\": spec definition is nil",
	)
}

func (s *ResourceSpecValidationTestSuite) Test_reports_error_for_resource_type_with_missing_schema(c *C) {
	name := "Resource"
	handler := "test.handler"

	resource := &schema.Resource{
		Type: &schema.ResourceTypeWrapper{Value: "test/missingSchema"},
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
			"testHandler": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testHandler",
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
		"blueprint load error: validation failed due to a missing spec definition schema for resource \"testHandler\" "+
			"of type \"test/missingSchema\"",
	)
}

func (s *ResourceSpecValidationTestSuite) Test_reports_error_for_empty_required_object_field(c *C) {
	resource := createTestValidResource()
	resource.Spec = nil
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testHandler": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testHandler",
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
		"blueprint load error: validation failed due to an empty resource item at path "+
			"\"resources.testHandler.spec\" where the object type was expected",
	)
}

func (s *ResourceSpecValidationTestSuite) Test_reports_error_for_computed_field_defined_in_blueprint(c *C) {
	resource := createTestValidResource()
	idValue := "id-value"
	resource.Spec.Fields["map"].Fields["item1"] = &core.MappingNode{
		Fields: map[string]*core.MappingNode{
			"id": {
				Literal: &core.ScalarValue{
					StringValue: &idValue,
				},
			},
		},
	}
	computedValue := "test-computed-value"
	resource.Spec.Fields["computed"] = &core.MappingNode{
		Literal: &core.ScalarValue{
			StringValue: &computedValue,
		},
	}

	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testHandler": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testHandler",
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
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeComputedFieldInBlueprint)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to \"resources.testHandler.spec.computed\" being a "+
			"computed field defined in the blueprint for resource \"testHandler\", this field is computed by the provider "+
			"after the resource has been created",
	)
}

func (s *ResourceSpecValidationTestSuite) Test_reports_error_for_missing_required_field(c *C) {
	resource := createTestValidResource()
	floatVal := 4039.21
	resource.Spec = &core.MappingNode{
		Fields: map[string]*core.MappingNode{
			"array": {
				Items: []*core.MappingNode{
					{
						Literal: &core.ScalarValue{
							FloatValue: &floatVal,
						},
					},
				},
			},
		},
	}
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testHandler": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testHandler",
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
		"blueprint load error: validation failed due to a missing required field \"map\" of type map at path "+
			"\"resources.testHandler.spec.map\"",
	)
}

func (s *ResourceSpecValidationTestSuite) Test_reports_error_for_empty_required_map_field(c *C) {
	resource := createTestValidResource()
	floatVal := 4039.21
	resource.Spec = &core.MappingNode{
		Fields: map[string]*core.MappingNode{
			"array": {
				Items: []*core.MappingNode{
					{
						Literal: &core.ScalarValue{
							FloatValue: &floatVal,
						},
					},
				},
			},
			"map": nil,
		},
	}
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testHandler": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testHandler",
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
		"blueprint load error: validation failed due to an empty resource item at path "+
			"\"resources.testHandler.spec.map\" where the map type was expected",
	)
}

func (s *ResourceSpecValidationTestSuite) Test_reports_error_for_empty_required_array_field(c *C) {
	resource := createTestValidResource()
	resource.Spec.Fields["array"] = nil
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testHandler": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testHandler",
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
		"blueprint load error: validation failed due to an empty resource item at path "+
			"\"resources.testHandler.spec.array\" where the array type was expected",
	)
}

func (s *ResourceSpecValidationTestSuite) Test_reports_error_for_empty_string_field(c *C) {
	resource := createTestValidResource()
	resource.Spec.Fields["map"].Fields["item1"] = &core.MappingNode{
		Fields: map[string]*core.MappingNode{
			"id": {
				Literal: nil,
			},
		},
	}
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testHandler": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testHandler",
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
		"blueprint load error: validation failed due to an empty resource item at path "+
			"\"resources.testHandler.spec.map.item1.id\" where the string type was expected",
	)
}

func (s *ResourceSpecValidationTestSuite) Test_reports_error_for_empty_string_field_value(c *C) {
	resource := createTestValidResource()
	resource.Spec.Fields["map"].Fields["item1"] = &core.MappingNode{
		Fields: map[string]*core.MappingNode{
			"id": {
				Literal: &core.ScalarValue{
					StringValue: nil,
				},
			},
		},
	}
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testHandler": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testHandler",
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
		"blueprint load error: validation failed due to an empty resource item at path "+
			"\"resources.testHandler.spec.map.item1.id\" where the string type was expected",
	)
}

func (s *ResourceSpecValidationTestSuite) Test_reports_error_for_invalid_type_for_string_field(c *C) {
	resource := createTestValidResource()
	testIntVal := 502012
	resource.Spec.Fields["map"].Fields["item1"] = &core.MappingNode{
		Fields: map[string]*core.MappingNode{
			"id": {
				Literal: &core.ScalarValue{
					IntValue: &testIntVal,
				},
			},
		},
	}
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testHandler": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testHandler",
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
		"blueprint load error: validation failed due to an invalid resource item at path "+
			"\"resources.testHandler.spec.map.item1.id\" where the string type was expected, but integer was found",
	)
}

func (s *ResourceSpecValidationTestSuite) Test_reports_error_for_union_invalid_substitution_resolved_type(c *C) {
	resource := createTestValidResource()
	testStrVal := "testString"
	resource.Spec.Fields["array"] = &core.MappingNode{
		Items: []*core.MappingNode{
			{
				StringWithSubstitutions: &substitutions.StringOrSubstitutions{
					Values: []*substitutions.StringOrSubstitution{
						{
							SubstitutionValue: &substitutions.Substitution{
								Function: &substitutions.SubstitutionFunctionExpr{
									FunctionName: "len",
									Arguments: []*substitutions.SubstitutionFunctionArg{
										{
											Value: &substitutions.Substitution{
												StringValue: &testStrVal,
											},
										},
									},
								},
							},
						},
						{
							StringValue: &testStrVal,
						},
					},
				},
			},
		},
	}
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testHandler": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testHandler",
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
		"blueprint load error: validation failed due to an invalid resource item found at path "+
			"\"resources.testHandler.spec.array[0]\" where one of the types (float | integer | boolean | object)"+
			" was expected",
	)
}

func (s *ResourceSpecValidationTestSuite) Test_reports_error_for_invalid_mapping_node_type_expecting_object(c *C) {
	resource := createTestValidResource()
	testStrVal := "testString"
	resource.Spec = &core.MappingNode{
		Literal: &core.ScalarValue{
			StringValue: &testStrVal,
		},
	}
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testHandler": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testHandler",
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
		"blueprint load error: validation failed due to an invalid resource item at path "+
			"\"resources.testHandler.spec\" where the object type was expected, but string was found",
	)
}

func (s *ResourceSpecValidationTestSuite) Test_reports_error_for_invalid_mapping_node_type_expecting_map(c *C) {
	resource := createTestValidResource()
	testStrVal := "testString"
	resource.Spec.Fields["map"] = &core.MappingNode{
		Literal: &core.ScalarValue{
			StringValue: &testStrVal,
		},
	}
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testHandler": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testHandler",
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
		"blueprint load error: validation failed due to an invalid resource item at path "+
			"\"resources.testHandler.spec.map\" where the map type was expected, but string was found",
	)
}

func (s *ResourceSpecValidationTestSuite) Test_reports_error_for_invalid_mapping_node_type_expecting_array(c *C) {
	resource := createTestValidResource()
	testStrVal := "testString"
	resource.Spec.Fields["array"] = &core.MappingNode{
		Literal: &core.ScalarValue{
			StringValue: &testStrVal,
		},
	}
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"testHandler": resource,
		},
	}

	blueprint := &schema.Blueprint{
		Resources: resourceMap,
	}

	diagnostics, err := ValidateResource(
		context.Background(),
		"testHandler",
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
		"blueprint load error: validation failed due to an invalid resource item at path "+
			"\"resources.testHandler.spec.array\" where the array type was expected, but string was found",
	)
}

//////////////////////////////////////////////////
// Test resources
//////////////////////////////////////////////////

func createTestValidResource() *schema.Resource {
	mappingItemId1 := "testId1"
	mappingItemId2 := "testId2"
	arrayValFloat := 4039.210
	lenStrValue := "testString"
	testIntVal := 504982

	return &schema.Resource{
		Type: &schema.ResourceTypeWrapper{Value: "test/exampleResource"},
		Spec: &core.MappingNode{
			Fields: map[string]*core.MappingNode{
				"map": {
					Fields: map[string]*core.MappingNode{
						"item1": {
							Fields: map[string]*core.MappingNode{
								"id": {
									Literal: &core.ScalarValue{
										StringValue: &mappingItemId1,
									},
								},
							},
						},
						"item2": {
							Fields: map[string]*core.MappingNode{
								"id": {
									StringWithSubstitutions: &substitutions.StringOrSubstitutions{
										Values: []*substitutions.StringOrSubstitution{
											{
												SubstitutionValue: &substitutions.Substitution{
													StringValue: &mappingItemId2,
												},
											},
										},
									},
								},
							},
						},
					},
				},
				"array": {
					Items: []*core.MappingNode{
						{
							Literal: &core.ScalarValue{
								FloatValue: &arrayValFloat,
							},
						},
						{
							StringWithSubstitutions: &substitutions.StringOrSubstitutions{
								Values: []*substitutions.StringOrSubstitution{
									{
										SubstitutionValue: &substitutions.Substitution{
											// Yields an integer value.
											Function: &substitutions.SubstitutionFunctionExpr{
												FunctionName: "len",
												Arguments: []*substitutions.SubstitutionFunctionArg{
													{
														Value: &substitutions.Substitution{
															StringValue: &lenStrValue,
														},
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Literal: &core.ScalarValue{
								BoolValue: &True,
							},
						},
						{
							Fields: map[string]*core.MappingNode{
								"value": {
									Literal: &core.ScalarValue{
										IntValue: &testIntVal,
									},
								},
							},
						},
					},
				},
				"nullable": {
					Literal: &core.ScalarValue{
						StringValue: nil,
					},
				},
			},
		},
	}
}

//////////////////////////////////////////////////
// Test resource type implementations
//////////////////////////////////////////////////

type testResourceMissingSpecDef struct{}

func newTestResourceMissingSpecDef() provider.Resource {
	return &testResourceMissingSpecDef{}
}

// CanLinkTo is not used for validation!
func (r *testResourceMissingSpecDef) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{}, nil
}

// StabilisedDependencies is not used for validation!
func (r *testResourceMissingSpecDef) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{}, nil
}

// IsCommonTerminal is not used for validation!
func (r *testResourceMissingSpecDef) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: false,
	}, nil
}

func (r *testResourceMissingSpecDef) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "test/missingSpecDef",
	}, nil
}

func (r *testResourceMissingSpecDef) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		MarkdownDescription:  "",
		PlainTextDescription: "",
	}, nil
}

func (r *testResourceMissingSpecDef) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

func (r *testResourceMissingSpecDef) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return &provider.ResourceGetSpecDefinitionOutput{
		SpecDefinition: nil,
	}, nil
}

// Deploy is not used for validation!
func (r *testResourceMissingSpecDef) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

// GetExternalState is not used for validation!
func (r *testResourceMissingSpecDef) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

// Destroy is not used for validation!
func (r *testResourceMissingSpecDef) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}

type testResourceMissingSchema struct{}

func newTestResourceMissingSchema() provider.Resource {
	return &testResourceMissingSchema{}
}

// CanLinkTo is not used for validation!
func (r *testResourceMissingSchema) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{}, nil
}

// StabilisedDependencies is not used for validation!
func (r *testResourceMissingSchema) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{}, nil
}

// IsCommonTerminal is not used for validation!
func (r *testResourceMissingSchema) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: false,
	}, nil
}

func (r *testResourceMissingSchema) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "test/missingSchema",
	}, nil
}

func (r *testResourceMissingSchema) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		MarkdownDescription:  "",
		PlainTextDescription: "",
	}, nil
}

func (r *testResourceMissingSchema) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

func (r *testResourceMissingSchema) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return &provider.ResourceGetSpecDefinitionOutput{
		SpecDefinition: &provider.ResourceSpecDefinition{
			Schema: nil,
		},
	}, nil
}

// Deploy is not used for validation!
func (r *testResourceMissingSchema) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

// GetExternalState is not used for validation!
func (r *testResourceMissingSchema) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

// Destroy is not used for validation!
func (r *testResourceMissingSchema) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}

type specValidationTestExampleResource struct{}

func newSpecValidationTestExampleResource() provider.Resource {
	return &specValidationTestExampleResource{}
}

// CanLinkTo is not used for validation!
func (r *specValidationTestExampleResource) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{}, nil
}

// StabilisedDependencies is not used for validation!
func (r *specValidationTestExampleResource) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{}, nil
}

// IsCommonTerminal is not used for validation!
func (r *specValidationTestExampleResource) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: false,
	}, nil
}

func (r *specValidationTestExampleResource) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "test/exampleResource",
	}, nil
}

func (r *specValidationTestExampleResource) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		MarkdownDescription:  "",
		PlainTextDescription: "",
	}, nil
}

func (r *specValidationTestExampleResource) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

func (r *specValidationTestExampleResource) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return &provider.ResourceGetSpecDefinitionOutput{
		SpecDefinition: &provider.ResourceSpecDefinition{
			Schema: &provider.ResourceDefinitionsSchema{
				Type:     provider.ResourceDefinitionsSchemaTypeObject,
				Required: []string{"map", "array"},
				Attributes: map[string]*provider.ResourceDefinitionsSchema{
					"map": {
						Type: provider.ResourceDefinitionsSchemaTypeMap,
						MapValues: &provider.ResourceDefinitionsSchema{
							Type:     provider.ResourceDefinitionsSchemaTypeObject,
							Required: []string{"id"},
							Attributes: map[string]*provider.ResourceDefinitionsSchema{
								"id": {
									Type: provider.ResourceDefinitionsSchemaTypeString,
								},
							},
						},
					},
					"array": {
						Type: provider.ResourceDefinitionsSchemaTypeArray,
						Items: &provider.ResourceDefinitionsSchema{
							Type: provider.ResourceDefinitionsSchemaTypeUnion,
							OneOf: []*provider.ResourceDefinitionsSchema{
								{
									Type: provider.ResourceDefinitionsSchemaTypeFloat,
								},
								{
									Type: provider.ResourceDefinitionsSchemaTypeInteger,
								},
								{
									Type: provider.ResourceDefinitionsSchemaTypeBoolean,
								},
								{
									Type: provider.ResourceDefinitionsSchemaTypeObject,
									Attributes: map[string]*provider.ResourceDefinitionsSchema{
										"value": {
											Type: provider.ResourceDefinitionsSchemaTypeInteger,
										},
									},
								},
							},
						},
					},
					"optionalString": {
						Type: provider.ResourceDefinitionsSchemaTypeString,
					},
					"nullable": {
						Type:     provider.ResourceDefinitionsSchemaTypeString,
						Nullable: true,
					},
					"computed": {
						Type:     provider.ResourceDefinitionsSchemaTypeString,
						Computed: true,
					},
				},
			},
		},
	}, nil
}

// Deploy is not used for validation!
func (r *specValidationTestExampleResource) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

// GetExternalState is not used for validation!
func (r *specValidationTestExampleResource) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

// Destroy is not used for validation!
func (r *specValidationTestExampleResource) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}
