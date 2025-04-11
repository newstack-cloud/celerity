package plugintestsuites

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/internal/testprovider"
	"github.com/two-hundred/celerity/libs/plugin-framework/internal/testutils"
)

const (
	instanceTypeCustomVarType = "aws/ec2/instanceType"
)

func (s *ProviderPluginV1Suite) Test_custom_variable_type_get_type() {
	customVarType, err := s.provider.CustomVariableType(
		context.Background(),
		instanceTypeCustomVarType,
	)
	s.Require().NoError(err)

	output, err := customVarType.GetType(
		context.Background(),
		customVarTypeGetTypeInput(),
	)
	s.Require().NoError(err)
	s.Assert().Equal(
		&provider.CustomVariableTypeGetTypeOutput{
			Type:  instanceTypeCustomVarType,
			Label: "AWS EC2 Instance Type",
		},
		output,
	)
}

func (s *ProviderPluginV1Suite) Test_custom_variable_type_get_type_fails_for_unexpected_host() {
	customVarType, err := s.providerWrongHost.CustomVariableType(
		context.Background(),
		instanceTypeCustomVarType,
	)
	s.Require().NoError(err)

	_, err = customVarType.GetType(
		context.Background(),
		customVarTypeGetTypeInput(),
	)
	testutils.AssertInvalidHost(
		err,
		errorsv1.PluginActionProviderGetCustomVariableType,
		testWrongHostID,
		&s.Suite,
	)
}

func (s *ProviderPluginV1Suite) Test_custom_variable_type_get_type_reports_expected_error_for_failure() {
	customVarType, err := s.failingProvider.CustomVariableType(
		context.Background(),
		instanceTypeCustomVarType,
	)
	s.Require().NoError(err)

	_, err = customVarType.GetType(
		context.Background(),
		customVarTypeGetTypeInput(),
	)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "internal error occurred retrieving custom variable type information")
}

func (s *ProviderPluginV1Suite) Test_custom_variable_type_get_type_description() {
	customVarType, err := s.provider.CustomVariableType(
		context.Background(),
		instanceTypeCustomVarType,
	)
	s.Require().NoError(err)

	output, err := customVarType.GetDescription(
		context.Background(),
		customVarTypeGetDescriptionInput(),
	)
	s.Require().NoError(err)
	s.Assert().Equal(
		testprovider.CustomVarTypeInstanceTypeDescriptionOutput(),
		output,
	)
}

func (s *ProviderPluginV1Suite) Test_custom_variable_type_get_description_fails_for_unexpected_host() {
	customVarType, err := s.providerWrongHost.CustomVariableType(
		context.Background(),
		instanceTypeCustomVarType,
	)
	s.Require().NoError(err)

	_, err = customVarType.GetDescription(
		context.Background(),
		customVarTypeGetDescriptionInput(),
	)
	testutils.AssertInvalidHost(
		err,
		errorsv1.PluginActionProviderGetCustomVariableTypeDescription,
		testWrongHostID,
		&s.Suite,
	)
}

func (s *ProviderPluginV1Suite) Test_custom_variable_type_get_type_description_reports_expected_error_for_failure() {
	customVarType, err := s.failingProvider.CustomVariableType(
		context.Background(),
		instanceTypeCustomVarType,
	)
	s.Require().NoError(err)

	_, err = customVarType.GetDescription(
		context.Background(),
		customVarTypeGetDescriptionInput(),
	)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "internal error occurred retrieving custom variable type description")
}
