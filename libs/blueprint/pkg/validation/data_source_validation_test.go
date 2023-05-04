package validation

// This does not cover ensuring the filter.operator field is valid,
// as that validation is carried out while parsing the schema of a blueprint.

import (
	"context"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	. "gopkg.in/check.v1"
)

type DataSourceValidationTestSuite struct{}

var _ = Suite(&DataSourceValidationTestSuite{})

func (s *DataSourceValidationTestSuite) Test_succeeds_without_any_issues_for_a_valid_data_source(c *C) {
	search := "Production"

	dataSource := &schema.DataSource{
		Type: "aws/vpc",
		Filter: &schema.DataSourceFilter{
			Field: "tags",
			Operator: &schema.DataSourceFilterOperatorWrapper{
				Value: schema.DataSourceFilterOperatorHasKey,
			},
			Search: &schema.DataSourceFilterSearch{
				Values: []*bpcore.ScalarValue{
					{
						StringValue: &search,
					},
				},
			},
		},
		Exports: map[string]*schema.DataSourceFieldExport{
			"instanceId": {
				Type: &schema.DataSourceFieldTypeWrapper{
					Value: schema.DataSourceFieldTypeString,
				},
				AliasFor: "instanceConfig.id",
			},
		},
	}
	err := ValidateDataSource(context.Background(), "vpc", dataSource)
	c.Assert(err, IsNil)
}

func (s *DataSourceValidationTestSuite) Test_reports_errors_when_filter_is_missing(c *C) {
	dataSource := &schema.DataSource{
		Type: "aws/ec2/instance",
		// Filter omitted.
		Exports: map[string]*schema.DataSourceFieldExport{
			"instanceId": {
				Type: &schema.DataSourceFieldTypeWrapper{
					Value: schema.DataSourceFieldTypeString,
				},
				AliasFor: "instanceConfig.id",
			},
		},
	}

	err := ValidateDataSource(context.Background(), "vmInstance", dataSource)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := err.(*bpcore.LoadError)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidDataSource)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to a missing filter in "+
			"data source \"vmInstance\", every data source must have a filter",
	)
}

func (s *DataSourceValidationTestSuite) Test_reports_errors_when_field_is_empty(c *C) {
	name1 := "Processor-Dev"
	name2 := "Processor-Prod"

	dataSource := &schema.DataSource{
		Type: "aws/ec2/instance",
		Filter: &schema.DataSourceFilter{
			// Field omitted.
			Operator: &schema.DataSourceFilterOperatorWrapper{
				Value: schema.DataSourceFilterOperatorIn,
			},
			Search: &schema.DataSourceFilterSearch{
				Values: []*bpcore.ScalarValue{
					{
						StringValue: &name1,
					}, {
						StringValue: &name2,
					},
				},
			},
		},
		Exports: map[string]*schema.DataSourceFieldExport{
			"instanceId": {
				Type: &schema.DataSourceFieldTypeWrapper{
					Value: schema.DataSourceFieldTypeString,
				},
				AliasFor: "instanceConfig.id",
			},
		},
	}

	err := ValidateDataSource(context.Background(), "vmInstance", dataSource)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := err.(*bpcore.LoadError)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidDataSource)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to a missing field in filter for "+
			"data source \"vmInstance\", field must be set for a data source filter",
	)
}

func (s *DataSourceValidationTestSuite) Test_reports_errors_when_filter_search_is_empty(c *C) {
	dataSource := &schema.DataSource{
		Type: "aws/ec2/instance",
		Filter: &schema.DataSourceFilter{
			Field: "instanceId",
			Operator: &schema.DataSourceFilterOperatorWrapper{
				Value: schema.DataSourceFilterOperatorIn,
			},
			// Search omitted.
		},
		Exports: map[string]*schema.DataSourceFieldExport{
			"instanceId": {
				Type: &schema.DataSourceFieldTypeWrapper{
					Value: schema.DataSourceFieldTypeString,
				},
				AliasFor: "instanceConfig.id",
			},
		},
	}

	err := ValidateDataSource(context.Background(), "vmInstance", dataSource)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := err.(*bpcore.LoadError)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidDataSource)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to a missing search in filter for "+
			"data source \"vmInstance\", at least one search value must be provided for a filter",
	)
}

func (s *DataSourceValidationTestSuite) Test_reports_errors_when_no_exported_fields_are_provided(c *C) {
	search := "Production"

	dataSource := &schema.DataSource{
		Type: "aws/ec2/instance",
		Filter: &schema.DataSourceFilter{
			Field: "instanceId",
			Operator: &schema.DataSourceFilterOperatorWrapper{
				Value: schema.DataSourceFilterOperatorIn,
			},
			Search: &schema.DataSourceFilterSearch{
				Values: []*bpcore.ScalarValue{
					{
						StringValue: &search,
					},
				},
			},
		},
		Exports: map[string]*schema.DataSourceFieldExport{
			// No exports provided for the data source.
		},
	}

	err := ValidateDataSource(context.Background(), "vmInstance", dataSource)
	c.Assert(err, NotNil)
	loadErr, isLoadErr := err.(*bpcore.LoadError)
	c.Assert(isLoadErr, Equals, true)
	c.Assert(loadErr.ReasonCode, Equals, ErrorReasonCodeInvalidDataSource)
	c.Assert(
		loadErr.Error(),
		Equals,
		"blueprint load error: validation failed due to missing exports for "+
			"data source \"vmInstance\", at least one field must be exported for a data source",
	)
}
