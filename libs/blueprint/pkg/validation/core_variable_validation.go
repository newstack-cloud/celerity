package validation

import (
	"context"
	"strings"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
)

// ValidateCoreVariable deals with validating a blueprint variable
// against the supported core scalar variable types in the blueprint
// specification.
func ValidateCoreVariable(
	ctx context.Context,
	varName string,
	varSchema *schema.Variable,
	params bpcore.BlueprintParams,
	validateRuntimeParams bool,
) error {
	if varSchema.Type == schema.VariableTypeString {
		return validateCoreStringVariable(
			ctx, varName, varSchema, params, validateRuntimeParams,
		)
	}

	if varSchema.Type == schema.VariableTypeInteger {
		return validateCoreIntegerVariable(
			ctx, varName, varSchema, params, validateRuntimeParams,
		)
	}

	if varSchema.Type == schema.VariableTypeFloat {
		return validateCoreFloatVariable(
			ctx, varName, varSchema, params, validateRuntimeParams,
		)
	}

	if varSchema.Type == schema.VariableTypeBoolean {
		return validateCoreBooleanVariable(
			ctx, varName, varSchema, params, validateRuntimeParams,
		)
	}

	return nil
}

func validateCoreStringVariable(
	ctx context.Context,
	varName string,
	varSchema *schema.Variable,
	params bpcore.BlueprintParams,
	validateRuntimeParams bool,
) error {
	if len(varSchema.AllowedValues) > 0 {
		err := validateCoreStringVariableAllowedValues(
			ctx, varName, varSchema, params,
		)

		if err != nil {
			return err
		}
	}

	// Catch default value issues initially, regardless of whether
	// or not the default value will be used in the variable instance.
	if varSchema.Default != nil && varSchema.Default.StringValue == nil {
		return errVariableInvalidDefaultValue(
			schema.VariableTypeString,
			varName,
			varSchema.Default,
		)
	}

	if varSchema.Default != nil && strings.TrimSpace(*varSchema.Default.StringValue) == "" {
		return errVariableEmptyDefaultValue(
			schema.VariableTypeString,
			varName,
			varSchema,
		)
	}

	userProvidedValue := params.BlueprintVariable(varName)
	finalValue := fallbackToDefault(userProvidedValue, varSchema.Default)

	if validateRuntimeParams && finalValue == nil {
		return errRequiredVariableMissing(varName, varSchema)
	}

	if validateRuntimeParams && finalValue.StringValue == nil {
		return errVariableInvalidOrMissing(
			schema.VariableTypeString,
			varName,
			finalValue,
			varSchema,
		)
	}

	if validateRuntimeParams && strings.TrimSpace(*finalValue.StringValue) == "" {
		return errVariableEmptyValue(
			schema.VariableTypeString,
			varName,
			varSchema,
		)
	}

	return validateValueInAllowedList(
		varSchema,
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
	params bpcore.BlueprintParams,
) error {
	// Collect all invalid allowed values in one go to help
	// speed up the debugging process.
	invalidAllowedValueErrors := []error{}
	for _, allowedValue := range varSchema.AllowedValues {
		if allowedValue == nil || allowedValue.StringValue == nil {
			err := errVariableInvalidAllowedValue(
				schema.VariableTypeString,
				allowedValue,
				varSchema,
			)
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
	params bpcore.BlueprintParams,
	validateRuntimeParams bool,
) error {
	if len(varSchema.AllowedValues) > 0 {
		err := validateCoreIntegerVariableAllowedValues(
			ctx, varName, varSchema, params,
		)

		if err != nil {
			return err
		}
	}

	// Catch default value issues initially, regardless of whether
	// or not the default value will be used in the variable instance.
	if varSchema.Default != nil && varSchema.Default.IntValue == nil {
		return errVariableInvalidDefaultValue(
			schema.VariableTypeInteger,
			varName,
			varSchema.Default,
		)
	}

	// No need for explicit empty value checks for an integer as the default empty value
	// for an integer in go (0) is a valid value.

	userProvidedValue := params.BlueprintVariable(varName)
	finalValue := fallbackToDefault(userProvidedValue, varSchema.Default)

	if validateRuntimeParams && finalValue == nil {
		return errRequiredVariableMissing(varName, varSchema)
	}

	if validateRuntimeParams && finalValue.IntValue == nil {
		return errVariableInvalidOrMissing(
			schema.VariableTypeInteger,
			varName,
			finalValue,
			varSchema,
		)
	}

	return validateValueInAllowedList(
		varSchema,
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
	params bpcore.BlueprintParams,
) error {
	// Collect all invalid allowed values in one go to help
	// speed up the debugging process.
	invalidAllowedValueErrors := []error{}
	for _, allowedValue := range varSchema.AllowedValues {
		if allowedValue.IntValue == nil {
			err := errVariableInvalidAllowedValue(
				schema.VariableTypeInteger,
				allowedValue,
				varSchema,
			)
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
	params bpcore.BlueprintParams,
	validateRuntimeParams bool,
) error {
	if len(varSchema.AllowedValues) > 0 {
		err := validateCoreFloatVariableAllowedValues(
			ctx, varName, varSchema, params,
		)

		if err != nil {
			return err
		}
	}

	// Catch default value issues initially, regardless of whether
	// or not the default value will be used in the variable instance.
	if varSchema.Default != nil && varSchema.Default.FloatValue == nil {
		return errVariableInvalidDefaultValue(
			schema.VariableTypeFloat,
			varName,
			varSchema.Default,
		)
	}

	// No need for explicit empty value checks for a float as the default empty value
	// for a floating point number in go (0.0) is a valid value.

	userProvidedValue := params.BlueprintVariable(varName)
	finalValue := fallbackToDefault(userProvidedValue, varSchema.Default)

	if validateRuntimeParams && finalValue == nil {
		return errRequiredVariableMissing(varName, varSchema)
	}

	if validateRuntimeParams && finalValue.FloatValue == nil {
		return errVariableInvalidOrMissing(
			schema.VariableTypeFloat,
			varName,
			finalValue,
			varSchema,
		)
	}

	return validateValueInAllowedList(
		varSchema,
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
	params bpcore.BlueprintParams,
) error {
	// Collect all invalid allowed values in one go to help
	// speed up the debugging process.
	invalidAllowedValueErrors := []error{}
	for _, allowedValue := range varSchema.AllowedValues {
		if allowedValue.FloatValue == nil {
			err := errVariableInvalidAllowedValue(
				schema.VariableTypeFloat,
				allowedValue,
				varSchema,
			)
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
	params bpcore.BlueprintParams,
	validateRuntimeParams bool,
) error {
	if len(varSchema.AllowedValues) > 0 {
		return errVariableInvalidAllowedValuesNotSupported(
			schema.VariableTypeBoolean,
			varName,
			varSchema,
		)
	}

	// Catch default value issues initially, regardless of whether
	// or not the default value will be used in the variable instance.
	if varSchema.Default != nil && varSchema.Default.BoolValue == nil {
		return errVariableInvalidDefaultValue(
			schema.VariableTypeBoolean,
			varName,
			varSchema.Default,
		)
	}

	// No need for explicit empty value checks for a boolean as the default empty value
	// for a boolean in go (false) is a valid value.

	value := params.BlueprintVariable(varName)
	if value == nil {
		value = varSchema.Default
	}

	if validateRuntimeParams && value == nil {
		return errRequiredVariableMissing(varName, varSchema)
	}

	if validateRuntimeParams && value.BoolValue == nil {
		return errVariableInvalidOrMissing(
			schema.VariableTypeBoolean,
			varName,
			value,
			varSchema,
		)
	}

	return nil
}

func validateValueInAllowedList(
	varSchema *schema.Variable,
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
			varSchema,
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
