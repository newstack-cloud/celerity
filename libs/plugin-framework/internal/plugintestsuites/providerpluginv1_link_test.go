package plugintestsuites

import (
	"context"

	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/internal/testprovider"
	"github.com/two-hundred/celerity/libs/plugin-framework/internal/testutils"
)

const (
	testLinkID   = "link-id-1"
	testLinkName = "processOrderFunction_0::ordersTable"
)

func (s *ProviderPluginV1Suite) Test_stage_link_changes() {
	link, err := s.provider.Link(
		context.Background(),
		lambdaFunctionResourceType,
		dynamoDBTableResourceType,
	)
	s.Require().NoError(err)

	output, err := link.StageChanges(
		context.Background(),
		linkStageChangesInput(),
	)
	s.Require().NoError(err)
	expected := testprovider.LinkLambdaDynamoDBChangesOutput()
	testutils.AssertLinkChangesEquals(
		expected.Changes,
		output.Changes,
		&s.Suite,
	)
}

func (s *ProviderPluginV1Suite) Test_stage_link_changes_fails_for_unexpected_host() {
	link, err := s.providerWrongHost.Link(
		context.Background(),
		lambdaFunctionResourceType,
		dynamoDBTableResourceType,
	)
	s.Require().NoError(err)

	_, err = link.StageChanges(
		context.Background(),
		linkStageChangesInput(),
	)
	testutils.AssertInvalidHost(
		err,
		errorsv1.PluginActionProviderStageLinkChanges,
		testWrongHostID,
		&s.Suite,
	)
}

func (s *ProviderPluginV1Suite) Test_stage_link_changes_reports_expected_error_for_failure() {
	link, err := s.failingProvider.Link(
		context.Background(),
		lambdaFunctionResourceType,
		dynamoDBTableResourceType,
	)
	s.Require().NoError(err)

	_, err = link.StageChanges(
		context.Background(),
		linkStageChangesInput(),
	)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "internal error occurred when staging changes for link")
}
