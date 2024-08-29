package validation

import (
	"fmt"
	"strings"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/errors"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/common/core"
)

const (
	// ErrorReasonCodeMissingType is provided when the reason
	// for a blueprint spec load error is due to the version property
	// not being provided for a blueprint.
	ErrorReasonCodeMissingVersion errors.ErrorReasonCode = "missing_version"
	// ErrorReasonCodeInvalidVersion is provided when the reason
	// for a blueprint spec load error is due to an invalid version
	// of the spec being provided.
	ErrorReasonCodeInvalidVersion errors.ErrorReasonCode = "invalid_version"
	// ErrorReasonCodeInvalidResource is provided when the reason
	// for a blueprint spec load error is due to one or more resources
	// being invalid.
	ErrorReasonCodeInvalidResource errors.ErrorReasonCode = "invalid_resource"
	// ErrorReasonCodeMissingResources is provided when the reason
	// for a blueprint spec load error is due to an empty map of resources being
	// provided or where the resources property is omitted.
	ErrorReasonCodeMissingResources errors.ErrorReasonCode = "missing_resources"
	// ErrorReasonCodeInvalidVariable is provided when the reason
	// for a blueprint spec load error is due to one or more variables
	// being invalid.
	// This could be due to a mismatch between the type and the value,
	// a missing required variable (one without a default value),
	// an invalid default value, invalid allowed values or an incorrect variable type.
	ErrorReasonCodeInvalidVariable errors.ErrorReasonCode = "invalid_variable"
	// ErrorReasonCodeInvalidValue is provided when the reason
	// for a blueprint spec load error is due to an invalid value
	// being provided.
	ErrorReasonCodeInvalidValue errors.ErrorReasonCode = "invalid_value"
	// ErrorReasonCodeInvalidExport is provided when the reason
	// for a blueprint spec load error is due to one or more exports
	// being invalid.
	ErrorReasonCodeInvalidExport errors.ErrorReasonCode = "invalid_export"
	// ErrorReasonCodeInvalidReference is provided when the reason
	// for a blueprint spec load error is due to one or more references
	// being invalid.
	ErrorReasonCodeInvalidReference errors.ErrorReasonCode = "invalid_reference"
	// ErrorReasonCodeInvalidInclude is provided when the reason
	// for a blueprint spec load error is due to one or more includes
	// being invalid.
	ErrorReasonCodeInvalidInclude errors.ErrorReasonCode = "invalid_include"
	// ErrorReasonCodeInvalidResource is provided when the reason
	// for a blueprint spec load error is due to one or more data sources
	// being invalid.
	ErrorReasonCodeInvalidDataSource errors.ErrorReasonCode = "invalid_data_source"
	// ErrorReasonCodeInvalidMapKey is provided when the reason
	// for a blueprint spec load error is due to an invalid map key.
	ErrorReasonCodeInvalidMapKey errors.ErrorReasonCode = "invalid_map_key"
	// ErrorReasonCodeMultipleValidationErrors is provided when the reason
	// for a blueprint spec load error is due to multiple validation errors.
	ErrorReasonCodeMultipleValidationErrors errors.ErrorReasonCode = "multiple_validation_errors"
)

func errBlueprintMissingVersion() error {
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeMissingVersion,
		Err:        fmt.Errorf("validation failed due to a version not being provided, version is a required property"),
	}
}

func errBlueprintMissingResources() error {
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeMissingResources,
		Err: fmt.Errorf(
			"validation failed due to an empty set of resources," +
				" at least one resource must be defined in a blueprint",
		),
	}
}

func errBlueprintUnsupportedVersion(version string) error {
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidVersion,
		Err: fmt.Errorf(
			"validation failed due to an unsupported version \"%s\" being provided. "+
				"supported versions include: %s",
			version,
			strings.Join(SupportedVersions, ", "),
		),
	}
}

func errMappingNameContainsSubstitution(
	mappingName string,
	mappingType string,
	reasonCode errors.ErrorReasonCode,
	varSourceMeta *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(varSourceMeta)
	return &errors.LoadError{
		ReasonCode: reasonCode,
		Err: fmt.Errorf(
			"${..} substitutions can not be used in %s names, found in %s \"%s\"",
			mappingType,
			mappingType,
			mappingName,
		),
		Line:   line,
		Column: col,
	}
}

func errVariableInvalidDefaultValue(
	varType schema.VariableType,
	varName string,
	defaultValue *bpcore.ScalarValue,
	varSourceMeta *source.Meta,
) error {
	defaultVarType := deriveVarType(defaultValue)

	line, col := positionFromScalarValue(defaultValue, varSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to an invalid default value for variable \"%s\", %s was provided when %s was expected",
			varName,
			defaultVarType,
			varType,
		),
		Line:   line,
		Column: col,
	}
}

func errVariableEmptyDefaultValue(varType schema.VariableType, varName string, varSourceMeta *source.Meta) error {
	line, col := source.PositionFromSourceMeta(varSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to an empty default %s value for variable \"%s\", you must provide a value when declaring a default in a blueprint",
			varType,
			varName,
		),
		Line:   line,
		Column: col,
	}
}

func errVariableInvalidOrMissing(
	varType schema.VariableType,
	varName string,
	value *bpcore.ScalarValue,
	varSourceMeta *source.Meta,
) error {
	actualVarType := deriveOptionalVarType(value)
	if actualVarType == nil {
		line, col := source.PositionFromSourceMeta(varSourceMeta)
		return &errors.LoadError{
			ReasonCode: ErrorReasonCodeInvalidVariable,
			Err: fmt.Errorf(
				"validation failed to a missing value for variable \"%s\", a value of type %s must be provided",
				varName,
				varType,
			),
			Line:   line,
			Column: col,
		}
	}

	line, col := source.PositionFromSourceMeta(varSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to an incorrect type used for variable \"%s\", "+
				"expected a value of type %s but one of type %s was provided",
			varName,
			varType,
			*actualVarType,
		),
		Line:   line,
		Column: col,
	}
}

func errVariableEmptyValue(
	varType schema.VariableType,
	varName string,
	varSourceMeta *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(varSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to an empty value being provided for variable \"%s\", "+
				"please provide a valid %s value that is not empty",
			varName,
			varType,
		),
		Line:   line,
		Column: col,
	}
}

func errVariableInvalidAllowedValue(
	varType schema.VariableType,
	allowedValue *bpcore.ScalarValue,
	varSourceMeta *source.Meta,
) error {
	allowedValueVarType := deriveVarType(allowedValue)
	scalarValueStr := deriveScalarValueAsString(allowedValue)

	line, col := positionFromScalarValue(allowedValue, varSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"an invalid allowed value was provided, %s with the value \"%s\" was provided when only %ss are allowed",
			varTypeToUnit(allowedValueVarType),
			scalarValueStr,
			varType,
		),
		Line:   line,
		Column: col,
	}
}

func errVariableNullAllowedValue(
	varType schema.VariableType,
	allowedValue *bpcore.ScalarValue,
	varSourceMeta *source.Meta,
) error {
	line, col := positionFromScalarValue(allowedValue, varSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"null was provided for an allowed value, a valid %s must be provided",
			varType,
		),
		Line:   line,
		Column: col,
	}
}

func errVariableInvalidAllowedValues(
	varName string,
	allowedValueErrors []error,
) error {
	return &errors.LoadError{
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
	varSourceMeta *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(varSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to an allowed values list being provided for %s variable \"%s\","+
				" %s variables do not support allowed values enumeration",
			varType,
			varName,
			varType,
		),
		Line:   line,
		Column: col,
	}
}

func errVariableValueNotAllowed(
	varType schema.VariableType,
	varName string,
	value *bpcore.ScalarValue,
	allowedValues []*bpcore.ScalarValue,
	varSourceMeta *source.Meta,
	usingDefault bool,
) error {
	valueLabel := deriveValueLabel(value, usingDefault)
	line, col := positionFromScalarValue(value, varSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to an invalid %s being provided for %s variable \"%s\","+
				" only the following values are supported: %s",
			valueLabel,
			varType,
			varName,
			scalarListToString(allowedValues),
		),
		Line:   line,
		Column: col,
	}
}

func errCustomVariableValueNotInOptions(
	varType schema.VariableType,
	varName string,
	value *bpcore.ScalarValue,
	varSourceMeta *source.Meta,
	usingDefault bool,
) error {
	valueLabel := deriveValueLabel(value, usingDefault)
	line, col := positionFromScalarValue(value, varSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to an invalid %s \"%s\" being provided for variable \"%s\","+
				" which is not a valid %s option, see the custom type documentation for more details",
			valueLabel,
			deriveScalarValueAsString(value),
			varName,
			varType,
		),
		Line:   line,
		Column: col,
	}
}

func errRequiredVariableMissing(varName string, varSourceMeta *source.Meta) error {
	line, col := source.PositionFromSourceMeta(varSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to a value not being provided for the "+
				"required variable \"%s\", as it does not have a default",
			varName,
		),
		Line:   line,
		Column: col,
	}
}

func errCustomVariableOptions(
	varName string,
	varSchema *schema.Variable,
	varSourceMeta *source.Meta,
	err error,
) error {
	line, col := source.PositionFromSourceMeta(varSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to an error when loading options for variable \"%s\" of custom type \"%s\"",
			varName,
			varSchema.Type,
		),
		ChildErrors: []error{err},
		Line:        line,
		Column:      col,
	}
}

func errCustomVariableMixedTypes(
	varName string,
	varSchema *schema.Variable,
	varSourceMeta *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(varSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to mixed types provided as options for variable type \"%s\" used in variable \"%s\", "+
				"all options must be of the same scalar type",
			varSchema.Type,
			varName,
		),
		Line:   line,
		Column: col,
	}
}

func errCustomVariableInvalidDefaultValueType(
	varType schema.VariableType,
	varName string,
	defaultValue *bpcore.ScalarValue,
	varSourceMeta *source.Meta,
) error {
	defaultVarType := deriveVarType(defaultValue)
	line, col := positionFromScalarValue(defaultValue, varSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to an invalid type for a default value for variable \"%s\", %s was provided "+
				"when a custom variable type option of %s was expected",
			varName,
			defaultVarType,
			varType,
		),
		Line:   line,
		Column: col,
	}
}

func errCustomVariableAllowedValuesNotInOptions(
	varType schema.VariableType,
	varName string,
	invalidOptions []string,
	varSourceMeta *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(varSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to invalid allowed values being provided for variable \"%s\" "+
				"of custom type \"%s\". See custom type documentation for possible values. Invalid values provided: %s",
			varName,
			varType,
			strings.Join(invalidOptions, ", "),
		),
		Line:   line,
		Column: col,
	}
}

func errCustomVariableDefaultValueNotInOptions(
	varType schema.VariableType,
	varName string,
	defaultValue string,
	varSourceMeta *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(varSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidVariable,
		Err: fmt.Errorf(
			"validation failed due to an invalid default value for variable \"%s\" "+
				"of custom type \"%s\". See custom type documentation for possible values. Invalid default value provided: %s",
			varName,
			varType,
			defaultValue,
		),
		Line:   line,
		Column: col,
	}
}

func errInvalidExportType(exportType schema.ExportType, exportName string, exportSourceMeta *source.Meta) error {
	validExportTypes := strings.Join(
		core.Map(
			schema.ExportTypes,
			func(exportType schema.ExportType, index int) string {
				return string(exportType)
			},
		),
		", ",
	)
	line, col := source.PositionFromSourceMeta(exportSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidExport,
		Err: fmt.Errorf(
			"validation failed due to an invalid export type of \"%s\" being provided for export \"%s\". "+
				"The following export types are supported: %s",
			exportType,
			exportName,
			validExportTypes,
		),
		Line:   line,
		Column: col,
	}
}

func errEmptyExportField(exportName string, exportSourceMeta *source.Meta) error {
	line, col := source.PositionFromSourceMeta(exportSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidExport,
		Err: fmt.Errorf(
			"validation failed due to an empty field string being provided for export \"%s\"",
			exportName,
		),
		Line:   line,
		Column: col,
	}
}

func errReferenceContextAccess(reference string, context string, referenceableType Referenceable) error {
	referencedObjectLabel := referenceableLabel(referenceableType)
	return &errors.LoadError{
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
	return &errors.LoadError{
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

func errIncludeEmptyPath(includeName string, varSourceMeta *source.Meta) error {
	line, col := source.PositionFromSourceMeta(varSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidInclude,
		Err: fmt.Errorf(
			"validation failed due to an empty path being provided for include \"%s\"",
			includeName,
		),
		Line:   line,
		Column: col,
	}
}

func errDataSourceMissingFilter(dataSourceName string, dataSourceMeta *source.Meta) error {
	line, col := source.PositionFromSourceMeta(dataSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidDataSource,
		Err: fmt.Errorf(
			"validation failed due to a missing filter in "+
				"data source \"%s\", every data source must have a filter",
			dataSourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errDataSourceMissingFilterField(dataSourceName string, dataSourceMeta *source.Meta) error {
	line, col := source.PositionFromSourceMeta(dataSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidDataSource,
		Err: fmt.Errorf(
			"validation failed due to a missing field in filter for "+
				"data source \"%s\", field must be set for a data source filter",
			dataSourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errDataSourceMissingFilterSearch(dataSourceName string, dataSourceMeta *source.Meta) error {
	line, col := source.PositionFromSourceMeta(dataSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidDataSource,
		Err: fmt.Errorf(
			"validation failed due to a missing search in filter for "+
				"data source \"%s\", at least one search value must be provided for a filter",
			dataSourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errDataSourceMissingExports(dataSourceName string, dataSourceMeta *source.Meta) error {
	line, col := source.PositionFromSourceMeta(dataSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidDataSource,
		Err: fmt.Errorf(
			"validation failed due to missing exports for "+
				"data source \"%s\", at least one field must be exported for a data source",
			dataSourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errResourceSpecPreValidationFailed(errs []error, resourceName string, resourceSourceMeta *source.Meta) error {
	line, col := source.PositionFromSourceMeta(resourceSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to errors in the pre-validation of the resource spec for resource \"%s\"",
			resourceName,
		),
		ChildErrors: errs,
		Line:        line,
		Column:      col,
	}
}

// errMultipleValidationErrors is used to wrap multiple errors that occurred during validation.
// The idea is to collect and surface as many validation errors to the user as possible
// to provide them the full picture of issues in the blueprint instead of just the first error.
func ErrMultipleValidationErrors(errs []error) error {
	return &errors.LoadError{
		ReasonCode:  ErrorReasonCodeMultipleValidationErrors,
		Err:         fmt.Errorf("validation failed due to multiple errors"),
		ChildErrors: errs,
	}
}

func errMappingNodeKeyContainsSubstitution(
	key string,
	nodeParentType string,
	nodeParentName string,
	nodeSourceMeta *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(nodeSourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidMapKey,
		Err: fmt.Errorf(
			"${..} substitutions can not be used in map keys,"+
				" found \"%s\" in child mapping key of %s \"%s\"",
			key,
			nodeParentType,
			nodeParentName,
		),
		Line:   line,
		Column: col,
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

func positionFromScalarValue(value *bpcore.ScalarValue, parentSourceMeta *source.Meta) (line, col *int) {
	if value == nil {
		if parentSourceMeta != nil {
			return source.PositionFromSourceMeta(parentSourceMeta)
		}
		return nil, nil
	}

	return source.PositionFromSourceMeta(value.SourceMeta)
}
