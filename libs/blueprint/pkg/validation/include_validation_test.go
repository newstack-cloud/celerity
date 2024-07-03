package validation

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/errors"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/substitutions"
	. "gopkg.in/check.v1"
)

type IncludeValidationTestSuite struct{}

var _ = Suite(&IncludeValidationTestSuite{})

func (s *IncludeValidationTestSuite) Test_succeeds_with_no_errors_for_a_valid_child_blueprint_include(c *C) {
	databaseName := "${variables.databaseName}"
	path := "core-infra.yml"
	sourceType := "aws/s3"
	bucket := "order-system-blueprints"
	region := "eu-west-1"
	description := "A child blueprint that creates a core infrastructure."
	includeSchema := &schema.Include{
		Path: &substitutions.StringOrSubstitutions{
			Values: []*substitutions.StringOrSubstitution{
				{
					StringValue: &path,
				},
			},
		},
		Variables: &core.MappingNode{
			Fields: map[string]*core.MappingNode{
				"databaseName": {
					StringWithSubstitutions: &substitutions.StringOrSubstitutions{
						Values: []*substitutions.StringOrSubstitution{
							{
								StringValue: &databaseName,
							},
						},
					},
				},
			},
		},
		Metadata: &core.MappingNode{
			Fields: map[string]*core.MappingNode{
				"sourceType": {
					StringWithSubstitutions: &substitutions.StringOrSubstitutions{
						Values: []*substitutions.StringOrSubstitution{
							{
								StringValue: &sourceType,
							},
						},
					},
				},
				"bucket": {
					StringWithSubstitutions: &substitutions.StringOrSubstitutions{
						Values: []*substitutions.StringOrSubstitution{
							{
								StringValue: &bucket,
							},
						},
					},
				},
				"region": {
					StringWithSubstitutions: &substitutions.StringOrSubstitutions{
						Values: []*substitutions.StringOrSubstitution{
							{
								StringValue: &region,
							},
						},
					},
				},
			},
		},
		Description: &substitutions.StringOrSubstitutions{
			Values: []*substitutions.StringOrSubstitution{
				{
					StringValue: &description,
				},
			},
		},
	}
	includeMap := &schema.IncludeMap{
		Values: map[string]*schema.Include{
			"coreInfra": includeSchema,
		},
	}
	err := ValidateInclude(context.Background(), "coreInfra", includeSchema, includeMap)
	c.Assert(err, IsNil)
}

func (s *IncludeValidationTestSuite) Test_reports_error_for_a_child_blueprint_include_with_an_empty_path(c *C) {
	databaseName := "${variables.databaseName}"
	path := ""
	sourceType := "aws/s3"
	bucket := "order-system-blueprints"
	region := "eu-west-1"
	description := "A child blueprint that creates a core infrastructure."
	includeSchema := &schema.Include{
		Path: &substitutions.StringOrSubstitutions{
			Values: []*substitutions.StringOrSubstitution{
				{
					StringValue: &path,
				},
			},
		},
		Variables: &core.MappingNode{
			Fields: map[string]*core.MappingNode{
				"databaseName": {
					StringWithSubstitutions: &substitutions.StringOrSubstitutions{
						Values: []*substitutions.StringOrSubstitution{
							{
								StringValue: &databaseName,
							},
						},
					},
				},
			},
		},
		Metadata: &core.MappingNode{
			Fields: map[string]*core.MappingNode{
				"sourceType": {
					StringWithSubstitutions: &substitutions.StringOrSubstitutions{
						Values: []*substitutions.StringOrSubstitution{
							{
								StringValue: &sourceType,
							},
						},
					},
				},
				"bucket": {
					StringWithSubstitutions: &substitutions.StringOrSubstitutions{
						Values: []*substitutions.StringOrSubstitution{
							{
								StringValue: &bucket,
							},
						},
					},
				},
				"region": {
					StringWithSubstitutions: &substitutions.StringOrSubstitutions{
						Values: []*substitutions.StringOrSubstitution{
							{
								StringValue: &region,
							},
						},
					},
				},
			},
		},
		Description: &substitutions.StringOrSubstitutions{
			Values: []*substitutions.StringOrSubstitution{
				{
					StringValue: &description,
				},
			},
		},
	}
	includeMap := &schema.IncludeMap{
		Values: map[string]*schema.Include{
			"coreInfra": includeSchema,
		},
	}
	err := ValidateInclude(context.Background(), "coreInfra", includeSchema, includeMap)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := err.(*errors.LoadError)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidInclude)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to an empty path being provided for include \"coreInfra\"",
	)
}
