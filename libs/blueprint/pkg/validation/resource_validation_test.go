package validation

import (
	"github.com/two-hundred/celerity/libs/blueprint/pkg/errors"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/source"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/substitutions"
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
