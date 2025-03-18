package plugintestsuites

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/internal/testprovider"
	"github.com/two-hundred/celerity/libs/plugin-framework/internal/testutils"
)

const (
	lambdaFunctionResourceType = "aws/lambda/function"
)

func (s *ProviderPluginV1Suite) Test_custom_validate_resource() {
	resource, err := s.provider.Resource(context.Background(), lambdaFunctionResourceType)
	s.Require().NoError(err)

	output, err := resource.CustomValidate(
		context.Background(),
		resourceValidateInput(),
	)
	s.Require().NoError(err)
	s.Assert().Equal(
		testprovider.ResourceLambdaFunctionValidateOutput(),
		output,
	)
}

func (s *ProviderPluginV1Suite) Test_custom_validate_resource_fails_for_unexpected_host() {
	resource, err := s.providerWrongHost.Resource(context.Background(), lambdaFunctionResourceType)
	s.Require().NoError(err)

	_, err = resource.CustomValidate(
		context.Background(),
		resourceValidateInput(),
	)
	testutils.AssertInvalidHost(
		err,
		errorsv1.PluginActionProviderCustomValidateResource,
		testWrongHostID,
		&s.Suite,
	)
}

func (s *ProviderPluginV1Suite) Test_custom_validate_resource_reports_expected_error_for_failure() {
	resource, err := s.failingProvider.Resource(context.Background(), lambdaFunctionResourceType)
	s.Require().NoError(err)

	_, err = resource.CustomValidate(
		context.Background(),
		resourceValidateInput(),
	)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "internal error occurred applying custom validation for resource")
}

func resourceValidateInput() *provider.ResourceValidateInput {
	return &provider.ResourceValidateInput{
		SchemaResource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{
				Value: "aws/lambda/function",
			},
			Metadata: &schema.Metadata{
				Annotations: &schema.StringOrSubstitutionsMap{
					Values: map[string]*substitutions.StringOrSubstitutions{},
				},
				Labels: &schema.StringMap{
					Values: map[string]string{},
				},
			},
			Spec: &core.MappingNode{
				Fields: map[string]*core.MappingNode{
					"functionName": core.MappingNodeFromString("my-function"),
				},
			},
		},
		ProviderContext: testutils.CreateTestProviderContext("aws"),
	}
}
