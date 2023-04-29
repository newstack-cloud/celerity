package validation

import (
	"context"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	. "gopkg.in/check.v1"
)

type ExportValidationTestSuite struct{}

var _ = Suite(&ExportValidationTestSuite{})

func (s *ExportValidationTestSuite) Test_succeeds_with_no_errors_for_a_valid_export(c *C) {
	exportSchema := &schema.Export{
		Type:        schema.ExportTypeObject,
		Description: "The endpoint information to be used to connect to a cache cluster.",
		Field:       "resources.cacheCluster.state.cacheNodes.endpoints",
	}
	err := ValidateExport(context.Background(), "cacheEndpointInfo", exportSchema)
	c.Assert(err, IsNil)
}

func (s *ExportValidationTestSuite) Test_reports_error_when_an_unsupported_export_type_is_provided(c *C) {
	exportSchema := &schema.Export{
		// mapping[string, integer] is not a supported export type.
		Type:        schema.ExportType("mapping[string, integer]"),
		Description: "The endpoint information to be used to connect to a cache cluster.",
		Field:       "resources.cacheCluster.state.cacheNodes.endpoints",
	}
	err := ValidateExport(context.Background(), "cacheEndpointInfo", exportSchema)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := err.(*bpcore.LoadError)
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
	exportSchema := &schema.Export{
		Type:        schema.ExportTypeObject,
		Description: "The endpoint information to be used to connect to a cache cluster.",
		Field:       "",
	}
	err := ValidateExport(context.Background(), "cacheEndpointInfo", exportSchema)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := err.(*bpcore.LoadError)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidExport)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to an empty field string being provided for export \"cacheEndpointInfo\"",
	)
}

func (s *ExportValidationTestSuite) Test_reports_error_when_an_incorrect_reference_is_provided(c *C) {
	exportSchema := &schema.Export{
		Type:        schema.ExportTypeObject,
		Description: "The endpoint information to be used to connect to a cache cluster.",
		// Variable field types are simple key value pairs with primitive or enum values.
		Field: "variables.cacheCluster.state.cacheNodes.endpoints[0].host",
	}
	err := ValidateExport(context.Background(), "cacheEndpointInfo", exportSchema)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := err.(*bpcore.LoadError)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidReference)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to an incorrectly formed reference to a variable "+
			"(\"variables.cacheCluster.state.cacheNodes.endpoints[0].host\") in \"exports.cacheEndpointInfo\". "+
			"See the spec documentation for examples and rules for references",
	)
}
