package schema

import (
	"fmt"
)

// Error represents an error due to an issue
// with the schema of a blueprint.
type Error struct {
	ReasonCode ErrorSchemaReasonCode
	Err        error
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
	// ErrorSchemaReasonCodeInvalidTransformType is provided
	// when the reason for a blueprint schema load error is due to
	// an invalid transform field value being provided.
	ErrorSchemaReasonCodeInvalidTransformType ErrorSchemaReasonCode = "invalid_transform_type"
)

func errInvalidDataSourceFieldType(dataSourceFieldType DataSourceFieldType) error {
	return &Error{
		ReasonCode: ErrorSchemaReasonCodeInvalidDataSourceFieldType,
		Err: fmt.Errorf(
			"unsupported data source field type %s has been provided, you can choose from string, integer, float, boolean, object and array",
			dataSourceFieldType,
		),
	}
}

func errInvalidTransformType(underlyingError error) error {
	return &Error{
		ReasonCode: ErrorSchemaReasonCodeInvalidTransformType,
		Err: fmt.Errorf(
			"unsupported type provided for spec transform, must be string or a list of strings: %s",
			underlyingError.Error(),
		),
	}
}
