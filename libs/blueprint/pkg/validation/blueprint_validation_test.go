package validation

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/errors"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	. "gopkg.in/check.v1"
)

type BlueprintValidationTestSuite struct{}

var _ = Suite(&BlueprintValidationTestSuite{})

func (s *BlueprintValidationTestSuite) Test_succeeds_without_any_issues_for_a_valid_blueprint(c *C) {

	instanceType := "t2.micro"
	blueprint := &schema.Blueprint{
		Version: Version2023_04_20,
		Resources: map[string]*schema.Resource{
			"resource1": {
				Type: "aws/ec2/instance",
				Spec: &core.MappingNode{
					Fields: map[string]*core.MappingNode{
						"instanceType": &core.MappingNode{
							Literal: &core.ScalarValue{
								StringValue: &instanceType,
							},
						},
					},
				},
			},
		},
	}
	err := ValidateBlueprint(context.Background(), blueprint)
	c.Assert(err, IsNil)
}

func (s *BlueprintValidationTestSuite) Test_reports_errors_when_the_version_is_not_set(c *C) {
	instanceType := "t2.micro"
	blueprint := &schema.Blueprint{
		Resources: map[string]*schema.Resource{
			"resource1": {
				Type: "aws/ec2/instance",
				Spec: &core.MappingNode{
					Fields: map[string]*core.MappingNode{
						"instanceType": &core.MappingNode{
							Literal: &core.ScalarValue{
								StringValue: &instanceType,
							},
						},
					},
				},
			},
		},
	}

	err := ValidateBlueprint(context.Background(), blueprint)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := err.(*errors.LoadError)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeMissingVersion)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to a version not being provided, version is a required property",
	)
}

func (s *BlueprintValidationTestSuite) Test_reports_errors_when_the_version_is_incorrect(c *C) {
	// In the intial version of blueprint framework, only version
	// 2023-04-20 of the spec is supported.
	instanceType := "t2.micro"
	blueprint := &schema.Blueprint{
		Version: "2023-09-15",
		Resources: map[string]*schema.Resource{
			"resource1": {
				Type: "aws/ec2/instance",
				Spec: &core.MappingNode{
					Fields: map[string]*core.MappingNode{
						"instanceType": &core.MappingNode{
							Literal: &core.ScalarValue{
								StringValue: &instanceType,
							},
						},
					},
				},
			},
		},
	}
	err := ValidateBlueprint(context.Background(), blueprint)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := err.(*errors.LoadError)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidVersion)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to an unsupported version \"2023-09-15\" being provided. "+
			"supported versions include: 2023-04-20",
	)
}

func (s *BlueprintValidationTestSuite) Test_reports_errors_when_the_resources_property_is_missing(c *C) {
	blueprint := &schema.Blueprint{
		Version: Version2023_04_20,
	}
	err := ValidateBlueprint(context.Background(), blueprint)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := err.(*errors.LoadError)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeMissingResources)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to an empty set of resources, at least one resource must be defined in a blueprint",
	)
}

func (s *BlueprintValidationTestSuite) Test_reports_errors_when_no_resources_are_provided(c *C) {
	blueprint := &schema.Blueprint{
		Version:   Version2023_04_20,
		Resources: map[string]*schema.Resource{},
	}
	err := ValidateBlueprint(context.Background(), blueprint)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := err.(*errors.LoadError)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeMissingResources)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to an empty set of resources, at least one resource must be defined in a blueprint",
	)
}
