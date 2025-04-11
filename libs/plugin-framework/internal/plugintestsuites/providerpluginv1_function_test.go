package plugintestsuites

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/internal/testprovider"
	"github.com/two-hundred/celerity/libs/plugin-framework/internal/testutils"
)

const (
	functionName = "trim_suffix"
)

func (s *ProviderPluginV1Suite) Test_function_get_definition() {
	function, err := s.provider.Function(
		context.Background(),
		functionName,
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
		functionName,
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
		functionName,
	)
	s.Require().NoError(err)

	_, err = function.GetDefinition(
		context.Background(),
		functionGetDefinitionInput(),
	)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "internal error occurred retrieving function definition")
}
