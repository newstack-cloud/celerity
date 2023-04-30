package validation

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	. "gopkg.in/check.v1"
)

type IncludeValidationTestSuite struct{}

var _ = Suite(&IncludeValidationTestSuite{})

func (s *IncludeValidationTestSuite) Test_succeeds_with_no_errors_for_a_valid_child_blueprint_include(c *C) {
	databaseName := "${variables.databaseName}"
	includeSchema := &schema.Include{
		Path: "core-infra.yml",
		Variables: map[string]*core.ScalarValue{
			"databaseName": {
				StringValue: &databaseName,
			},
		},
		Metadata: map[string]interface{}{
			"sourceType": "aws/s3",
			"bucket":     "order-system-blueprints",
			"region":     "eu-west-1",
		},
		Description: "A child blueprint that creates a core infrastructure.",
	}
	variables := map[string]*schema.Variable{
		"databaseName": {
			Type: schema.VariableTypeString,
		},
	}
	err := ValidateInclude(context.Background(), "coreInfra", includeSchema, variables)
	c.Assert(err, IsNil)
}

func (s *IncludeValidationTestSuite) Test_reports_error_for_a_child_blueprint_include_with_an_empty_path(c *C) {
	databaseName := "${variables.databaseName}"
	includeSchema := &schema.Include{
		Path: "",
		Variables: map[string]*core.ScalarValue{
			"databaseName": {
				StringValue: &databaseName,
			},
		},
		Metadata: map[string]interface{}{
			"sourceType": "aws/s3",
			"bucket":     "order-system-blueprints",
			"region":     "eu-west-1",
		},
		Description: "A child blueprint that creates a core infrastructure.",
	}
	variables := map[string]*schema.Variable{
		"databaseName": {
			Type: schema.VariableTypeString,
		},
	}
	err := ValidateInclude(context.Background(), "coreInfra", includeSchema, variables)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := err.(*core.LoadError)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidInclude)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to an empty path being provided for include \"coreInfra\"",
	)
}
