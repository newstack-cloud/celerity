package validation

import (
	"context"
	"strings"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/source"
)

// ValidateCoreVariable deals with validating a blueprint variable
// against the supported core scalar variable types in the blueprint
// specification.
func ValidateCoreVariable(
	ctx context.Context,
	varName string,
	varSchema *schema.Variable,
	varMap *schema.VariableMap,
	params bpcore.BlueprintParams,
	validateRuntimeParams bool,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	if varSchema.Type == schema.VariableTypeString {
		return validateCoreStringVariable(
			ctx, varName, varSchema, varMap, params, validateRuntimeParams,
		)
	}

	if varSchema.Type == schema.VariableTypeInteger {
		return validateCoreIntegerVariable(
			ctx, varName, varSchema, varMap, params, validateRuntimeParams,
		)
	}

	if varSchema.Type == schema.VariableTypeFloat {
		return validateCoreFloatVariable(
			ctx, varName, varSchema, varMap, params, validateRuntimeParams,
		)
	}

	if varSchema.Type == schema.VariableTypeBoolean {
		return validateCoreBooleanVariable(
			ctx, varName, varSchema, varMap, params, validateRuntimeParams,
		)
	}

	return diagnostics, nil
}

func validateCoreStringVariable(
	ctx context.Context,
	varName string,
	varSchema *schema.Variable,
	varMap *schema.VariableMap,
	params bpcore.BlueprintParams,
	validateRuntimeParams bool,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	if len(varSchema.AllowedValues) > 0 {
		err := validateCoreStringVariableAllowedValues(
			ctx, varName, varSchema, varMap, params,
		)

		if err != nil {
			return diagnostics, err
		}
	}

	// Catch default value issues initially, regardless of whether
	// or not the default value will be used in the variable instance.
	if varSchema.Default != nil && varSchema.Default.StringValue == nil {
		return diagnostics, errVariableInvalidDefaultValue(
			schema.VariableTypeString,
			varName,
			varSchema.Default,
			getVarSourceMeta(varMap, varName),
		)
	}

	if varSchema.Default != nil && strings.TrimSpace(*varSchema.Default.StringValue) == "" {
		return diagnostics, errVariableEmptyDefaultValue(
			schema.VariableTypeString,
			varName,
			getVarSourceMeta(varMap, varName),
		)
	}

	userProvidedValue := params.BlueprintVariable(varName)
	finalValue := fallbackToDefault(userProvidedValue, varSchema.Default)

	if validateRuntimeParams && finalValue == nil {
		return diagnostics, errRequiredVariableMissing(varName, getVarSourceMeta(varMap, varName))
	}

	if validateRuntimeParams && finalValue.StringValue == nil {
		return diagnostics, errVariableInvalidOrMissing(
			schema.VariableTypeString,
			varName,
			finalValue,
			getVarSourceMeta(varMap, varName),
		)
	}

	if validateRuntimeParams && strings.TrimSpace(*finalValue.StringValue) == "" {
		return diagnostics, errVariableEmptyValue(
			schema.VariableTypeString,
			varName,
			getVarSourceMeta(varMap, varName),
		)
	}

	return diagnostics, validateValueInAllowedList(
		varSchema,
		varMap,
		schema.VariableTypeString,
		finalValue,
		userProvidedValue,
		varName,
		validateRuntimeParams,
	)
}

func validateCoreStringVariableAllowedValues(
	ctx context.Context,
	varName string,
	varSchema *schema.Variable,
	varMap *schema.VariableMap,
	params bpcore.BlueprintParams,
) error {
	// Collect all invalid allowed values in one go to help
	// speed up the debugging process.
	invalidAllowedValueErrors := []error{}
	for _, allowedValue := range varSchema.AllowedValues {
		var err error
		if allowedValue == nil || scalarAllNil(allowedValue) {
			err = errVariableNullAllowedValue(
				schema.VariableTypeString,
				allowedValue,
				getVarSourceMeta(varMap, varName),
			)
		} else if allowedValue.StringValue == nil {
			err = errVariableInvalidAllowedValue(
				schema.VariableTypeString,
				allowedValue,
				getVarSourceMeta(varMap, varName),
			)
		}

		if err != nil {
			invalidAllowedValueErrors = append(invalidAllowedValueErrors, err)
		}
	}

	if len(invalidAllowedValueErrors) > 0 {
		return errVariableInvalidAllowedValues(
			varName,
			invalidAllowedValueErrors,
		)
	}

	return nil
}

func validateCoreIntegerVariable(
	ctx context.Context,
	varName string,
	varSchema *schema.Variable,
	varMap *schema.VariableMap,
	params bpcore.BlueprintParams,
	validateRuntimeParams bool,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	if len(varSchema.AllowedValues) > 0 {
		err := validateCoreIntegerVariableAllowedValues(
			ctx, varName, varSchema, varMap, params,
		)

		if err != nil {
			return diagnostics, err
		}
	}

	// Catch default value issues initially, regardless of whether
	// or not the default value will be used in the variable instance.
	if varSchema.Default != nil && varSchema.Default.IntValue == nil {
		return diagnostics, errVariableInvalidDefaultValue(
			schema.VariableTypeInteger,
			varName,
			varSchema.Default,
			getVarSourceMeta(varMap, varName),
		)
	}

	// No need for explicit empty value checks for an integer as the default empty value
	// for an integer in go (0) is a valid value.

	userProvidedValue := params.BlueprintVariable(varName)
	finalValue := fallbackToDefault(userProvidedValue, varSchema.Default)

	if validateRuntimeParams && finalValue == nil {
		return diagnostics, errRequiredVariableMissing(varName, getVarSourceMeta(varMap, varName))
	}

	if validateRuntimeParams && finalValue.IntValue == nil {
		return diagnostics, errVariableInvalidOrMissing(
			schema.VariableTypeInteger,
			varName,
			finalValue,
			getVarSourceMeta(varMap, varName),
		)
	}

	return diagnostics, validateValueInAllowedList(
		varSchema,
		varMap,
		schema.VariableTypeInteger,
		finalValue,
		userProvidedValue,
		varName,
		validateRuntimeParams,
	)
}

func validateCoreIntegerVariableAllowedValues(
	ctx context.Context,
	varName string,
	varSchema *schema.Variable,
	varMap *schema.VariableMap,
	params bpcore.BlueprintParams,
) error {
	// Collect all invalid allowed values in one go to help
	// speed up the debugging process.
	invalidAllowedValueErrors := []error{}
	for _, allowedValue := range varSchema.AllowedValues {
		var err error
		if allowedValue == nil || scalarAllNil(allowedValue) {
			err = errVariableNullAllowedValue(
				schema.VariableTypeInteger,
				allowedValue,
				getVarSourceMeta(varMap, varName),
			)
		} else if allowedValue.IntValue == nil {
			err = errVariableInvalidAllowedValue(
				schema.VariableTypeInteger,
				allowedValue,
				getVarSourceMeta(varMap, varName),
			)
		}

		if err != nil {
			invalidAllowedValueErrors = append(invalidAllowedValueErrors, err)
		}
	}

	if len(invalidAllowedValueErrors) > 0 {
		return errVariableInvalidAllowedValues(
			varName,
			invalidAllowedValueErrors,
		)
	}

	return nil
}

func validateCoreFloatVariable(
	ctx context.Context,
	varName string,
	varSchema *schema.Variable,
	varMap *schema.VariableMap,
	params bpcore.BlueprintParams,
	validateRuntimeParams bool,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	if len(varSchema.AllowedValues) > 0 {
		err := validateCoreFloatVariableAllowedValues(
			ctx, varName, varSchema, varMap, params,
		)

		if err != nil {
			return diagnostics, err
		}
	}

	// Catch default value issues initially, regardless of whether
	// or not the default value will be used in the variable instance.
	if varSchema.Default != nil && varSchema.Default.FloatValue == nil {
		return diagnostics, errVariableInvalidDefaultValue(
			schema.VariableTypeFloat,
			varName,
			varSchema.Default,
			getVarSourceMeta(varMap, varName),
		)
	}

	// No need for explicit empty value checks for a float as the default empty value
	// for a floating point number in go (0.0) is a valid value.

	userProvidedValue := params.BlueprintVariable(varName)
	finalValue := fallbackToDefault(userProvidedValue, varSchema.Default)

	if validateRuntimeParams && finalValue == nil {
		return diagnostics, errRequiredVariableMissing(varName, getVarSourceMeta(varMap, varName))
	}

	if validateRuntimeParams && finalValue.FloatValue == nil {
		return diagnostics, errVariableInvalidOrMissing(
			schema.VariableTypeFloat,
			varName,
			finalValue,
			getVarSourceMeta(varMap, varName),
		)
	}

	return diagnostics, validateValueInAllowedList(
		varSchema,
		varMap,
		schema.VariableTypeFloat,
		finalValue,
		userProvidedValue,
		varName,
		validateRuntimeParams,
	)
}

func validateCoreFloatVariableAllowedValues(
	ctx context.Context,
	varName string,
	varSchema *schema.Variable,
	varMap *schema.VariableMap,
	params bpcore.BlueprintParams,
) error {
	// Collect all invalid allowed values in one go to help
	// speed up the debugging process.
	invalidAllowedValueErrors := []error{}
	for _, allowedValue := range varSchema.AllowedValues {
		var err error
		if allowedValue == nil || scalarAllNil(allowedValue) {
			err = errVariableNullAllowedValue(
				schema.VariableTypeFloat,
				allowedValue,
				getVarSourceMeta(varMap, varName),
			)
		} else if allowedValue.FloatValue == nil {
			err = errVariableInvalidAllowedValue(
				schema.VariableTypeFloat,
				allowedValue,
				getVarSourceMeta(varMap, varName),
			)
		}

		if err != nil {
			invalidAllowedValueErrors = append(invalidAllowedValueErrors, err)
		}
	}

	if len(invalidAllowedValueErrors) > 0 {
		return errVariableInvalidAllowedValues(
			varName,
			invalidAllowedValueErrors,
		)
	}

	return nil
}

func validateCoreBooleanVariable(
	ctx context.Context,
	varName string,
	varSchema *schema.Variable,
	varMap *schema.VariableMap,
	params bpcore.BlueprintParams,
	validateRuntimeParams bool,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	if len(varSchema.AllowedValues) > 0 {
		return diagnostics, errVariableInvalidAllowedValuesNotSupported(
			schema.VariableTypeBoolean,
			varName,
			getVarSourceMeta(varMap, varName),
		)
	}

	// Catch default value issues initially, regardless of whether
	// or not the default value will be used in the variable instance.
	if varSchema.Default != nil && varSchema.Default.BoolValue == nil {
		return diagnostics, errVariableInvalidDefaultValue(
			schema.VariableTypeBoolean,
			varName,
			varSchema.Default,
			getVarSourceMeta(varMap, varName),
		)
	}

	// No need for explicit empty value checks for a boolean as the default empty value
	// for a boolean in go (false) is a valid value.

	value := params.BlueprintVariable(varName)
	if value == nil {
		value = varSchema.Default
	}

	if validateRuntimeParams && value == nil {
		return diagnostics, errRequiredVariableMissing(varName, getVarSourceMeta(varMap, varName))
	}

	if validateRuntimeParams && value.BoolValue == nil {
		return diagnostics, errVariableInvalidOrMissing(
			schema.VariableTypeBoolean,
			varName,
			value,
			getVarSourceMeta(varMap, varName),
		)
	}

	return diagnostics, nil
}

func validateValueInAllowedList(
	varSchema *schema.Variable,
	varMap *schema.VariableMap,
	varType schema.VariableType,
	finalValue *bpcore.ScalarValue,
	userProvidedValue *bpcore.ScalarValue,
	varName string,
	validateRuntimeParams bool,
) error {
	if len(varSchema.AllowedValues) > 0 && !bpcore.IsInScalarList(finalValue, varSchema.AllowedValues) {
		usingDefault := userProvidedValue == nil
		if usingDefault && !validateRuntimeParams {
			return nil
		}

		return errVariableValueNotAllowed(
			varType,
			varName,
			finalValue,
			varSchema.AllowedValues,
			getVarSourceMeta(varMap, varName),
			usingDefault,
		)
	}

	return nil
}

func fallbackToDefault(value *bpcore.ScalarValue, defaultValue *bpcore.ScalarValue) *bpcore.ScalarValue {
	if value == nil {
		return defaultValue
	}
	return value
}

func getVarSourceMeta(varMap *schema.VariableMap, varName string) *source.Meta {
	if varMap == nil {
		return nil
	}

	return varMap.SourceMeta[varName]
}

func scalarAllNil(scalar *bpcore.ScalarValue) bool {
	return scalar.StringValue == nil && scalar.IntValue == nil && scalar.FloatValue == nil && scalar.BoolValue == nil
}
