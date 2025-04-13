package plugintestsuites

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	"github.com/two-hundred/celerity/libs/blueprint/transform"
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/internal/testtransformer"
	"github.com/two-hundred/celerity/libs/plugin-framework/internal/testutils"
)

const (
	celerityHandlerAbstractResourceType = "celerity/handler"
)

func (s *TransformerPluginV1Suite) Test_custom_validate_abstract_resource() {
	resource, err := s.transformer.AbstractResource(
		context.Background(),
		celerityHandlerAbstractResourceType,
	)
	s.Require().NoError(err)

	output, err := resource.CustomValidate(
		context.Background(),
		abstractResourceValidateInput(),
	)
	s.Require().NoError(err)
	s.Assert().Equal(
		testtransformer.AbstractResourceHandlerValidateOutput(),
		output,
	)
}

func (s *TransformerPluginV1Suite) Test_custom_validate_abstract_resource_fails_for_unexpected_host() {
	abstractResource, err := s.transformerWrongHost.AbstractResource(
		context.Background(),
		celerityHandlerAbstractResourceType,
	)
	s.Require().NoError(err)

	_, err = abstractResource.CustomValidate(
		context.Background(),
		abstractResourceValidateInput(),
	)
	testutils.AssertInvalidHost(
		err,
		errorsv1.PluginActionTransformerCustomValidateAbstractResource,
		testWrongHostID,
		&s.Suite,
	)
}

func (s *TransformerPluginV1Suite) Test_custom_validate_resource_reports_expected_error_for_failure() {
	abstractResource, err := s.failingTransformer.AbstractResource(
		context.Background(),
		celerityHandlerAbstractResourceType,
	)
	s.Require().NoError(err)

	_, err = abstractResource.CustomValidate(
		context.Background(),
		abstractResourceValidateInput(),
	)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "internal error occurred validating abstract resource")
}

func abstractResourceValidateInput() *transform.AbstractResourceValidateInput {
	return &transform.AbstractResourceValidateInput{
		SchemaResource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{
				Value: celerityHandlerAbstractResourceType,
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
					"handlerName": core.MappingNodeFromString("my-handler"),
				},
			},
		},
		TransformerContext: testutils.CreateTestTransformerContext("celerity"),
	}
}
