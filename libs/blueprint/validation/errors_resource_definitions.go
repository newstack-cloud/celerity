package validation

import (
	"fmt"

	"github.com/two-hundred/celerity/libs/blueprint/errors"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/source"
)

func errResourceDefItemEmpty(
	path string,
	resourceSpecType provider.ResourceDefinitionsSchemaType,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to an empty resource item "+
				"at path %q where the %s type was expected",
			path,
			resourceSpecType,
		),
		Line:   line,
		Column: col,
	}
}

func errResourceDefInvalidType(
	path string,
	foundType provider.ResourceDefinitionsSchemaType,
	expectedType provider.ResourceDefinitionsSchemaType,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to an invalid resource item "+
				"at path %q where the %s type was expected, but %s was found",
			path,
			expectedType,
			foundType,
		),
		Line:   line,
		Column: col,
	}
}

func errResourceDefMissingRequiredField(
	path string,
	field string,
	fieldType provider.ResourceDefinitionsSchemaType,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to a missing required field %q of type %s "+
				"at path %q",
			field,
			fieldType,
			path,
		),
		Line:   line,
		Column: col,
	}
}

func errResourceDefUnknownField(
	path string,
	field string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to an unknown field %q "+
				"at path %q, only fields that match the resource definition schema are allowed",
			field,
			path,
		),
		Line:   line,
		Column: col,
	}
}

func errInvalidResourceDefSubType(
	resolvedType string,
	path string,
	expectedResolvedType string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to an invalid resource item "+
				"at path %q where a value of type %s was expected, but type %s was found",
			path,
			expectedResolvedType,
			resolvedType,
		),
		Line:   line,
		Column: col,
	}
}

func errResourceDefUnionItemEmpty(
	path string,
	unionSchema []*provider.ResourceDefinitionsSchema,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	unionType := resourceDefinitionsUnionTypeToString(unionSchema)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to an empty resource item "+
				"at path %s where one of the types %s was expected",
			path,
			unionType,
		),
		Line:   line,
		Column: col,
	}
}

func errResourceDefUnionInvalidType(
	path string,
	unionSchema []*provider.ResourceDefinitionsSchema,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	unionType := resourceDefinitionsUnionTypeToString(unionSchema)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to an invalid resource item found "+
				"at path %q where one of the types %s was expected",
			path,
			unionType,
		),
		Line:   line,
		Column: col,
	}
}
