package plugintestsuites

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/function"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/internal/testprovider"
	"github.com/two-hundred/celerity/libs/plugin-framework/internal/testutils"
)

const (
	trimSuffixFunctionName = "trim_suffix"
)

func (s *ProviderPluginV1Suite) Test_function_get_definition() {
	function, err := s.provider.Function(
		context.Background(),
		trimSuffixFunctionName,
	)
	s.Require().NoError(err)

	output, err := function.GetDefinition(
		context.Background(),
		functionGetDefinitionInput(),
	)
	s.Require().NoError(err)
	s.Assert().Equal(
		&provider.FunctionGetDefinitionOutput{
			Definition: testprovider.TrimSuffixFunctionDefinition(),
		},
		output,
	)
}

func (s *ProviderPluginV1Suite) Test_function_get_defintion_fails_for_unexpected_host() {
	function, err := s.providerWrongHost.Function(
		context.Background(),
		trimSuffixFunctionName,
	)
	s.Require().NoError(err)

	_, err = function.GetDefinition(
		context.Background(),
		functionGetDefinitionInput(),
	)
	testutils.AssertInvalidHost(
		err,
		errorsv1.PluginActionProviderGetFunctionDefinition,
		testWrongHostID,
		&s.Suite,
	)
}

func (s *ProviderPluginV1Suite) Test_function_get_definition_reports_expected_error_for_failure() {
	function, err := s.failingProvider.Function(
		context.Background(),
		trimSuffixFunctionName,
	)
	s.Require().NoError(err)

	_, err = function.GetDefinition(
		context.Background(),
		functionGetDefinitionInput(),
	)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "internal error occurred retrieving function definition")
}

func (s *ProviderPluginV1Suite) Test_function_call() {
	fn, err := s.provider.Function(
		context.Background(),
		trimSuffixFunctionName,
	)
	s.Require().NoError(err)

	callStack := function.NewStack()
	registryForCall := s.funcRegistry.ForCallContext(callStack)
	callContext := &functionCallContextMock{
		registry:  registryForCall,
		callStack: callStack,
		params:    emptyConcreteParams(),
	}
	output, err := fn.Call(
		context.Background(),
		&provider.FunctionCallInput{
			Arguments: callContext.NewCallArgs(
				"Hello, universe!",
				", universe!",
			),
			CallContext: callContext,
		},
	)
	s.Require().NoError(err)
	s.Assert().Equal(
		&provider.FunctionCallOutput{
			ResponseData: "Hello",
		},
		output,
	)
}

func (s *ProviderPluginV1Suite) Test_function_call_fails_for_unexpected_host() {
	fn, err := s.providerWrongHost.Function(
		context.Background(),
		trimSuffixFunctionName,
	)
	s.Require().NoError(err)

	callStack := function.NewStack()
	registryForCall := s.funcRegistry.ForCallContext(callStack)
	callContext := &functionCallContextMock{
		registry:  registryForCall,
		callStack: callStack,
		params:    emptyConcreteParams(),
	}
	_, err = fn.Call(
		context.Background(),
		&provider.FunctionCallInput{
			Arguments: callContext.NewCallArgs(
				"Hello there!",
				" there!",
			),
			CallContext: callContext,
		},
	)
	testutils.AssertInvalidHost(
		err,
		errorsv1.PluginActionProviderCallFunction,
		testWrongHostID,
		&s.Suite,
	)
}

func (s *ProviderPluginV1Suite) Test_function_call_reports_expected_error_for_failure() {
	fn, err := s.failingProvider.Function(
		context.Background(),
		trimSuffixFunctionName,
	)
	s.Require().NoError(err)

	callStack := function.NewStack()
	registryForCall := s.funcRegistry.ForCallContext(callStack)
	callContext := &functionCallContextMock{
		registry:  registryForCall,
		callStack: callStack,
		params:    emptyConcreteParams(),
	}
	_, err = fn.Call(
		context.Background(),
		&provider.FunctionCallInput{
			Arguments: callContext.NewCallArgs(
				"Input with suffix!",
				" with suffix!",
			),
			CallContext: callContext,
		},
	)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "internal error occurred calling function")
}

func emptyConcreteParams() *core.ParamsImpl {
	return &core.ParamsImpl{
		ProviderConf:       map[string]map[string]*core.ScalarValue{},
		TransformerConf:    map[string]map[string]*core.ScalarValue{},
		ContextVariables:   map[string]*core.ScalarValue{},
		BlueprintVariables: map[string]*core.ScalarValue{},
	}
}
