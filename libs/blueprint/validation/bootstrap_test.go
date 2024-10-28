package validation

import (
	"context"
	"errors"
	"testing"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) {
	TestingT(t)
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
	input *provider.CustomVariableTypeOptionsInput,
) (*provider.CustomVariableTypeOptionsOutput, error) {
	t2nano := "t2.nano"
	t2micro := "t2.micro"
	t2small := "t2.small"
	t2medium := "t2.medium"
	t2large := "t2.large"
	t2xlarge := "t2.xlarge"
	t22xlarge := "t2.2xlarge"
	return &provider.CustomVariableTypeOptionsOutput{
		Options: map[string]*core.ScalarValue{
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
		},
	}, nil
}

func (t *testEC2InstanceTypeCustomVariableType) GetType(
	ctx context.Context,
	input *provider.CustomVariableTypeGetTypeInput,
) (*provider.CustomVariableTypeGetTypeOutput, error) {
	return &provider.CustomVariableTypeGetTypeOutput{
		Type: "aws/ec2/instanceType",
	}, nil
}

func (t *testEC2InstanceTypeCustomVariableType) GetDescription(
	ctx context.Context,
	input *provider.CustomVariableTypeGetDescriptionInput,
) (*provider.CustomVariableTypeGetDescriptionOutput, error) {
	return &provider.CustomVariableTypeGetDescriptionOutput{
		MarkdownDescription:  "",
		PlainTextDescription: "",
	}, nil
}

type testInvalidEC2InstanceTypeCustomVariableType struct{}

func (t *testInvalidEC2InstanceTypeCustomVariableType) Options(
	ctx context.Context,
	input *provider.CustomVariableTypeOptionsInput,
) (*provider.CustomVariableTypeOptionsOutput, error) {
	// Invalid due to mixed scalar types.
	t2nano := "t2.nano"
	t2micro := 54039
	t2small := "t2.small"
	t2medium := "t2.medium"
	t2large := 32192.49
	t2xlarge := "t2.xlarge"
	t22xlarge := true
	return &provider.CustomVariableTypeOptionsOutput{
		Options: map[string]*core.ScalarValue{
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
		},
	}, nil
}

func (t *testInvalidEC2InstanceTypeCustomVariableType) GetType(
	ctx context.Context,
	input *provider.CustomVariableTypeGetTypeInput,
) (*provider.CustomVariableTypeGetTypeOutput, error) {
	return &provider.CustomVariableTypeGetTypeOutput{
		Type: "aws/ec2/instanceType",
	}, nil
}

func (t *testInvalidEC2InstanceTypeCustomVariableType) GetDescription(
	ctx context.Context,
	input *provider.CustomVariableTypeGetDescriptionInput,
) (*provider.CustomVariableTypeGetDescriptionOutput, error) {
	return &provider.CustomVariableTypeGetDescriptionOutput{
		MarkdownDescription:  "",
		PlainTextDescription: "",
	}, nil
}

type testFailToLoadOptionsCustomVariableType struct{}

func (t *testFailToLoadOptionsCustomVariableType) Options(
	ctx context.Context,
	input *provider.CustomVariableTypeOptionsInput,
) (*provider.CustomVariableTypeOptionsOutput, error) {
	return nil, errors.New("failed to load options")
}

func (t *testFailToLoadOptionsCustomVariableType) GetType(
	ctx context.Context,
	input *provider.CustomVariableTypeGetTypeInput,
) (*provider.CustomVariableTypeGetTypeOutput, error) {
	return &provider.CustomVariableTypeGetTypeOutput{
		Type: "aws/ec2/instanceType",
	}, nil
}

func (t *testFailToLoadOptionsCustomVariableType) GetDescription(
	ctx context.Context,
	input *provider.CustomVariableTypeGetDescriptionInput,
) (*provider.CustomVariableTypeGetDescriptionOutput, error) {
	return &provider.CustomVariableTypeGetDescriptionOutput{
		MarkdownDescription:  "",
		PlainTextDescription: "",
	}, nil
}

type testRegionCustomVariableType struct{}

func (t *testRegionCustomVariableType) Options(
	ctx context.Context,
	input *provider.CustomVariableTypeOptionsInput,
) (*provider.CustomVariableTypeOptionsOutput, error) {
	usEast1 := "us-east-1"
	usEast2 := "us-east-2"
	usWest1 := "us-west-1"
	usWest2 := "us-west-2"
	euWest1 := "eu-west-1"
	euWest2 := "eu-west-2"
	euCentral1 := "eu-central-1"

	return &provider.CustomVariableTypeOptionsOutput{
		Options: map[string]*core.ScalarValue{
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
		},
	}, nil
}

func (t *testRegionCustomVariableType) GetType(
	ctx context.Context,
	input *provider.CustomVariableTypeGetTypeInput,
) (*provider.CustomVariableTypeGetTypeOutput, error) {
	return &provider.CustomVariableTypeGetTypeOutput{
		Type: "aws/region",
	}, nil
}

func (t *testRegionCustomVariableType) GetDescription(
	ctx context.Context,
	input *provider.CustomVariableTypeGetDescriptionInput,
) (*provider.CustomVariableTypeGetDescriptionOutput, error) {
	return &provider.CustomVariableTypeGetDescriptionOutput{
		MarkdownDescription:  "",
		PlainTextDescription: "",
	}, nil
}

type testExampleResource struct{}

// CanLinkTo is not used for validation!
func (r *testExampleResource) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{}, nil
}

// StabilisedDependencies is not used for validation!
func (r *testExampleResource) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{}, nil
}

// IsCommonTerminal is not used for validation!
func (r *testExampleResource) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: false,
	}, nil
}

func (r *testExampleResource) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "celerity/exampleResource",
	}, nil
}

func (r *testExampleResource) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		MarkdownDescription:  "",
		PlainTextDescription: "",
	}, nil
}

// StageChanges is not used for validation!
func (r *testExampleResource) StageChanges(
	ctx context.Context,
	input *provider.ResourceStageChangesInput,
) (*provider.ResourceStageChangesOutput, error) {
	return &provider.ResourceStageChangesOutput{}, nil
}

func (r *testExampleResource) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

func (r *testExampleResource) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return &provider.ResourceGetSpecDefinitionOutput{
		SpecDefinition: &provider.ResourceSpecDefinition{
			Schema: &provider.ResourceDefinitionsSchema{
				Type: provider.ResourceDefinitionsSchemaTypeObject,
				Attributes: map[string]*provider.ResourceDefinitionsSchema{
					"name": {
						Type: provider.ResourceDefinitionsSchemaTypeString,
					},
					"ids": {
						Type: provider.ResourceDefinitionsSchemaTypeArray,
						Items: &provider.ResourceDefinitionsSchema{
							Type: provider.ResourceDefinitionsSchemaTypeObject,
							Attributes: map[string]*provider.ResourceDefinitionsSchema{
								"name": {
									Type: provider.ResourceDefinitionsSchemaTypeString,
								},
							},
						},
					},
				},
			},
		},
	}, nil
}

func (r *testExampleResource) GetStateDefinition(
	ctx context.Context,
	input *provider.ResourceGetStateDefinitionInput,
) (*provider.ResourceGetStateDefinitionOutput, error) {
	return &provider.ResourceGetStateDefinitionOutput{
		StateDefinition: &provider.ResourceStateDefinition{
			Schema: &provider.ResourceDefinitionsSchema{
				Type: provider.ResourceDefinitionsSchemaTypeObject,
				Attributes: map[string]*provider.ResourceDefinitionsSchema{
					"name": {
						Type: provider.ResourceDefinitionsSchemaTypeString,
					},
				},
			},
		},
	}, nil
}

// Deploy is not used for validation!
func (r *testExampleResource) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

// GetExternalState is not used for validation!
func (r *testExampleResource) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

// Destroy is not used for validation!
func (r *testExampleResource) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}

type testExampleResourceMissingSpecDefinition struct{}

// CanLinkTo is not used for validation!
func (r *testExampleResourceMissingSpecDefinition) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{}, nil
}

// StabilisedDependencies is not used for validation!
func (r *testExampleResourceMissingSpecDefinition) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{}, nil
}

// IsCommonTerminal is not used for validation!
func (r *testExampleResourceMissingSpecDefinition) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: false,
	}, nil
}

func (r *testExampleResourceMissingSpecDefinition) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "celerity/exampleResource",
	}, nil
}

func (r *testExampleResourceMissingSpecDefinition) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		MarkdownDescription:  "",
		PlainTextDescription: "",
	}, nil
}

// StageChanges is not used for validation!
func (r *testExampleResourceMissingSpecDefinition) StageChanges(
	ctx context.Context,
	input *provider.ResourceStageChangesInput,
) (*provider.ResourceStageChangesOutput, error) {
	return &provider.ResourceStageChangesOutput{}, nil
}

func (r *testExampleResourceMissingSpecDefinition) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

func (r *testExampleResourceMissingSpecDefinition) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return &provider.ResourceGetSpecDefinitionOutput{
		SpecDefinition: nil,
	}, nil
}

func (r *testExampleResourceMissingSpecDefinition) GetStateDefinition(
	ctx context.Context,
	input *provider.ResourceGetStateDefinitionInput,
) (*provider.ResourceGetStateDefinitionOutput, error) {
	return &provider.ResourceGetStateDefinitionOutput{
		StateDefinition: nil,
	}, nil
}

// Deploy is not used for validation!
func (r *testExampleResourceMissingSpecDefinition) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

// GetExternalState is not used for validation!
func (r *testExampleResourceMissingSpecDefinition) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

// Destroy is not used for validation!
func (r *testExampleResourceMissingSpecDefinition) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}

type testExampleResourceMissingSpecSchema struct{}

// CanLinkTo is not used for validation!
func (r *testExampleResourceMissingSpecSchema) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{}, nil
}

// StabilisedDependencies is not used for validation!
func (r *testExampleResourceMissingSpecSchema) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{}, nil
}

// IsCommonTerminal is not used for validation!
func (r *testExampleResourceMissingSpecSchema) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: false,
	}, nil
}

func (r *testExampleResourceMissingSpecSchema) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "celerity/exampleResource",
	}, nil
}

func (r *testExampleResourceMissingSpecSchema) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		MarkdownDescription:  "",
		PlainTextDescription: "",
	}, nil
}

// StageChanges is not used for validation!
func (r *testExampleResourceMissingSpecSchema) StageChanges(
	ctx context.Context,
	input *provider.ResourceStageChangesInput,
) (*provider.ResourceStageChangesOutput, error) {
	return &provider.ResourceStageChangesOutput{}, nil
}

func (r *testExampleResourceMissingSpecSchema) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

func (r *testExampleResourceMissingSpecSchema) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return &provider.ResourceGetSpecDefinitionOutput{
		SpecDefinition: &provider.ResourceSpecDefinition{
			Schema: nil,
		},
	}, nil
}

func (r *testExampleResourceMissingSpecSchema) GetStateDefinition(
	ctx context.Context,
	input *provider.ResourceGetStateDefinitionInput,
) (*provider.ResourceGetStateDefinitionOutput, error) {
	return &provider.ResourceGetStateDefinitionOutput{
		StateDefinition: nil,
	}, nil
}

// Deploy is not used for validation!
func (r *testExampleResourceMissingSpecSchema) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

// GetExternalState is not used for validation!
func (r *testExampleResourceMissingSpecSchema) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

// Destroy is not used for validation!
func (r *testExampleResourceMissingSpecSchema) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}

type testExampleResourceMissingStateDefinition struct{}

// CanLinkTo is not used for validation!
func (r *testExampleResourceMissingStateDefinition) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{}, nil
}

// StabilisedDependencies is not used for validation!
func (r *testExampleResourceMissingStateDefinition) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{}, nil
}

// IsCommonTerminal is not used for validation!
func (r *testExampleResourceMissingStateDefinition) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: false,
	}, nil
}

func (r *testExampleResourceMissingStateDefinition) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "celerity/exampleResource",
	}, nil
}

func (r *testExampleResourceMissingStateDefinition) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		MarkdownDescription:  "",
		PlainTextDescription: "",
	}, nil
}

// StageChanges is not used for validation!
func (r *testExampleResourceMissingStateDefinition) StageChanges(
	ctx context.Context,
	input *provider.ResourceStageChangesInput,
) (*provider.ResourceStageChangesOutput, error) {
	return &provider.ResourceStageChangesOutput{}, nil
}

func (r *testExampleResourceMissingStateDefinition) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

func (r *testExampleResourceMissingStateDefinition) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return &provider.ResourceGetSpecDefinitionOutput{
		SpecDefinition: nil,
	}, nil
}

func (r *testExampleResourceMissingStateDefinition) GetStateDefinition(
	ctx context.Context,
	input *provider.ResourceGetStateDefinitionInput,
) (*provider.ResourceGetStateDefinitionOutput, error) {
	return &provider.ResourceGetStateDefinitionOutput{
		StateDefinition: nil,
	}, nil
}

// Deploy is not used for validation!
func (r *testExampleResourceMissingStateDefinition) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

// GetExternalState is not used for validation!
func (r *testExampleResourceMissingStateDefinition) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

// Destroy is not used for validation!
func (r *testExampleResourceMissingStateDefinition) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}

type testEC2InstanceDataSource struct{}

func newTestEC2InstanceDataSource() provider.DataSource {
	return &testEC2InstanceDataSource{}
}

func (d *testEC2InstanceDataSource) GetSpecDefinition(
	ctx context.Context,
	input *provider.DataSourceGetSpecDefinitionInput,
) (*provider.DataSourceGetSpecDefinitionOutput, error) {
	return &provider.DataSourceGetSpecDefinitionOutput{
		SpecDefinition: &provider.DataSourceSpecDefinition{
			Fields: map[string]*provider.DataSourceSpecSchema{
				"serviceName": {
					Type: provider.DataSourceSpecTypeString,
				},
			},
		},
	}, nil
}

func (d *testEC2InstanceDataSource) Fetch(
	ctx context.Context,
	input *provider.DataSourceFetchInput,
) (*provider.DataSourceFetchOutput, error) {
	return &provider.DataSourceFetchOutput{
		Data: map[string]*core.MappingNode{},
	}, nil
}

func (d *testEC2InstanceDataSource) GetType(
	ctx context.Context,
	input *provider.DataSourceGetTypeInput,
) (*provider.DataSourceGetTypeOutput, error) {
	return &provider.DataSourceGetTypeOutput{
		Type: "aws/ec2/instance",
	}, nil
}

func (d *testEC2InstanceDataSource) GetTypeDescription(
	ctx context.Context,
	input *provider.DataSourceGetTypeDescriptionInput,
) (*provider.DataSourceGetTypeDescriptionOutput, error) {
	return &provider.DataSourceGetTypeDescriptionOutput{
		MarkdownDescription:  "",
		PlainTextDescription: "",
	}, nil
}

func (d *testEC2InstanceDataSource) GetFilterFields(
	ctx context.Context,
	input *provider.DataSourceGetFilterFieldsInput,
) (*provider.DataSourceGetFilterFieldsOutput, error) {
	return &provider.DataSourceGetFilterFieldsOutput{
		Fields: []string{"instanceConfigId", "tags"},
	}, nil
}

func (d *testEC2InstanceDataSource) CustomValidate(
	ctx context.Context,
	input *provider.DataSourceValidateInput,
) (*provider.DataSourceValidateOutput, error) {
	return &provider.DataSourceValidateOutput{}, nil
}

type testVPCDataSource struct{}

func newTestVPCDataSource() provider.DataSource {
	return &testVPCDataSource{}
}

func (d *testVPCDataSource) GetSpecDefinition(
	ctx context.Context,
	input *provider.DataSourceGetSpecDefinitionInput,
) (*provider.DataSourceGetSpecDefinitionOutput, error) {
	return &provider.DataSourceGetSpecDefinitionOutput{
		SpecDefinition: &provider.DataSourceSpecDefinition{
			Fields: map[string]*provider.DataSourceSpecSchema{
				"instanceConfigId": {
					Type: provider.DataSourceSpecTypeString,
				},
			},
		},
	}, nil
}

func (d *testVPCDataSource) Fetch(
	ctx context.Context,
	input *provider.DataSourceFetchInput,
) (*provider.DataSourceFetchOutput, error) {
	return &provider.DataSourceFetchOutput{
		Data: map[string]*core.MappingNode{},
	}, nil
}

func (d *testVPCDataSource) GetType(
	ctx context.Context,
	input *provider.DataSourceGetTypeInput,
) (*provider.DataSourceGetTypeOutput, error) {
	return &provider.DataSourceGetTypeOutput{
		Type: "aws/vpc",
	}, nil
}

func (d *testVPCDataSource) GetTypeDescription(
	ctx context.Context,
	input *provider.DataSourceGetTypeDescriptionInput,
) (*provider.DataSourceGetTypeDescriptionOutput, error) {
	return &provider.DataSourceGetTypeDescriptionOutput{
		MarkdownDescription:  "",
		PlainTextDescription: "",
	}, nil
}

func (d *testVPCDataSource) GetFilterFields(
	ctx context.Context,
	input *provider.DataSourceGetFilterFieldsInput,
) (*provider.DataSourceGetFilterFieldsOutput, error) {
	return &provider.DataSourceGetFilterFieldsOutput{
		Fields: []string{"instanceConfigId", "tags"},
	}, nil
}

func (d *testVPCDataSource) CustomValidate(
	ctx context.Context,
	input *provider.DataSourceValidateInput,
) (*provider.DataSourceValidateOutput, error) {
	return &provider.DataSourceValidateOutput{}, nil
}

type testVPC2DataSource struct{}

func newTestVPC2DataSource() provider.DataSource {
	return &testVPC2DataSource{}
}

func (d *testVPC2DataSource) GetSpecDefinition(
	ctx context.Context,
	input *provider.DataSourceGetSpecDefinitionInput,
) (*provider.DataSourceGetSpecDefinitionOutput, error) {
	return &provider.DataSourceGetSpecDefinitionOutput{
		SpecDefinition: nil,
	}, nil
}

func (d *testVPC2DataSource) Fetch(
	ctx context.Context,
	input *provider.DataSourceFetchInput,
) (*provider.DataSourceFetchOutput, error) {
	return &provider.DataSourceFetchOutput{
		Data: map[string]*core.MappingNode{},
	}, nil
}

func (d *testVPC2DataSource) GetType(
	ctx context.Context,
	input *provider.DataSourceGetTypeInput,
) (*provider.DataSourceGetTypeOutput, error) {
	return &provider.DataSourceGetTypeOutput{
		Type: "aws/vpc",
	}, nil
}

func (d *testVPC2DataSource) GetTypeDescription(
	ctx context.Context,
	input *provider.DataSourceGetTypeDescriptionInput,
) (*provider.DataSourceGetTypeDescriptionOutput, error) {
	return &provider.DataSourceGetTypeDescriptionOutput{
		MarkdownDescription:  "",
		PlainTextDescription: "",
	}, nil
}

func (d *testVPC2DataSource) GetFilterFields(
	ctx context.Context,
	input *provider.DataSourceGetFilterFieldsInput,
) (*provider.DataSourceGetFilterFieldsOutput, error) {
	return &provider.DataSourceGetFilterFieldsOutput{
		Fields: []string{"instanceConfigId", "tags"},
	}, nil
}

func (d *testVPC2DataSource) CustomValidate(
	ctx context.Context,
	input *provider.DataSourceValidateInput,
) (*provider.DataSourceValidateOutput, error) {
	return &provider.DataSourceValidateOutput{}, nil
}

type testVPC3DataSource struct{}

func newTestVPC3DataSource() provider.DataSource {
	return &testVPC3DataSource{}
}

func (d *testVPC3DataSource) GetSpecDefinition(
	ctx context.Context,
	input *provider.DataSourceGetSpecDefinitionInput,
) (*provider.DataSourceGetSpecDefinitionOutput, error) {
	return &provider.DataSourceGetSpecDefinitionOutput{
		SpecDefinition: &provider.DataSourceSpecDefinition{
			Fields: map[string]*provider.DataSourceSpecSchema{
				"instanceConfigId": {
					Type: provider.DataSourceSpecTypeString,
				},
			},
		},
	}, nil
}

func (d *testVPC3DataSource) Fetch(
	ctx context.Context,
	input *provider.DataSourceFetchInput,
) (*provider.DataSourceFetchOutput, error) {
	return &provider.DataSourceFetchOutput{
		Data: map[string]*core.MappingNode{},
	}, nil
}

func (d *testVPC3DataSource) GetType(
	ctx context.Context,
	input *provider.DataSourceGetTypeInput,
) (*provider.DataSourceGetTypeOutput, error) {
	return &provider.DataSourceGetTypeOutput{
		Type: "aws/vpc",
	}, nil
}

func (d *testVPC3DataSource) GetTypeDescription(
	ctx context.Context,
	input *provider.DataSourceGetTypeDescriptionInput,
) (*provider.DataSourceGetTypeDescriptionOutput, error) {
	return &provider.DataSourceGetTypeDescriptionOutput{
		MarkdownDescription:  "",
		PlainTextDescription: "",
	}, nil
}

func (d *testVPC3DataSource) GetFilterFields(
	ctx context.Context,
	input *provider.DataSourceGetFilterFieldsInput,
) (*provider.DataSourceGetFilterFieldsOutput, error) {
	return &provider.DataSourceGetFilterFieldsOutput{
		Fields: []string{},
	}, nil
}

func (d *testVPC3DataSource) CustomValidate(
	ctx context.Context,
	input *provider.DataSourceValidateInput,
) (*provider.DataSourceValidateOutput, error) {
	return &provider.DataSourceValidateOutput{}, nil
}

type testECSServiceResource struct{}

func newTestECSServiceResource() provider.Resource {
	return &testECSServiceResource{}
}

// CanLinkTo is not used for validation!
func (r *testECSServiceResource) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return &provider.ResourceCanLinkToOutput{}, nil
}

// StabilisedDependencies is not used for validation!
func (r *testECSServiceResource) StabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return &provider.ResourceStabilisedDependenciesOutput{}, nil
}

// IsCommonTerminal is not used for validation!
func (r *testECSServiceResource) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return &provider.ResourceIsCommonTerminalOutput{
		IsCommonTerminal: false,
	}, nil
}

func (r *testECSServiceResource) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: "celerity/exampleResource",
	}, nil
}

func (r *testECSServiceResource) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return &provider.ResourceGetTypeDescriptionOutput{
		MarkdownDescription:  "",
		PlainTextDescription: "",
	}, nil
}

// StageChanges is not used for validation!
func (r *testECSServiceResource) StageChanges(
	ctx context.Context,
	input *provider.ResourceStageChangesInput,
) (*provider.ResourceStageChangesOutput, error) {
	return &provider.ResourceStageChangesOutput{}, nil
}

func (r *testECSServiceResource) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	return &provider.ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

func (r *testECSServiceResource) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return &provider.ResourceGetSpecDefinitionOutput{
		SpecDefinition: &provider.ResourceSpecDefinition{
			Schema: &provider.ResourceDefinitionsSchema{
				Type: provider.ResourceDefinitionsSchemaTypeObject,
				Attributes: map[string]*provider.ResourceDefinitionsSchema{
					"serviceName": {
						Type: provider.ResourceDefinitionsSchemaTypeString,
					},
				},
			},
		},
	}, nil
}

func (r *testECSServiceResource) GetStateDefinition(
	ctx context.Context,
	input *provider.ResourceGetStateDefinitionInput,
) (*provider.ResourceGetStateDefinitionOutput, error) {
	return &provider.ResourceGetStateDefinitionOutput{
		StateDefinition: &provider.ResourceStateDefinition{
			Schema: &provider.ResourceDefinitionsSchema{
				Type: provider.ResourceDefinitionsSchemaTypeObject,
				Attributes: map[string]*provider.ResourceDefinitionsSchema{
					"id": {
						Type: provider.ResourceDefinitionsSchemaTypeString,
					},
				},
			},
		},
	}, nil
}

// Deploy is not used for validation!
func (r *testECSServiceResource) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return &provider.ResourceDeployOutput{}, nil
}

// GetExternalState is not used for validation!
func (r *testECSServiceResource) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return &provider.ResourceGetExternalStateOutput{}, nil
}

// Destroy is not used for validation!
func (r *testECSServiceResource) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}
