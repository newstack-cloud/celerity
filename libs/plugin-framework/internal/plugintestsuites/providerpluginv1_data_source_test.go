package plugintestsuites

import (
	"context"

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
