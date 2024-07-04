package validation

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/errors"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/substitutions"
	. "gopkg.in/check.v1"
)

type ExportValidationTestSuite struct{}

var _ = Suite(&ExportValidationTestSuite{})

func (s *ExportValidationTestSuite) Test_succeeds_with_no_errors_for_a_valid_export(c *C) {
	description := "The endpoint information to be used to connect to a cache cluster."
	exportSchema := &schema.Export{
		Type: schema.ExportTypeObject,
		Description: &substitutions.StringOrSubstitutions{
			Values: []*substitutions.StringOrSubstitution{
				{
					StringValue: &description,
				},
			},
		},
		Field: "resources.cacheCluster.state.cacheNodes.endpoints",
	}
	exportMap := &schema.ExportMap{
		Values: map[string]*schema.Export{
			"cacheEndpointInfo": exportSchema,
		},
	}
	_, err := ValidateExport(context.Background(), "cacheEndpointInfo", exportSchema, exportMap)
	c.Assert(err, IsNil)
}

func (s *ExportValidationTestSuite) Test_reports_error_when_an_unsupported_export_type_is_provided(c *C) {
	description := "The endpoint information to be used to connect to a cache cluster."
	exportSchema := &schema.Export{
		// mapping[string, integer] is not a supported export type.
		Type: schema.ExportType("mapping[string, integer]"),
		Description: &substitutions.StringOrSubstitutions{
			Values: []*substitutions.StringOrSubstitution{
				{
					StringValue: &description,
				},
			},
		},
		Field: "resources.cacheCluster.state.cacheNodes.endpoints",
	}
	exportMap := &schema.ExportMap{
		Values: map[string]*schema.Export{
			"cacheEndpointInfo": exportSchema,
		},
	}
	_, err := ValidateExport(context.Background(), "cacheEndpointInfo", exportSchema, exportMap)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := err.(*errors.LoadError)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidExport)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to an invalid export type of \"mapping[string, integer]\""+
			" being provided for export \"cacheEndpointInfo\". "+
			"The following export types are supported: string, object, integer, float, array, boolean",
	)
}

func (s *ExportValidationTestSuite) Test_reports_error_when_an_empty_export_field_is_provided(c *C) {
	description := "The endpoint information to be used to connect to a cache cluster."
	exportSchema := &schema.Export{
		Type: schema.ExportTypeObject,
		Description: &substitutions.StringOrSubstitutions{
			Values: []*substitutions.StringOrSubstitution{
				{
					StringValue: &description,
				},
			},
		},
		Field: "",
	}
	exportMap := &schema.ExportMap{
		Values: map[string]*schema.Export{
			"cacheEndpointInfo": exportSchema,
		},
	}
	_, err := ValidateExport(context.Background(), "cacheEndpointInfo", exportSchema, exportMap)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := err.(*errors.LoadError)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidExport)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to an empty field string being provided for export \"cacheEndpointInfo\"",
	)
}

func (s *ExportValidationTestSuite) Test_reports_error_when_an_incorrect_reference_is_provided(c *C) {
	description := "The endpoint information to be used to connect to a cache cluster."
	exportSchema := &schema.Export{
		Type: schema.ExportTypeObject,
		Description: &substitutions.StringOrSubstitutions{
			Values: []*substitutions.StringOrSubstitution{
				{
					StringValue: &description,
				},
			},
		},
		// Missing a valid attribute that can be extracted from a resource.
		Field: "resources.cacheCluster.",
	}
	exportMap := &schema.ExportMap{
		Values: map[string]*schema.Export{
			"cacheEndpointInfo": exportSchema,
		},
	}
	_, err := ValidateExport(context.Background(), "cacheEndpointInfo", exportSchema, exportMap)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := err.(*errors.LoadError)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidReference)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to an incorrectly formed reference to a resource "+
			"(\"resources.cacheCluster.\") in \"exports.cacheEndpointInfo\". "+
			"See the spec documentation for examples and rules for references",
	)
}
