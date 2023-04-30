package validation

import (
	"fmt"
	"strings"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	"github.com/two-hundred/celerity/libs/common/pkg/core"
)

const (
	// ErrorReasonCodeInvalidVariable is provided when the reason
	// for a blueprint spec load error is due to one or more variables
	// being invalid.
	// This could be due to a mismatch between the type and the value,
	// a missing required variable (one without a default value),
	// an invalid default value, invalid allowed values or an incorrect variable type.
	ErrorReasonCodeInvalidVariable bpcore.ErrorReasonCode = "invalid_variable"
	// ErrorReasonCodeInvalidExport is provided when the reason
	// for a blueprint spec load error is due to one or more exports
	// being invalid.
	ErrorReasonCodeInvalidExport bpcore.ErrorReasonCode = "invalid_export"
	// ErrorReasonCodeInvalidReference is provided when the reason
	// for a blueprint spec load error is due to one or more references
	// being invalid.
	ErrorReasonCodeInvalidReference bpcore.ErrorReasonCode = "invalid_reference"
	// ErrorReasonCodeInvalidInclude is provided when the reason
	// for a blueprint spec load error is due to one or more includes
	// being invalid.
	ErrorReasonCodeInvalidInclude bpcore.ErrorReasonCode = "invalid_include"
)

func errVariableInvalidDefaultValue(varType schema.VariableType, varName string, defaultValue *bpcore.ScalarValue) error {
	defaultVarType := deriveVarType(defaultValue)

	return &bpcore.LoadError{
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
	return &bpcore.LoadError{
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
		return &bpcore.LoadError{
			ReasonCode: ErrorReasonCodeInvalidVariable,
			Err: fmt.Errorf(
				"validation failed to a missing value for variable \"%s\", a value of type %s must be provided",
				varName,
				varType,
			),
		}
	}

	return &bpcore.LoadError{
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
	return &bpcore.LoadError{
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
	return &bpcore.LoadError{
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
	return &bpcore.LoadError{
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
	return &bpcore.LoadError{
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
	return &bpcore.LoadError{
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
	return &bpcore.LoadError{
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
	return &bpcore.LoadError{
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
	return &bpcore.LoadError{
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

	return &bpcore.LoadError{
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
	return &bpcore.LoadError{
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
	return &bpcore.LoadError{
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

func errInvalidExportType(exportType schema.ExportType, exportName string) error {
	validExportTypes := strings.Join(
		core.Map(
			schema.ExportTypes,
			func(exportType schema.ExportType, index int) string {
				return string(exportType)
			},
		),
		", ",
	)
	return &bpcore.LoadError{
		ReasonCode: ErrorReasonCodeInvalidExport,
		Err: fmt.Errorf(
			"validation failed due to an invalid export type of \"%s\" being provided for export \"%s\". "+
				"The following export types are supported: %s",
			exportType,
			exportName,
			validExportTypes,
		),
	}
}

func errEmptyExportField(exportName string) error {
	return &bpcore.LoadError{
		ReasonCode: ErrorReasonCodeInvalidExport,
		Err: fmt.Errorf(
			"validation failed due to an empty field string being provided for export \"%s\"",
			exportName,
		),
	}
}

func errReferenceContextAccess(reference string, context string, referenceableType Referenceable) error {
	referencedObjectLabel := referenceableLabel(referenceableType)
	return &bpcore.LoadError{
		ReasonCode: ErrorReasonCodeInvalidReference,
		Err: fmt.Errorf(
			"validation failed due to a reference to a %s (\"%s\") being made from \"%s\", "+
				"which can not access values from a %s",
			referencedObjectLabel,
			reference,
			context,
			referencedObjectLabel,
		),
	}
}

func errInvalidReferencePattern(reference string, context string, referenceableType Referenceable) error {
	return &bpcore.LoadError{
		ReasonCode: ErrorReasonCodeInvalidReference,
		Err: fmt.Errorf(
			"validation failed due to an incorrectly formed reference to a %s (\"%s\") in \"%s\". "+
				"See the spec documentation for examples and rules for references",
			referenceableLabel(referenceableType),
			reference,
			context,
		),
	}
}

func errIncludeEmptyPath(includeName string) error {
	return &bpcore.LoadError{
		ReasonCode: ErrorReasonCodeInvalidInclude,
		Err: fmt.Errorf(
			"validation failed due to an empty path being provided for include \"%s\"",
			includeName,
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

func scalarListToString(scalars []*bpcore.ScalarValue) string {
	scalarStrings := make([]string, len(scalars))
	for i, scalar := range scalars {
		scalarStrings[i] = deriveScalarValueAsString(scalar)
	}

	return strings.Join(scalarStrings, ", ")
}

func deriveValueLabel(value *bpcore.ScalarValue, usingDefault bool) string {
	if usingDefault {
		return "default value"
	}

	return "value"
}
