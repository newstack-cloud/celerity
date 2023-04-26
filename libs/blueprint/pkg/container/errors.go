package container

import (
	"fmt"
	"strings"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	"github.com/two-hundred/celerity/libs/common/pkg/core"
)

type LoadError struct {
	ReasonCode  ErrorReasonCode
	Err         error
	ChildErrors []error
}

func (e *LoadError) Error() string {
	childErrCount := len(e.ChildErrors)
	if childErrCount == 0 {
		return fmt.Sprintf("blueprint load error: %s", e.Err.Error())
	}
	return fmt.Sprintf("blueprint load error (%d child errors): %s", childErrCount, e.Err.Error())
}

type ErrorReasonCode string

const (
	// ErrorReasonCodeInvalidSpecExtension is provided
	// when the reason for a blueprint spec load error
	// is due to an invalid specification file extension.
	ErrorReasonCodeInvalidSpecExtension ErrorReasonCode = "invalid_spec_ext"
	// ErrorReasonCodeInvalidResourceType is provided
	// when the reason for a blueprint spec load error
	// is due to an invalid resource type provided in one
	// of the resources in the spec.
	ErrorReasonCodeInvalidResourceType ErrorReasonCode = "invalid_resource_type"
	// ErrorReasonCodeMissingProvider is provided when the
	// reason for a blueprint spec load error is due to
	// a missing provider for one of the resources in
	// the spec.
	ErrorReasonCodeMissingProvider ErrorReasonCode = "missing_provider"
	// ErrorReasonCodeMissingResource is provided when the
	// reason for a blueprint spec load error is due to
	// the resource provider missing an implementation for the
	// resource type for one of the resources in the spec.
	ErrorReasonCodeMissingResource ErrorReasonCode = "missing_resource"
	// ErrorReasonCodeResourceValidationErrors is provided
	// when the reason for a blueprint spec load error is due to
	// a collection of errors for one or more resources in the spec.
	// This should be used for a wrapper error that holds more specific
	// errors which can be used for reporting useful information
	// about issues with the spec.
	ErrorReasonCodeResourceValidationErrors ErrorReasonCode = "resource_validation_errors"
	// ErrorReasonMissingTransformers is provided when the
	// reason for a blueprint spec load error is due to a spec referencing
	// transformers that aren't supported by the blueprint loader
	// used to parse the schema.
	ErrorReasonMissingTransformers ErrorReasonCode = "missing_transformers"
	// ErrorReasonCodeInvalidVariable is provided when the reason
	// for a blueprint spec load error is due to one or more variables
	// being invalid.
	// This could be due to a mismatch between the type and the value,
	// a missing required variable (one without a default value),
	// an invalid default value, invalid allowed values or an incorrect variable type.
	ErrorReasonCodeInvalidVariable ErrorReasonCode = "invalid_variable"
)

func errUnsupportedSpecFileExtension(filePath string) error {
	return &LoadError{
		ReasonCode: ErrorReasonCodeInvalidSpecExtension,
		Err:        fmt.Errorf("unsupported spec file extension in %s, only json and yaml are supported", filePath),
	}
}

func errInvalidResourceType(resourceType string) error {
	return &LoadError{
		ReasonCode: ErrorReasonCodeInvalidResourceType,
		Err:        fmt.Errorf("resource type format is invalid for %s, resource type must be of the form {provider}/*", resourceType),
	}
}

func errMissingProvider(providerKey string, resourceType string) error {
	return &LoadError{
		ReasonCode: ErrorReasonCodeMissingProvider,
		Err:        fmt.Errorf("missing provider %s for the resource type %s", providerKey, resourceType),
	}
}

func errMissingResource(providerKey string, resourceType string) error {
	return &LoadError{
		ReasonCode: ErrorReasonCodeMissingResource,
		Err:        fmt.Errorf("missing resource in provider %s for the resource type %s", providerKey, resourceType),
	}
}

func errResourceValidationError(errorMap map[string]error) error {
	errCount := len(errorMap)
	return &LoadError{
		ReasonCode:  ErrorReasonCodeResourceValidationErrors,
		Err:         fmt.Errorf("validation failed due to issues with %d resources in the spec", errCount),
		ChildErrors: core.MapToSlice(errorMap),
	}
}

func errTransformersMissingError(missingTransformers []string) error {
	return &LoadError{
		ReasonCode: ErrorReasonMissingTransformers,
		Err: fmt.Errorf(
			"the following transformers are missing in the blueprint loader: %s", strings.Join(missingTransformers, ", "),
		),
	}
}

func errVariableInvalidDefaultValue(varType schema.VariableType, varName string, defaultValue *bpcore.ScalarValue) error {
	defaultVarType := deriveVarType(defaultValue)

	return &LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to an invalid default value for variable \"%s\", %s was provided when %s was expected",
			varName,
			defaultVarType,
			varType,
		),
	}
}

func errVariableEmptyDefaultValue(varType schema.VariableType, varName string) error {
	return &LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to an empty default %s value, you must provide a value when declaring a default in a blueprint",
			varType,
		),
	}
}

func errVariableInvalidOrMissing(
	varType schema.VariableType,
	varName string,
	value *bpcore.ScalarValue,
	varSchema *schema.Variable,
) error {
	actualVarType := deriveOptionalVarType(value)
	if actualVarType == nil {
		return &LoadError{
			ReasonCode: ErrorReasonCodeInvalidVariable,
			Err: fmt.Errorf(
				"validation failed to a missing value for variable \"%s\", a value of type %s must be provided",
				varName,
				varType,
			),
		}
	}

	return &LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to an incorrect type used for variable \"%s\", "+
				"expected a value of type %s but one of type %s was provided",
			varName,
			varType,
			*actualVarType,
		),
	}
}

func errVariableEmptyValue(
	varType schema.VariableType,
	varName string,
) error {
	return &LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to an empty value being provided for variable \"%s\", "+
				"please provide a valid %s value that is not empty",
			varName,
			varType,
		),
	}
}

func errVariableInvalidAllowedValue(
	varType schema.VariableType,
	allowedValue *bpcore.ScalarValue,
) error {
	allowedValueVarType := deriveVarType(allowedValue)
	scalarValueStr := deriveScalarValueAsString(allowedValue)

	return fmt.Errorf(
		"an invalid allowed value was provided, %s with the value \"%s\" was provided when only %ss are allowed",
		varTypeToUnit(allowedValueVarType),
		scalarValueStr,
		varType,
	)
}

func errVariableInvalidAllowedValues(
	varName string,
	allowedValueErrors []error,
) error {
	return &LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to one or more invalid allowed values being provided for variable \"%s\"",
			varName,
		),
		ChildErrors: allowedValueErrors,
	}
}

func errVariableInvalidAllowedValuesNotSupported(
	varType schema.VariableType,
	varName string,
) error {
	return &LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to an allowed values list being provided for %s variable \"%s\","+
				" %s variables do not support allowed values enumeration",
			varType,
			varName,
			varType,
		),
	}
}

func deriveOptionalVarType(value *bpcore.ScalarValue) *schema.VariableType {
	if value.IntValue != nil {
		intVarType := schema.VariableTypeInteger
		return &intVarType
	}

	if value.FloatValue != nil {
		floatVarType := schema.VariableTypeFloat
		return &floatVarType
	}

	if value.BoolValue != nil {
		boolVarType := schema.VariableTypeBoolean
		return &boolVarType
	}

	if value.StringValue != nil {
		stringVarType := schema.VariableTypeString
		return &stringVarType
	}

	return nil
}

func deriveVarType(value *bpcore.ScalarValue) schema.VariableType {
	if value.IntValue != nil {
		return schema.VariableTypeInteger
	}

	if value.FloatValue != nil {
		return schema.VariableTypeFloat
	}

	if value.BoolValue != nil {
		return schema.VariableTypeBoolean
	}

	// This should only ever be used in a context where
	// the given scalar has a value, so string will always
	// be the default.
	return schema.VariableTypeString
}

func deriveScalarValueAsString(value *bpcore.ScalarValue) string {
	if value.IntValue != nil {
		return fmt.Sprintf("%d", *value.IntValue)
	}

	if value.FloatValue != nil {
		return fmt.Sprintf("%.2f", *value.FloatValue)
	}

	if value.BoolValue != nil {
		return fmt.Sprintf("%t", *value.BoolValue)
	}

	if value.StringValue != nil {
		return *value.StringValue
	}

	return ""
}

func varTypeToUnit(varType schema.VariableType) string {
	switch varType {
	case schema.VariableTypeInteger:
		return "an integer"
	case schema.VariableTypeFloat:
		return "a float"
	case schema.VariableTypeBoolean:
		return "a boolean"
	case schema.VariableTypeString:
		return "a string"
	default:
		return "an unknown type"
	}
}
