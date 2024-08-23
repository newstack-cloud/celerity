package corefunctions

import (
	"context"
	"fmt"
	"reflect"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/function"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

type functionCallArgsMock struct {
	args    []any
	callCtx provider.FunctionCallContext
}

func (f *functionCallArgsMock) Get(ctx context.Context, position int) (any, error) {
	return f.args[position], nil
}

func (f *functionCallArgsMock) GetVar(ctx context.Context, position int, target any) error {
	val := reflect.ValueOf(target)
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
			if targetVal.Elem().Kind() != argVal.Kind() {
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
	return nil
}

type functionCallContextMock struct {
	params    *blueprintParamsMock
	registry  *functionRegistryMock
	callStack function.Stack
}

func (f *functionCallContextMock) Registry() provider.FunctionRegistry {
	return f.registry
}

func (f *functionCallContextMock) Params() core.BlueprintParams {
	return f.params
}

func (f *functionCallContextMock) NewCallArgs(args ...any) provider.FunctionCallArguments {
	return &functionCallArgsMock{args: args, callCtx: f}
}

func (f *functionCallContextMock) CallStackSnapshot() []*function.Call {
	// Take a copy of the current call stack.
	return f.callStack.Snapshot()
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

type functionRegistryMock struct {
	functions map[string]provider.Function
	callStack function.Stack
}

func (f *functionRegistryMock) Call(
	ctx context.Context,
	functionName string,
	input *provider.FunctionCallInput,
) (*provider.FunctionCallOutput, error) {
	fnc, ok := f.functions[functionName]
	if !ok {
		return nil, function.NewFuncCallError(
			fmt.Sprintf("function %s not found", functionName),
			function.FuncCallErrorCodeFunctionNotFound,
			input.CallContext.CallStackSnapshot(),
		)
	}
	f.callStack.Push(&function.Call{
		FunctionName: functionName,
		// todo: source location from parsed blueprint.
		Location: nil,
	})
	output, err := fnc.Call(ctx, input)
	f.callStack.Pop()
	return output, err
}
