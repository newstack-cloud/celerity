package validation

import (
	"fmt"
	"strings"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/errors"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
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
	// ErrorReasonCodeInvalidSubstitution is provided when the reason
	// for a blueprint spec load error is due to one or more substitutions
	// being invalid.
	ErrorReasonCodeInvalidSubstitution errors.ErrorReasonCode = "invalid_substitution"
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
	// ErrorReasonCodeReferenceCycle is provided when the reason
	// for a blueprint spec load error is due to a reference cycle being detected.
	// This error code is used to collect and surface reference cycle errors
	// for pure substitution reference cycles and link <-> substitution reference cycles.
	ErrorReasonCodeReferenceCycle errors.ErrorReasonCode = "reference_cycle"
	// ErrorReasonCodeInvalidMappingNode is provided when the reason
	// for a blueprint spec load error is due to an invalid mapping node.
	ErrorReasonCodeInvalidMappingNode errors.ErrorReasonCode = "invalid_mapping_node"
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
	valueLabel := deriveValueLabel(usingDefault)
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
	valueLabel := deriveValueLabel(usingDefault)
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

func errSubFuncInvalidNumberOfArgs(
	expectedParamCount int,
	passedArgCount int,
	subFunc *substitutions.SubstitutionFunctionExpr,
) error {
	line, col := source.PositionFromSourceMeta(subFunc.SourceMeta)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to an invalid number of arguments "+
				"being provided for substitution function \"%s\", expected %d but got %d",
			subFunc.FunctionName,
			expectedParamCount,
			passedArgCount,
		),
		Line:   line,
		Column: col,
	}
}

func errSubFuncArgTypeMismatch(
	argIndex int,
	expectedType string,
	actualType string,
	funcName string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to an invalid argument type being provided for substitution function \"%s\", "+
				"expected argument %d to be of type %s but got %s",
			funcName,
			argIndex,
			expectedType,
			actualType,
		),
		Line:   line,
		Column: col,
	}
}

func errSubFuncArgInvalidStringChoice(
	argIndex int,
	expectedChoices []string,
	actualValue string,
	funcName string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to an invalid argument value being provided for substitution function \"%s\", "+
				"expected argument %d to be one of the following choices: %s but got \"%s\"",
			funcName,
			argIndex,
			strings.Join(expectedChoices, ", "),
			actualValue,
		),
		Line:   line,
		Column: col,
	}
}

func errSubFuncNamedArgsNotAllowed(
	argName string,
	funcName string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to named arguments being provided for substitution function \"%s\", "+
				"found named argument \"%s\", named arguments are only supported in the \"%s\" function",
			funcName,
			argName,
			substitutions.SubstitutionFunctionObject,
		),
		Line:   line,
		Column: col,
	}
}

func errSubVarNotFound(
	varName string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to the variable \"%s\" not existing in the blueprint",
			varName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubValSelfReference(
	valName string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to the value \"%s\" referencing itself",
			valName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubValNotFound(
	valName string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to the value \"%s\" not existing in the blueprint",
			valName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubElemRefNotInResource(
	elemRefType string,
	location *source.Meta,
) error {
	elemRefTypeLabel := deriveElemRefTypeLabel(elemRefType)
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to an %s reference being used outside of a resource",
			elemRefTypeLabel,
		),
		Line:   line,
		Column: col,
	}
}

func errSubElemRefResourceNotFound(
	elemRefType string,
	resourceName string,
	location *source.Meta,
) error {
	elemRefTypeLabel := deriveElemRefTypeLabel(elemRefType)
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to the resource \"%s\" for %s reference not existing in the blueprint",
			resourceName,
			elemRefTypeLabel,
		),
		Line:   line,
		Column: col,
	}
}

func errSubElemRefResourceNotEach(
	elemRefType string,
	resourceName string,
	location *source.Meta,
) error {
	elemRefTypeLabel := deriveElemRefTypeLabel(elemRefType)
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to the resource \"%s\" for %s reference not "+
				"being a resource template, a resource template must have the `each` property defined",
			resourceName,
			elemRefTypeLabel,
		),
		Line:   line,
		Column: col,
	}
}

func errSubResourceNotFound(
	resourceName string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to the resource \"%s\" not existing in the blueprint",
			resourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubResourceSelfReference(
	resourceName string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to the resource \"%s\" referencing itself",
			resourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubDataSourceNotFound(
	dataSourceName string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to the data source \"%s\" not existing in the blueprint",
			dataSourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubDataSourceSelfReference(
	dataSourceName string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to the data source \"%s\" referencing itself",
			dataSourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubChildBlueprintNotFound(
	childName string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to the child blueprint \"%s\" not existing in the blueprint",
			childName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubChildBlueprintSelfReference(
	childName string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to the child blueprint \"%s\" referencing itself",
			childName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubResourceNotEach(
	resourceName string,
	indexAccessed *int64,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed as the index %d is accessed for resource \"%s\""+
				" which is not a resource template, "+
				"a resource template must have the `each` property defined",
			*indexAccessed,
			resourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubDataSourceNoExportedFields(
	dataSourceName string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to no fields being exported for data source \"%s\" "+
				"referenced in substitution",
			dataSourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubDataSourceFieldNotExported(
	dataSourceName string,
	field string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to the field \"%s\" referenced in the substitution"+
				" not being an exported field for data source \"%s\"",
			field,
			dataSourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubDataSourceFieldMissingType(
	dataSourceName string,
	field string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to the field \"%s\" referenced in the substitution"+
				" not having a type defined for data source \"%s\"",
			field,
			dataSourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubDataSourceFieldNotArray(
	dataSourceName string,
	field string,
	indexAccessed int64,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed as the field \"%s\" being referenced with index \"%d\" in the substitution"+
				" is not an array for data source \"%s\"",
			field,
			indexAccessed,
			dataSourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errResourceTypeMissingSpecDefinition(
	resourceName string,
	resourceType string,
	inSubstitution bool,
	resourceSourceMeta *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(resourceSourceMeta)
	contextInfo := ""
	if inSubstitution {
		contextInfo = " referenced in substitution"
	}
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to a missing spec definition for resource \"%s\" "+
				"of type \"%s\"%s",
			resourceName,
			resourceType,
			contextInfo,
		),
		Line:   line,
		Column: col,
	}
}

func errResourceTypeSpecDefMissingSchema(
	resourceName string,
	resourceType string,
	inSubstitution bool,
	resourceSourceMeta *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(resourceSourceMeta)
	contextInfo := ""
	if inSubstitution {
		contextInfo = " referenced in substitution"
	}
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to a missing spec definition schema for resource \"%s\" "+
				"of type \"%s\"%s",
			resourceName,
			resourceType,
			contextInfo,
		),
		Line:   line,
		Column: col,
	}
}

func errDataSourceTypeMissingSpecDefinition(
	dataSourceName string,
	dataSourceType string,
	dataSourceLocation *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(dataSourceLocation)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidDataSource,
		Err: fmt.Errorf(
			"validation failed due to a missing spec definition for data source \"%s\" "+
				"of type \"%s\"",
			dataSourceName,
			dataSourceType,
		),
		Line:   line,
		Column: col,
	}
}

func errDataSourceTypeMissingFields(
	dataSourceName string,
	dataSourceType string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidDataSource,
		Err: fmt.Errorf(
			"validation failed due to a missing fields definition for data source \"%s\" "+
				"of type \"%s\"",
			dataSourceName,
			dataSourceType,
		),
		Line:   line,
		Column: col,
	}
}

func errDataSourceFilterFieldNotSupported(
	dataSourceName string,
	field string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidDataSource,
		Err: fmt.Errorf(
			"validation failed due to the field \"%s\" in the filter for data source \"%s\" "+
				"not being supported",
			field,
			dataSourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubResourceSpecInvalidRef(
	resourceName string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed as the spec reference for resource \"%s\" is not valid",
			resourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubResourceMetadataInvalidRef(
	resourceName string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed as the metadata reference for resource \"%s\" is not valid",
			resourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubResourceMetadataInvalidProperty(
	resourceName string,
	property string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed as the metadata property \"%s\" provided for resource \"%s\" is not valid",
			property,
			resourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubResourceMetadataInvalidDisplayNameRef(
	resourceName string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed as the metadata display name reference for "+
				"resource \"%s\" provided can not have children",
			resourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubResourceMetadataInvalidAnnotationsRef(
	resourceName string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed as the metadata annotations reference for "+
				"resource \"%s\" was invalid, must be of the form "+
				"`metadata.annotations.<key>` or `metadata.annotations[\"<key>\"]`",
			resourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubResourceMetadataMissingAnnotation(
	resourceName string,
	annotationKey string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed as the metadata annotation \"%s\" for "+
				"resource \"%s\" was not found",
			annotationKey,
			resourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubResourceMetadataInvalidLabelsRef(
	resourceName string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed as the metadata labels reference for "+
				"resource \"%s\" was invalid, must be of the form "+
				"`metadata.labels.<key>` or `metadata.labels[\"<key>\"]`",
			resourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubResourceMetadataMissingLabel(
	resourceName string,
	labelKey string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed as the metadata label \"%s\" for "+
				"resource \"%s\" was not found",
			labelKey,
			resourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errSubResourcePropertyNotFound(
	resourceName string,
	path []*substitutions.SubstitutionPathItem,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed as %s is not valid for resource \"%s\"",
			subPathToString(path),
			resourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errInvalidDescriptionSubType(
	usedIn string,
	resolvedType string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to an invalid substitution found in %q, "+
				"resolved type %q is not supported by descriptions, "+
				"only values that resolve as strings are supported",
			usedIn,
			resolvedType,
		),
		Line:   line,
		Column: col,
	}
}

func errInvalidIncludePathSubType(
	usedIn string,
	resolvedType string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to an invalid substitution found in %q, "+
				"resolved type %q is not supported by include paths, "+
				"only values that resolve as strings are supported",
			usedIn,
			resolvedType,
		),
		Line:   line,
		Column: col,
	}
}

func errInvalidDisplayNameSubType(
	usedIn string,
	resolvedType string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to an invalid substitution found in %q, "+
				"resolved type %q is not supported by display names, "+
				"only values that resolve as strings are supported",
			usedIn,
			resolvedType,
		),
		Line:   line,
		Column: col,
	}
}

func errInvalidSubType(
	usedIn string,
	valueContext string,
	resolvedType string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to an invalid substitution found in %q, "+
				"resolved type %q is not supported by %ss, "+
				"only values that resolve as primitives are supported",
			usedIn,
			resolvedType,
			valueContext,
		),
		Line:   line,
		Column: col,
	}
}

func errInvalidSubTypeNotBoolean(
	usedIn string,
	valueContext string,
	resolvedType string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to an invalid substitution found in %q, "+
				"resolved type %q is not supported by %ss, "+
				"only values that resolve as booleans are supported",
			usedIn,
			resolvedType,
			valueContext,
		),
		Line:   line,
		Column: col,
	}
}

func errInvalidSubTypeNotArray(
	usedIn string,
	valueContext string,
	resolvedType string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidSubstitution,
		Err: fmt.Errorf(
			"validation failed due to an invalid substitution found in %q, "+
				"resolved type %q is not supported in %s, "+
				"only values that resolve as arrays are supported",
			usedIn,
			resolvedType,
			valueContext,
		),
		Line:   line,
		Column: col,
	}
}

func errMissingValueContent(
	valueID string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidValue,
		Err: fmt.Errorf(
			"validation failed as an empty value was found in %q, "+
				"values must be populated with a value that resolves to the defined value type",
			valueID,
		),
		Line:   line,
		Column: col,
	}
}

func errValueIncorrectTypeInterpolatedString(
	usedIn string,
	valueType string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidValue,
		Err: fmt.Errorf(
			"validation failed due to an interpolated string being used in %q, "+
				"value type %q does not support interpolated strings",
			usedIn,
			valueType,
		),
		Line:   line,
		Column: col,
	}
}

func errInvalidValueSubType(
	usedIn string,
	resolvedType string,
	expectedResolvedType string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidValue,
		Err: fmt.Errorf(
			"validation failed due to an invalid substitution found in %q, "+
				"resolved type %q is not supported by value of type %q",
			usedIn,
			resolvedType,
			expectedResolvedType,
		),
		Line:   line,
		Column: col,
	}
}

func errMissingValueType(
	valName string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidValue,
		Err: fmt.Errorf(
			"validation failed as the value %q is missing a type, "+
				"all values must have a type defined",
			valName,
		),
		Line:   line,
		Column: col,
	}
}

// ErrReferenceCycles is used to wrap errors that occurred during reference cycle validation.
// This error is used to collect and surface reference cycle errors for pure substitution reference
// cycles and link <-> substitution reference cycles.
func ErrReferenceCycles(rootRefChains []*ReferenceChain) error {
	var errs []error
	for _, refChain := range rootRefChains {
		errs = append(errs, &errors.LoadError{
			ReasonCode: ErrorReasonCodeReferenceCycle,
			Err: fmt.Errorf(
				"validation failed due to a reference cycle in the blueprint, "+
					"the cycle started with element: %q, this could be due to explicit references between elements "+
					"or an implicit link conflicting with an explicit item reference",
				refChain.ElementName,
			),
		})
	}
	return ErrMultipleValidationErrors(errs)
}

func errMissingMappingNodeValue(
	context string,
	propertyPath string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidMappingNode,
		Err: fmt.Errorf(
			"validation failed due to a missing value for property %q in %q",
			propertyPath,
			context,
		),
		Line:   line,
		Column: col,
	}
}

func errDataSourceExportFieldNotSupported(
	dataSourceName string,
	dataSourceType string,
	exportAlias string,
	exportedSourceField string,
	wrapperLocation *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(wrapperLocation)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidDataSource,
		Err: fmt.Errorf(
			"validation failed due to the exported field %q in data source %q not being supported, "+
				"the exported field %q is not present for data source type %q",
			exportAlias,
			dataSourceName,
			exportedSourceField,
			dataSourceType,
		),
		Line:   line,
		Column: col,
	}
}

func errDataSourceExportFieldTypeMismatch(
	dataSourceName string,
	exportAlias string,
	dataSourceField string,
	dataSourceFieldType string,
	exportedFieldType string,
	wrapperLocation *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(wrapperLocation)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidDataSource,
		Err: fmt.Errorf(
			"validation failed due to the exported field %q in data source %q having an unexpected type, "+
				"the data source field %q has a type of %q, but the exported type is %q",
			exportAlias,
			dataSourceName,
			dataSourceField,
			dataSourceFieldType,
			exportedFieldType,
		),
		Line:   line,
		Column: col,
	}
}

func errDataSourceExportEmpty(
	dataSourceName string,
	exportName string,
	wrapperLocation *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(wrapperLocation)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidDataSource,
		Err: fmt.Errorf(
			"validation failed due to the exported field %q in data source %q having an empty value",
			exportName,
			dataSourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errDataSourceExportTypeMissing(
	dataSourceName string,
	exportName string,
	wrapperLocation *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(wrapperLocation)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidDataSource,
		Err: fmt.Errorf(
			"validation failed due to export %q in data source %q missing a type",
			exportName,
			dataSourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errDataSourceTypeNotSupported(
	dataSourceName string,
	dataSourceType string,
	wrapperLocation *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(wrapperLocation)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidDataSource,
		Err: fmt.Errorf(
			"validation failed due to data source %q having an unsupported type %q,"+
				" this type is not made available by any of the loaded providers",
			dataSourceName,
			dataSourceType,
		),
		Line:   line,
		Column: col,
	}
}

func errDataSourceAnnotationKeyContainsSubstitution(
	dataSourceName string,
	annotationKey string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidDataSource,
		Err: fmt.Errorf(
			"validation failed due to an annotation key containing a substitution in data source %q, "+
				"the annotation key %q can not contain substitutions",
			dataSourceName,
			annotationKey,
		),
		Line:   line,
		Column: col,
	}
}

func errResourceTypeNotSupported(
	resourceName string,
	resourceType string,
	wrapperLocation *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(wrapperLocation)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to resource %q having an unsupported type %q,"+
				" this type is not made available by any of the loaded providers",
			resourceName,
			resourceType,
		),
		Line:   line,
		Column: col,
	}
}

func errLabelKeyContainsSubstitution(
	resourceName string,
	labelKey string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to a label key containing a substitution in resource %q, "+
				"the label key %q can not contain substitutions",
			resourceName,
			labelKey,
		),
		Line:   line,
		Column: col,
	}
}

func errLabelValueContainsSubstitution(
	resourceName string,
	labelKey string,
	labelValue string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to a label value containing a substitution in resource %q, "+
				"the label %q with value %q can not contain substitutions",
			resourceName,
			labelKey,
			labelValue,
		),
		Line:   line,
		Column: col,
	}
}

func errLinkSelectorKeyContainsSubstitution(
	resourceName string,
	linkSelectorKey string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to a link selector \"byLabel\" key containing a "+
				"substitution in resource %q, "+
				"the link selector label key %q can not contain substitutions",
			resourceName,
			linkSelectorKey,
		),
		Line:   line,
		Column: col,
	}
}

func errLinkSelectorValueContainsSubstitution(
	resourceName string,
	linkSelectorKey string,
	linkSelectorValue string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to a link selector \"byLabel\" value containing a "+
				"substitution in resource %q, "+
				"the link selector label %q with value %q can not contain substitutions",
			resourceName,
			linkSelectorKey,
			linkSelectorValue,
		),
		Line:   line,
		Column: col,
	}
}

func errAnnotationKeyContainsSubstitution(
	resourceName string,
	annotationKey string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to an annotation key containing a substitution in resource %q, "+
				"the annotation key %q can not contain substitutions",
			resourceName,
			annotationKey,
		),
		Line:   line,
		Column: col,
	}
}

func errNestedResourceConditionEmpty(
	resourceName string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResource,
		Err: fmt.Errorf(
			"validation failed due to a nested condition for resource %q being empty, "+
				"all nested conditions must have a value defined",
			resourceName,
		),
		Line:   line,
		Column: col,
	}
}

func errExportTypeMismatch(
	exportType schema.ExportType,
	resolvedType string,
	exportName string,
	field string,
	location *source.Meta,
) error {
	line, col := source.PositionFromSourceMeta(location)
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidExport,
		Err: fmt.Errorf(
			"validation failed due to a type mismatch in export %q, "+
				"the expected export type %s does not match the resolved type %s for field %q",
			exportName,
			exportType,
			resolvedType,
			field,
		),
		Line:   line,
		Column: col,
	}
}

func deriveElemRefTypeLabel(elemRefType string) string {
	switch elemRefType {
	case "index":
		return "element index"
	default:
		return "element"
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

func deriveValueLabel(usingDefault bool) string {
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

func subPathToString(path []*substitutions.SubstitutionPathItem) string {
	sb := strings.Builder{}
	for _, item := range path {
		if item.FieldName != "" {
			fieldStr := fmt.Sprintf("[\"%s\"]", item.FieldName)
			sb.WriteString(fieldStr)
		} else {
			pathStr := fmt.Sprintf("[%d]", *item.PrimitiveArrIndex)
			sb.WriteString(pathStr)
		}
	}

	return sb.String()
}
