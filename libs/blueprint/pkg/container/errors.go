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
	errorsLabel := deriveErrorsLabel(childErrCount)
	return fmt.Sprintf("blueprint load error (%d child %s): %s", childErrCount, errorsLabel, e.Err.Error())
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
			"validation failed due to an empty default %s value for variable \"%s\", you must provide a value when declaring a default in a blueprint",
			varType,
			varName,
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

func errVariableValueNotAllowed(
	varType schema.VariableType,
	varName string,
	value *bpcore.ScalarValue,
	allowedValues []*bpcore.ScalarValue,
	usingDefault bool,
) error {
	valueLabel := deriveValueLabel(value, usingDefault)
	return &LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to an invalid %s being provided for variable \"%s\","+
				" only the following values are supported: %s",
			valueLabel,
			varName,
			scalarListToString(allowedValues),
		),
	}
}

func errCustomVariableValueNotInOptions(
	varType schema.VariableType,
	varName string,
	value *bpcore.ScalarValue,
	usingDefault bool,
) error {
	valueLabel := deriveValueLabel(value, usingDefault)
	return &LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to an invalid %s \"%s\" being provided for variable \"%s\","+
				" which is not a valid %s option, see the custom type documentation for more details",
			valueLabel,
			deriveScalarValueAsString(value),
			varName,
			varType,
		),
	}
}

func errRequiredVariableMissing(varName string) error {
	return &LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to a value not being provided for the "+
				"required variable \"%s\", as it does not have a default",
			varName,
		),
	}
}

func errCustomVariableOptions(
	varName string,
	varSchema *schema.Variable,
	err error,
) error {
	return &LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to an error when loading options for variable \"%s\" of custom type \"%s\"",
			varName,
			varSchema.Type,
		),
		ChildErrors: []error{err},
	}
}

func errCustomVariableMixedTypes(
	varName string,
	varSchema *schema.Variable,
) error {
	return &LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to mixed types provided as options for variable type \"%s\" used in variable \"%s\", "+
				"all options must be of the same scalar type",
			varSchema.Type,
			varName,
		),
	}
}

func errCustomVariableInvalidDefaultValueType(varType schema.VariableType, varName string, defaultValue *bpcore.ScalarValue) error {
	defaultVarType := deriveVarType(defaultValue)

	return &LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to an invalid type for a default value for variable \"%s\", %s was provided "+
				"when a custom variable type option of %s was expected",
			varName,
			defaultVarType,
			varType,
		),
	}
}

func errCustomVariableAllowedValuesNotInOptions(varType schema.VariableType, varName string, invalidOptions []string) error {
	return &LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to invalid allowed values being provided for variable \"%s\" "+
				"of custom type \"%s\". See custom type documentation for possible values. Invalid values provided: %s",
			varName,
			varType,
			strings.Join(invalidOptions, ", "),
		),
	}
}

func errCustomVariableDefaultValueNotInOptions(varType schema.VariableType, varName string, defaultValue string) error {
	return &LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to an invalid default value for variable \"%s\" "+
				"of custom type \"%s\". See custom type documentation for possible values. Invalid default value provided: %s",
			varName,
			varType,
			defaultValue,
		),
	}
}

func deriveValueLabel(value *bpcore.ScalarValue, usingDefault bool) string {
	if usingDefault {
		return "default value"
	}

	return "value"
}

func deriveErrorsLabel(errorCount int) string {
	if errorCount == 1 {
		return "error"
	}

	return "errors"
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

func scalarListToString(scalars []*bpcore.ScalarValue) string {
	scalarStrings := make([]string, len(scalars))
	for i, scalar := range scalars {
		scalarStrings[i] = deriveScalarValueAsString(scalar)
	}

	return strings.Join(scalarStrings, ", ")
}
