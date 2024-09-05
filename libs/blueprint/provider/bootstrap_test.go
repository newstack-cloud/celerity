package provider

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/function"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) {
	TestingT(t)
}

type testProvider struct {
	functions   map[string]Function
	resources   map[string]Resource
	dataSources map[string]DataSource
	namespace   string
}

func (p *testProvider) Namespace(ctx context.Context) (string, error) {
	return p.namespace, nil
}

func (p *testProvider) Resource(ctx context.Context, resourceType string) (Resource, error) {
	resource, ok := p.resources[resourceType]
	if !ok {
		return nil, errors.New("resource not found")
	}
	return resource, nil
}

func (p *testProvider) DataSource(ctx context.Context, dataSourceType string) (DataSource, error) {
	dataSource, ok := p.dataSources[dataSourceType]
	if !ok {
		return nil, errors.New("data source not found")
	}
	return dataSource, nil
}

func (p *testProvider) Link(ctx context.Context, resourceTypeA string, resourceTypeB string) (Link, error) {
	return nil, nil
}

func (p *testProvider) CustomVariableType(ctx context.Context, customVariableType string) (CustomVariableType, error) {
	return nil, nil
}

func (p *testProvider) ListFunctions(ctx context.Context) ([]string, error) {
	functionNames := []string{}
	for name := range p.functions {
		functionNames = append(functionNames, name)
	}
	return functionNames, nil
}

func (p *testProvider) Function(ctx context.Context, functionName string) (Function, error) {
	function, ok := p.functions[functionName]
	if !ok {
		return nil, errors.New("function not found")
	}
	return function, nil
}

type testSubstrFunction struct {
	definition *function.Definition
}

func newTestSubstrFunction() Function {
	return &testSubstrFunction{
		definition: &function.Definition{
			Description: "Extracts a substring from the given string.",
			FormattedDescription: "Extracts a substring from the given string.\n\n" +
				"**Examples:**\n\n" +
				"```\n${substr(values.cacheClusterConfig.host, 0, 3)}\n```",
			Parameters: []function.Parameter{
				&function.ScalarParameter{
					Type: &function.ValueTypeDefinitionScalar{
						Label: "string",
						Type:  function.ValueTypeString,
					},
					Description: "A valid string literal, reference or function call yielding a return value " +
						"representing the string to extract the substring from.",
				},
				&function.ScalarParameter{
					Type: &function.ValueTypeDefinitionScalar{
						Label: "integer",
						Type:  function.ValueTypeInt64,
					},
					Description: "The index of the first character to include in the substring.",
				},
				&function.ScalarParameter{
					Type: &function.ValueTypeDefinitionScalar{
						Label: "integer",
						Type:  function.ValueTypeInt64,
					},
					Optional: true,
					Description: "The index of the last character to include in the substring. " +
						"If not provided, the substring will include all characters from the start index to the end of the string.",
				},
			},
			Return: &function.ScalarReturn{
				Type: &function.ValueTypeDefinitionScalar{
					Label: "string",
					Type:  function.ValueTypeString,
				},
				Description: "The substring extracted from the provided string.",
			},
		},
	}
}

func (f *testSubstrFunction) GetDefinition(
	ctx context.Context,
	input *FunctionGetDefinitionInput,
) (*FunctionGetDefinitionOutput, error) {
	return &FunctionGetDefinitionOutput{
		Definition: f.definition,
	}, nil
}

func (f *testSubstrFunction) Call(
	ctx context.Context,
	input *FunctionCallInput,
) (*FunctionCallOutput, error) {
	var inputStr string
	var start int64
	if err := input.Arguments.GetMultipleVars(ctx, &inputStr, &start); err != nil {
		return nil, err
	}

	var end int64
	if err := input.Arguments.GetVar(ctx, 2, &end); err != nil {
		end = int64(len(inputStr))
	}

	if start > end {
		return nil, function.NewFuncCallError(
			"start index cannot be greater than end index",
			function.FuncCallErrorCodeInvalidInput,
			input.CallContext.CallStackSnapshot(),
		)
	}

	if start < 0 || end < 0 {
		return nil, function.NewFuncCallError(
			"start and end indices cannot be negative",
			function.FuncCallErrorCodeInvalidInput,
			input.CallContext.CallStackSnapshot(),
		)
	}

	if start > int64(len(inputStr)-1) {
		return nil, function.NewFuncCallError(
			"start index cannot be greater than the last element index in the string",
			function.FuncCallErrorCodeInvalidInput,
			input.CallContext.CallStackSnapshot(),
		)
	}

	if end > int64(len(inputStr)) {
		return nil, function.NewFuncCallError(
			"end index cannot be greater than the length of the string",
			function.FuncCallErrorCodeInvalidInput,
			input.CallContext.CallStackSnapshot(),
		)
	}

	return &FunctionCallOutput{
		ResponseData: inputStr[start:end],
	}, nil
}

type functionCallArgsMock struct {
	args    []any
	callCtx FunctionCallContext
}

func (f *functionCallArgsMock) Get(ctx context.Context, position int) (any, error) {
	return f.args[position], nil
}

func (f *functionCallArgsMock) GetVar(ctx context.Context, position int, target any) error {
	val := reflect.ValueOf(target)
	if position >= len(f.args) {
		return function.NewFuncCallError(
			fmt.Sprintf("argument at index %d not found", position),
			function.FuncCallErrorCodeFunctionCall,
			f.callCtx.CallStackSnapshot(),
		)
	}

	targetVal := reflect.ValueOf(target)
	if targetVal.Kind() != reflect.Ptr {
		return function.NewFuncCallError(
			"target to read argument into is not a pointer",
			function.FuncCallErrorCodeInvalidArgumentType,
			f.callCtx.CallStackSnapshot(),
		)
	}

	argVal := reflect.ValueOf(f.args[position])
	// Allow interface{} as a target type so that the caller can carry out type assertions
	// when an argument can be of multiple types.
	if targetVal.Elem().Kind() != reflect.Interface && targetVal.Elem().Kind() != argVal.Kind() {
		return function.NewFuncCallError(
			fmt.Sprintf(
				"argument at index %d is of type %s, but target is of type %s",
				position,
				argVal.Kind(),
				targetVal.Elem().Kind(),
			),
			function.FuncCallErrorCodeInvalidArgumentType,
			f.callCtx.CallStackSnapshot(),
		)
	}

	val.Elem().Set(reflect.ValueOf(f.args[position]))
	return nil
}

func (f *functionCallArgsMock) GetMultipleVars(ctx context.Context, targets ...any) error {
	for i := 0; i < len(f.args); i += 1 {
		if i < len(targets) {
			targetVal := reflect.ValueOf(targets[i])
			if targetVal.Kind() != reflect.Ptr {
				return function.NewFuncCallError(
					fmt.Sprintf("target at index %d to read argument into is not a pointer", i),
					function.FuncCallErrorCodeInvalidArgumentType,
					f.callCtx.CallStackSnapshot(),
				)
			}

			argVal := reflect.ValueOf(f.args[i])
			// Allow interface{} as a target type so that the caller can carry out type assertions
			// when an argument can be of multiple types.
			if targetVal.Elem().Kind() != reflect.Interface && targetVal.Elem().Kind() != argVal.Kind() {
				return function.NewFuncCallError(
					fmt.Sprintf(
						"argument at index %d is of type %s, but target is of type %s",
						i,
						argVal.Kind(),
						targetVal.Elem().Kind(),
					),
					function.FuncCallErrorCodeInvalidArgumentType,
					f.callCtx.CallStackSnapshot(),
				)
			}
			targetVal.Elem().Set(argVal)
		}
	}

	if len(targets) > len(f.args) {
		expectedText := fmt.Sprintf("%d arguments expected", len(targets))
		if len(targets) == 1 {
			expectedText = "1 argument expected"
		}
		argsText := fmt.Sprintf(", but %d arguments were passed into function", len(f.args))
		if len(f.args) == 1 {
			argsText = ", but 1 argument was passed into function"
		}

		return function.NewFuncCallError(
			fmt.Sprintf(
				"%s%s",
				expectedText,
				argsText,
			),
			function.FuncCallErrorCodeFunctionCall,
			f.callCtx.CallStackSnapshot(),
		)
	}
	return nil
}

type functionCallContextMock struct {
	params          *blueprintParamsMock
	registry        FunctionRegistry
	callStack       function.Stack
	currentLocation *source.Meta
}

func (f *functionCallContextMock) Registry() FunctionRegistry {
	return f.registry
}

func (f *functionCallContextMock) Params() core.BlueprintParams {
	return f.params
}

func (f *functionCallContextMock) NewCallArgs(args ...any) FunctionCallArguments {
	return &functionCallArgsMock{args: args, callCtx: f}
}

func (f *functionCallContextMock) CallStackSnapshot() []*function.Call {
	// Take a copy of the current call stack.
	return f.callStack.Snapshot()
}

func (f *functionCallContextMock) CurrentLocation() *source.Meta {
	return f.currentLocation
}

func (f *functionCallContextMock) SetCurrentLocation(location *source.Meta) {
	f.currentLocation = location
}

type blueprintParamsMock struct {
	providerConfig     map[string]*core.ScalarValue
	contextVariables   map[string]*core.ScalarValue
	blueprintVariables map[string]*core.ScalarValue
}

func (b *blueprintParamsMock) ProviderConfig(namespace string) map[string]*core.ScalarValue {
	return b.providerConfig
}

func (b *blueprintParamsMock) ContextVariable(name string) *core.ScalarValue {
	return b.contextVariables[name]
}

func (b *blueprintParamsMock) BlueprintVariable(name string) *core.ScalarValue {
	return b.blueprintVariables[name]
}

type testExampleResource struct {
	definition *ResourceSpecDefinition
}

func newTestExampleResource() Resource {
	return &testExampleResource{
		definition: &ResourceSpecDefinition{
			Schema: &ResourceSpecSchema{
				Type: ResourceSpecTypeObject,
				Attributes: map[string]*ResourceSpecSchema{
					"name": {
						Type: ResourceSpecTypeString,
					},
					"ids": {
						Type: ResourceSpecTypeArray,
						Items: &ResourceSpecSchema{
							Type: ResourceSpecTypeObject,
							Attributes: map[string]*ResourceSpecSchema{
								"name": {
									Type: ResourceSpecTypeString,
								},
							},
						},
					},
				},
			},
		},
	}
}

// CanLinkTo is not used for validation!
func (r *testExampleResource) CanLinkTo(
	ctx context.Context,
	input *ResourceCanLinkToInput,
) (*ResourceCanLinkToOutput, error) {
	return &ResourceCanLinkToOutput{}, nil
}

// IsCommonTerminal is not used for validation!
func (r *testExampleResource) IsCommonTerminal(
	ctx context.Context,
	input *ResourceIsCommonTerminalInput,
) (*ResourceIsCommonTerminalOutput, error) {
	return &ResourceIsCommonTerminalOutput{
		IsCommonTerminal: false,
	}, nil
}

func (r *testExampleResource) GetType(
	ctx context.Context,
	input *ResourceGetTypeInput,
) (*ResourceGetTypeOutput, error) {
	return &ResourceGetTypeOutput{
		Type: "celerity/exampleResource",
	}, nil
}

// StageChanges is not used for validation!
func (r *testExampleResource) StageChanges(
	ctx context.Context,
	input *ResourceStageChangesInput,
) (*ResourceStageChangesOutput, error) {
	return &ResourceStageChangesOutput{}, nil
}

func (r *testExampleResource) CustomValidate(
	ctx context.Context,
	input *ResourceValidateInput,
) (*ResourceValidateOutput, error) {
	return &ResourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}

func (r *testExampleResource) GetSpecDefinition(
	ctx context.Context,
	input *ResourceGetSpecDefinitionInput,
) (*ResourceGetSpecDefinitionOutput, error) {
	return &ResourceGetSpecDefinitionOutput{
		SpecDefinition: r.definition,
	}, nil
}

// Deploy is not used for validation!
func (r *testExampleResource) Deploy(
	ctx context.Context,
	input *ResourceDeployInput,
) (*ResourceDeployOutput, error) {
	return &ResourceDeployOutput{}, nil
}

// GetExternalState is not used for validation!
func (r *testExampleResource) GetExternalState(
	ctx context.Context,
	input *ResourceGetExternalStateInput,
) (*ResourceGetExternalStateOutput, error) {
	return &ResourceGetExternalStateOutput{}, nil
}

// Destroy is not used for validation!
func (r *testExampleResource) Destroy(
	ctx context.Context,
	input *ResourceDestroyInput,
) error {
	return nil
}

type testExampleDataSource struct {
	definition *DataSourceSpecDefinition
}

func newTestExampleDataSource() DataSource {
	return &testExampleDataSource{
		definition: &DataSourceSpecDefinition{
			Fields: map[string]*DataSourceSpecSchema{
				"name": {
					Type: DataSourceSpecTypeString,
				},
			},
		},
	}
}

func (d *testExampleDataSource) GetSpecDefinition(
	ctx context.Context,
	input *DataSourceGetSpecDefinitionInput,
) (*DataSourceGetSpecDefinitionOutput, error) {
	return &DataSourceGetSpecDefinitionOutput{
		SpecDefinition: d.definition,
	}, nil
}

func (d *testExampleDataSource) Fetch(
	ctx context.Context,
	input *DataSourceFetchInput,
) (*DataSourceFetchOutput, error) {
	return &DataSourceFetchOutput{
		Data: map[string]interface{}{},
	}, nil
}

func (d *testExampleDataSource) GetType(
	ctx context.Context,
	input *DataSourceGetTypeInput,
) (*DataSourceGetTypeOutput, error) {
	return &DataSourceGetTypeOutput{
		Type: "test/exampleDataSource",
	}, nil
}

func (d *testExampleDataSource) CustomValidate(
	ctx context.Context,
	input *DataSourceValidateInput,
) (*DataSourceValidateOutput, error) {
	return &DataSourceValidateOutput{
		Diagnostics: []*core.Diagnostic{},
	}, nil
}
