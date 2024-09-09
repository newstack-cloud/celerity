package validation

import (
	"fmt"

	"github.com/two-hundred/celerity/libs/blueprint/errors"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/source"
)

func errResourceSpecItemEmpty(
	path string,
	resourceSpecType provider.ResourceSpecSchemaType,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to an empty resource spec item "+
				"at path %q where the %s type was expected",
			path,
			resourceSpecType,
		),
		Line:   line,
		Column: col,
	}
}

func errResourceSpecInvalidType(
	path string,
	foundType provider.ResourceSpecSchemaType,
	expectedType provider.ResourceSpecSchemaType,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to an invalid resource spec item "+
				"at path %q where the %s type was expected, but %s was found",
			path,
			expectedType,
			foundType,
		),
		Line:   line,
		Column: col,
	}
}

func errResourceSpecMissingRequiredField(
	path string,
	field string,
	fieldType provider.ResourceSpecSchemaType,
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

func errInvalidResourceSpecSubType(
	resolvedType string,
	path string,
	expectedResolvedType string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to an invalid resource spec item "+
				"at path %q where a value of type %s was expected, but type %s was found",
			path,
			expectedResolvedType,
			resolvedType,
		),
		Line:   line,
		Column: col,
	}
}

func errResourceSpecUnionItemEmpty(
	path string,
	unionSchema []*provider.ResourceSpecSchema,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	unionType := resourceSpecUnionTypeToString(unionSchema)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to an empty resource spec item "+
				"at path %s where one of the types %s was expected",
			path,
			unionType,
		),
		Line:   line,
		Column: col,
	}
}

func errResourceSpecUnionInvalidType(
	path string,
	unionSchema []*provider.ResourceSpecSchema,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	unionType := resourceSpecUnionTypeToString(unionSchema)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to an invalid resource spec item found "+
				"at path %q where one of the types %s was expected",
			path,
			unionType,
		),
		Line:   line,
		Column: col,
	}
}
