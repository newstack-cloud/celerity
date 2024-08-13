package validation

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/errors"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	. "gopkg.in/check.v1"
)

type ResourceValidationTestSuite struct{}

var _ = Suite(&ResourceValidationTestSuite{})

func (s *ResourceValidationTestSuite) Test_reports_error_when_substitution_provided_in_resource_name(c *C) {
	description := "EC2 instance for the application"
	resourceSchema := &schema.Resource{
		Type: "${variables.awsEC2InstanceName}",
		Description: &substitutions.StringOrSubstitutions{
			Values: []*substitutions.StringOrSubstitution{
				{
					StringValue: &description,
				},
			},
		},
	}
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"${variables.awsEC2InstanceName}": resourceSchema,
		},
		SourceMeta: map[string]*source.Meta{
			"${variables.awsEC2InstanceName}": {
				Line:   1,
				Column: 1,
			},
		},
	}
	err := ValidateResourceName("${variables.awsEC2InstanceName}", resourceMap)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := err.(*errors.LoadError)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidResource)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: ${..} substitutions can not be used in resource names, "+
			"found in resource \"${variables.awsEC2InstanceName}\"",
	)
}

func (s *ResourceValidationTestSuite) Test_reports_errors_when_substitutions_used_in_spec_mapping_keys(c *C) {
	version := "1.0.0"
	resourceSchema := &schema.Resource{
		Type: "celerity/api",
		Spec: &core.MappingNode{
			Items: []*core.MappingNode{
				{
					Fields: map[string]*core.MappingNode{
						"${variables.version}": {
							Literal: &core.ScalarValue{
								StringValue: &version,
							},
						},
					},
					SourceMeta: &source.Meta{
						Line:   1,
						Column: 1,
					},
					FieldsSourceMeta: map[string]*source.Meta{
						"${variables.version}": {
							Line:   1,
							Column: 1,
						},
					},
				},
			},
		},
	}
	resourceMap := &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"api": resourceSchema,
		},
		SourceMeta: map[string]*source.Meta{
			"api": {
				Line:   1,
				Column: 1,
			},
		},
	}
	err := PreValidateResourceSpec(context.TODO(), "api", resourceSchema, resourceMap)
	c.Assert(err, NotNil)
}
