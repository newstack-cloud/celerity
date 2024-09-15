package validation

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	. "gopkg.in/check.v1"
)

type TransformValidationTestSuite struct{}

var _ = Suite(&TransformValidationTestSuite{})

func (s *TransformValidationTestSuite) Test_succeeds_without_any_issues_for_a_valid_transform(c *C) {
	version := Version2023_04_20
	blueprint := &schema.Blueprint{
		Version: &core.ScalarValue{StringValue: &version},
		Transform: &schema.TransformValueWrapper{
			Values: []string{TransformCelerity2024_09_01},
			SourceMeta: []*source.Meta{
				{Position: source.Position{
					Line:   1,
					Column: 1,
				}},
			},
		},
	}
	diagnostics, err := ValidateTransforms(context.Background(), blueprint, false)
	c.Assert(err, IsNil)
	c.Assert(diagnostics, HasLen, 0)
}

func (s *BlueprintValidationTestSuite) Test_reports_errors_and_warnings_for_invalid_and_non_core_transforms(c *C) {
	version := Version2023_04_20
	blueprint := &schema.Blueprint{
		Version: &core.ScalarValue{StringValue: &version},
		Transform: &schema.TransformValueWrapper{
			Values: []string{TransformCelerity2024_09_01, "", "non-core-transform"},
			SourceMeta: []*source.Meta{
				{Position: source.Position{
					Line:   1,
					Column: 1,
				}},
				{Position: source.Position{
					Line:   2,
					Column: 1,
				}},
				{Position: source.Position{
					Line:   3,
					Column: 1,
				}},
			},
		},
	}
	diagnostics, err := ValidateTransforms(context.Background(), blueprint, false)
	c.Assert(err, IsNil)
	c.Assert(diagnostics, DeepEquals, []*core.Diagnostic{
		{
			Level:   core.DiagnosticLevelError,
			Message: "A transform can not be empty.",
			Range: &core.DiagnosticRange{
				Start: &source.Meta{Position: source.Position{
					Line:   2,
					Column: 1,
				}},
				End: &source.Meta{Position: source.Position{
					Line:   3,
					Column: 1,
				}},
			},
		},
		{
			Level: core.DiagnosticLevelWarning,
			Message: "The transform \"non-core-transform\" is not a core transform," +
				" you will need to make sure it is configured when deploying this blueprint.",
			Range: &core.DiagnosticRange{
				Start: &source.Meta{Position: source.Position{
					Line:   3,
					Column: 1,
				}},
				End: &source.Meta{Position: source.Position{
					Line:   4,
					Column: 1,
				}},
			},
		},
	})
}

func (s *BlueprintValidationTestSuite) Test_reports_error_for_sub_usage_in_transform(c *C) {
	version := Version2023_04_20
	blueprint := &schema.Blueprint{
		Version: &core.ScalarValue{StringValue: &version},
		Transform: &schema.TransformValueWrapper{
			Values: []string{TransformCelerity2024_09_01, "${variables.transform1}"},
			SourceMeta: []*source.Meta{
				{Position: source.Position{
					Line:   1,
					Column: 1,
				}},
				{Position: source.Position{
					Line:   2,
					Column: 1,
				}},
			},
		},
	}
	diagnostics, err := ValidateTransforms(context.Background(), blueprint, false)
	c.Assert(err, IsNil)
	c.Assert(diagnostics, DeepEquals, []*core.Diagnostic{
		{
			Level:   core.DiagnosticLevelError,
			Message: "${..} substitutions can not be used in a transform.",
			Range: &core.DiagnosticRange{
				Start: &source.Meta{Position: source.Position{
					Line:   2,
					Column: 1,
				}},
				End: &source.Meta{Position: source.Position{
					Line:   3,
					Column: 1,
				}},
			},
		},
	})
}
