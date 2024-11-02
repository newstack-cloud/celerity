package subengine

import (
	"fmt"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/errors"
)

const (
	// ErrorReasonCodeInvalidResolvedSubValue
	// is provided when the reason for an error
	// during deployment or change staging is due to
	// an substitution value that resolves to an invalid type.
	// For example, a substitution value that resolves to a
	// complex object or array when a type that can be cheaply converted
	// to a string is expected.
	ErrorReasonCodeInvalidResolvedSubValue errors.ErrorReasonCode = "invalid_resolved_sub_value"
	// ErrorReasonCodeInvalidSubstitutionValue
	// is provided when the reason for an error
	// during deployment or change staging is due to
	// an empty substitution value.
	ErrorReasonCodeEmptySubstitution errors.ErrorReasonCode = "empty_substitution"
	// ErrorReasonCodeMissingVariable
	// is provided when the reason for an error
	// during deployment or change staging is due to
	// a missing variable referenced in a substitution.
	ErrorReasonCodeMissingVariable errors.ErrorReasonCode = "missing_variable"
	// ErrorReasonCodeMissingValue
	// is provided when the reason for an error
	// during deployment or change staging is due to
	// a missing value referenced in a substitution.
	ErrorReasonCodeMissingValue errors.ErrorReasonCode = "missing_value"
	// ErrorReasonCodeMissingDataSource
	// is provided when the reason for an error
	// during deployment or change staging is due to
	// a missing data source referenced in a substitution.
	ErrorReasonCodeMissingDataSource errors.ErrorReasonCode = "missing_data_source"
	// ErrorReasonCodeEmptyDataSourceData
	// is provided when the reason for an error
	// during deployment or change staging is due to
	// the result of fetching data from a data source being empty.
	ErrorReasonCodeEmptyDataSourceData errors.ErrorReasonCode = "empty_data_source_data"
	// ErrorReasonCodeMissingDataSourceProp
	// is provided when the reason for an error
	// during deployment or change staging is due to
	// a missing property in the data fetched for a
	// data source referenced in a substitution.
	ErrorReasonCodeMissingDataSourceProp errors.ErrorReasonCode = "missing_data_source_prop"
	// ErrorReasonCodeDataSourcePropNotArray
	// is provided when the reason for an error
	// during deployment or change staging is due to
	// a property in the data fetched for a data source
	// referenced in a substitution not being an array.
	ErrorReasonCodeDataSourcePropNotArray errors.ErrorReasonCode = "data_source_prop_not_array"
	// ErrorReasonCodeDataSourcePropArrayIndexOutOfBounds
	// is provided when the reason for an error
	// during deployment or change staging is due to
	// an index being out of bounds for an array property
	// in the data fetched for a data source referenced in a substitution.
	ErrorReasonCodeDataSourcePropArrayIndexOutOfBounds errors.ErrorReasonCode = "data_source_prop_array_index_out_of_bounds"
	// ErrorReasonCodeResourceNotResolved
	// is provided when the reason for an error
	// during deployment or change staging is due to
	// a resource not being resolved before use.
	ErrorReasonCodeResourceNotResolved errors.ErrorReasonCode = "resource_not_resolved"
	// ErrorReasonCodeResourceEachIndexOutOfBounds
	// is provided when the reason for an error
	// during deployment or change staging is due to
	// an index being out of bounds for a resource template
	// that is being used in a resource definition that is made a template
	// by the use of the `each` property.
	ErrorReasonCodeResourceEachIndexOutOfBounds errors.ErrorReasonCode = "resource_each_index_out_of_bounds"
	// ErrorReasonCodeResourceEachEmpty
	// is provided when the reason for an error
	// during deployment or change staging is due to
	// the `each` property of a resource template yielding an empty list.
	ErrorReasonCodeResourceEachEmpty errors.ErrorReasonCode = "resource_each_empty"
	// ErrorReasonCodeResourceEachInvalidType
	// is provided when the reason for an error
	// during deployment or change staging is due to
	// the `each` property of a resource template yielding a value
	// that is not an array.
	ErrorReasonCodeResourceEachInvalidType errors.ErrorReasonCode = "resource_each_invalid_type"
)

func errInvalidInterpolationSubType(elementName string, resolvedValue *core.MappingNode) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeInvalidResolvedSubValue,
		Err: fmt.Errorf("[%s]: expected a string or primitive value that "+
			"can be converted to a string for an interpolation, got %v", elementName, determineValueType(resolvedValue)),
	}
}

func errEmptySubstitutionValue(elementName string) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeEmptySubstitution,
		Err:        fmt.Errorf("[%s]: a string value or substitution value must be provided", elementName),
	}
}

func errMissingVariable(elementName string, variableName string) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeMissingVariable,
		Err:        fmt.Errorf("[%s]: missing variable %q", elementName, variableName),
	}
}

func errMissingValue(elementName string, valueName string) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeMissingValue,
		Err:        fmt.Errorf("[%s]: missing value %q", elementName, valueName),
	}
}

func errMissingDataSource(elementName string, dataSourceName string) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeMissingDataSource,
		Err:        fmt.Errorf("[%s]: missing data source %q", elementName, dataSourceName),
	}
}

func errEmptyDataSourceData(elementName string, dataSourceName string) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeEmptyDataSourceData,
		Err:        fmt.Errorf("[%s]: data source %q returned no data", elementName, dataSourceName),
	}
}

func errMissingDataSourceProperty(elementName string, dataSourceName string, propertyName string) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeMissingDataSourceProp,
		Err:        fmt.Errorf("[%s]: missing property %q in data source %q", elementName, propertyName, dataSourceName),
	}
}

func errDataSourcePropNotArray(elementName string, dataSourceName string, propertyName string) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeDataSourcePropNotArray,
		Err:        fmt.Errorf("[%s]: property %q in data source %q is not an array", elementName, propertyName, dataSourceName),
	}
}

func errDataSourcePropArrayIndexOutOfBounds(elementName string, dataSourceName string, propertyName string, index int) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeDataSourcePropArrayIndexOutOfBounds,
		Err:        fmt.Errorf("[%s]: index %d out of bounds for property %q in data source %q", elementName, index, propertyName, dataSourceName),
	}
}

func errInvalidResourcePropertyPath(elementName string, path string) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeInvalidResolvedSubValue,
		Err:        fmt.Errorf("[%s]: invalid resource property path %q", elementName, path),
	}
}

func errResourceNotResolved(elementName string, resourceName string) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeResourceNotResolved,
		Err:        fmt.Errorf("[%s]: resource %q not resolved before use", elementName, resourceName),
	}
}

func errResourceEachIndexOutOfBounds(elementName string, resourceName string, index int) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeResourceEachIndexOutOfBounds,
		Err:        fmt.Errorf("[%s]: index %d out of bounds for resource template %q", elementName, index, resourceName),
	}
}

func errEmptyResourceEach(elementName string, resourceName string) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeResourceEachEmpty,
		Err: fmt.Errorf(
			"[%s]: resource template %q `each` property yields an empty list, it least one item must be in the list",
			elementName,
			resourceName,
		),
	}
}

func errResourceEachNotArray(elementName string, resourceName string, value *core.MappingNode) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeResourceEachInvalidType,
		Err: fmt.Errorf(
			"[%s]: `each` property in "+
				"resource template %q must yield an array, %s found",
			elementName,
			resourceName,
			determineValueType(value),
		),
	}
}

func determineValueType(resolvedValue *core.MappingNode) string {
	if resolvedValue == nil {
		return "null"
	}

	if resolvedValue.Literal != nil {
		if resolvedValue.Literal.StringValue != nil {
			return "string"
		}

		if resolvedValue.Literal.IntValue != nil {
			return "int"
		}

		if resolvedValue.Literal.FloatValue != nil {
			return "float"
		}

		if resolvedValue.Literal.BoolValue != nil {
			return "bool"
		}
	}

	if resolvedValue.Fields != nil {
		return "object"
	}

	if resolvedValue.Items != nil {
		return "array"
	}

	// StringOrSubstitutions should not be set in a resolved value,
	// in the erroneous case where it is, we return the type as "null".

	return "null"
}
