package schema

import (
	"fmt"
	"strings"

	"github.com/two-hundred/celerity/libs/common/pkg/core"
)

// Error represents an error due to an issue
// with the schema of a blueprint.
type Error struct {
	ReasonCode ErrorSchemaReasonCode
	Err        error
	// The line in the source blueprint file
	// where the error occurred.
	// This will be nil if the error is not related
	// to a specific line in the blueprint file
	// or the source format is JSON.
	SourceLine *int
	// The column on a line in the source blueprint file
	// where the error occurred.
	// This will be nil if the error is not related
	// to a specific line/column in the blueprint file
	// or the source format is JSON.
	SourceColumn *int
}

func (e *Error) Error() string {
	return e.Err.Error()
}

type ErrorSchemaReasonCode string

const (
	// ErrorSchemaReasonCodeInvalidVariableType is provided
	// when the reason for a blueprint schema load error is due
	// to an invalid variable type.
	ErrorSchemaReasonCodeInvalidVariableType ErrorSchemaReasonCode = "invalid_variable_type"
	// ErrorSchemaReasonCodeInvalidDataSourceFieldType is provided
	// when the reason for a blueprint schema load error is due
	// to an invalid data source exported field type.
	ErrorSchemaReasonCodeInvalidDataSourceFieldType ErrorSchemaReasonCode = "invalid_data_source_field_type"
	// ErrorSchemaReasonCodeInvalidDataSourceFilterOperator is provided
	// when the reason for a blueprint schema load error is due
	// to an invalid data source filter operator being provided.
	ErrorSchemaReasonCodeInvalidDataSourceFilterOperator ErrorSchemaReasonCode = "invalid_data_source_filter_operator"
	// ErrorSchemaReasonCodeInvalidTransformType is provided
	// when the reason for a blueprint schema load error is due to
	// an invalid transform field value being provided.
	ErrorSchemaReasonCodeInvalidTransformType ErrorSchemaReasonCode = "invalid_transform_type"
)

func errInvalidDataSourceFieldType(
	dataSourceFieldType DataSourceFieldType,
	line *int,
	column *int,
) error {
	return &Error{
		ReasonCode: ErrorSchemaReasonCodeInvalidDataSourceFieldType,
		Err: fmt.Errorf(
			"unsupported data source field type %s has been provided, you can choose from string, integer, float, boolean, object and array",
			dataSourceFieldType,
		),
		SourceLine:   line,
		SourceColumn: column,
	}
}

func errInvalidDataSourceFilterOperator(dataSourceFilterOperator DataSourceFilterOperator) error {
	return &Error{
		ReasonCode: ErrorSchemaReasonCodeInvalidDataSourceFilterOperator,
		Err: fmt.Errorf(
			"unsupported data source filter operator %s has been provided, you can choose from %s",
			dataSourceFilterOperator,
			strings.Join(
				core.Map(DataSourceFilterOperators, func(operator DataSourceFilterOperator, index int) string {
					return string(operator)
				}),
				",",
			),
		),
	}
}

func errInvalidTransformType(underlyingError error, line *int, column *int) error {
	return &Error{
		ReasonCode: ErrorSchemaReasonCodeInvalidTransformType,
		Err: fmt.Errorf(
			"unsupported type provided for spec transform, must be string or a list of strings: %s",
			underlyingError.Error(),
		),
		SourceLine:   line,
		SourceColumn: column,
	}
}
