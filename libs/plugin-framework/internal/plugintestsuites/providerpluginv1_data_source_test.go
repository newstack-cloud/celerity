package plugintestsuites

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/internal/testprovider"
	"github.com/two-hundred/celerity/libs/plugin-framework/internal/testutils"
)

const (
	vpcDataSourceType = "aws/vpc"
)

func (s *ProviderPluginV1Suite) Test_custom_validate_data_source() {
	dataSource, err := s.provider.DataSource(context.Background(), vpcDataSourceType)
	s.Require().NoError(err)

	output, err := dataSource.CustomValidate(
		context.Background(),
		dataSourceValidateInput(),
	)
	s.Require().NoError(err)
	s.Assert().Equal(
		testprovider.DataSourceVPCValidateOutput(),
		output,
	)
}

func (s *ProviderPluginV1Suite) Test_custom_validate_data_source_fails_for_unexpected_host() {
	dataSource, err := s.providerWrongHost.DataSource(context.Background(), vpcDataSourceType)
	s.Require().NoError(err)

	_, err = dataSource.CustomValidate(
		context.Background(),
		dataSourceValidateInput(),
	)
	testutils.AssertInvalidHost(
		err,
		errorsv1.PluginActionProviderCustomValidateDataSource,
		testWrongHostID,
		&s.Suite,
	)
}

func (s *ProviderPluginV1Suite) Test_custom_validate_data_source_reports_expected_error_for_failure() {
	dataSource, err := s.failingProvider.DataSource(context.Background(), vpcDataSourceType)
	s.Require().NoError(err)

	_, err = dataSource.CustomValidate(
		context.Background(),
		dataSourceValidateInput(),
	)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "internal error occurred applying custom validation for data source")
}

func (s *ProviderPluginV1Suite) Test_data_source_get_type() {
	dataSource, err := s.provider.DataSource(context.Background(), vpcDataSourceType)
	s.Require().NoError(err)

	output, err := dataSource.GetType(
		context.Background(),
		dataSourceGetTypeInput(),
	)
	s.Require().NoError(err)
	s.Assert().Equal(
		&provider.DataSourceGetTypeOutput{
			Type:  vpcDataSourceType,
			Label: "AWS Virtual Private Cloud",
		},
		output,
	)
}

func (s *ProviderPluginV1Suite) Test_data_source_get_type_fails_for_unexpected_host() {
	dataSource, err := s.providerWrongHost.DataSource(context.Background(), vpcDataSourceType)
	s.Require().NoError(err)

	_, err = dataSource.GetType(
		context.Background(),
		dataSourceGetTypeInput(),
	)
	testutils.AssertInvalidHost(
		err,
		errorsv1.PluginActionProviderGetDataSourceType,
		testWrongHostID,
		&s.Suite,
	)
}

func (s *ProviderPluginV1Suite) Test_data_source_get_type_reports_expected_error_for_failure() {
	dataSource, err := s.failingProvider.DataSource(context.Background(), vpcDataSourceType)
	s.Require().NoError(err)

	_, err = dataSource.GetType(
		context.Background(),
		dataSourceGetTypeInput(),
	)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "internal error occurred retrieving data source type information")
}

func (s *ProviderPluginV1Suite) Test_data_source_get_type_description() {
	dataSource, err := s.provider.DataSource(context.Background(), vpcDataSourceType)
	s.Require().NoError(err)

	output, err := dataSource.GetTypeDescription(
		context.Background(),
		dataSourceGetTypeDescriptionInput(),
	)
	s.Require().NoError(err)
	s.Assert().Equal(
		testprovider.DataSourceVPCTypeDescriptionOutput(),
		output,
	)
}

func (s *ProviderPluginV1Suite) Test_data_source_get_type_description_fails_for_unexpected_host() {
	dataSource, err := s.providerWrongHost.DataSource(context.Background(), vpcDataSourceType)
	s.Require().NoError(err)

	_, err = dataSource.GetTypeDescription(
		context.Background(),
		dataSourceGetTypeDescriptionInput(),
	)
	testutils.AssertInvalidHost(
		err,
		errorsv1.PluginActionProviderGetDataSourceTypeDescription,
		testWrongHostID,
		&s.Suite,
	)
}

func (s *ProviderPluginV1Suite) Test_data_source_get_type_description_reports_expected_error_for_failure() {
	dataSource, err := s.failingProvider.DataSource(context.Background(), vpcDataSourceType)
	s.Require().NoError(err)

	_, err = dataSource.GetTypeDescription(
		context.Background(),
		dataSourceGetTypeDescriptionInput(),
	)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "internal error occurred retrieving data source type description")
}
