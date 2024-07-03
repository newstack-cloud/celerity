package validation

import (
	"context"
	"strings"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/provider"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	"github.com/two-hundred/celerity/libs/common/pkg/core"
)

// ValidateCustomVariable validates a custom variable in a blueprint.
// This validation spans all the fields of a variable in the parsed schema
// as well as the runtime variable value provided by the user.
func ValidateCustomVariable(
	ctx context.Context,
	varName string,
	varSchema *schema.Variable,
	params bpcore.BlueprintParams,
	customVariableType provider.CustomVariableType,
) error {
	optionLabels, err := validateCustomVariableOptions(ctx, varName, varSchema, params, customVariableType)
	if err != nil {
		return err
	}

	// Values for custom variables must be the string labels for the options
	// provided by the custom type.
	if varSchema.Default != nil && varSchema.Default.StringValue == nil {
		return errCustomVariableInvalidDefaultValueType(
			varSchema.Type,
			varName,
			varSchema.Default,
		)
	}

	if varSchema.Default != nil && strings.TrimSpace(*varSchema.Default.StringValue) == "" {
		return errVariableEmptyDefaultValue(
			varSchema.Type,
			varName,
			varSchema,
		)
	}

	if varSchema.Default != nil && !core.SliceContainsComparable(optionLabels, *varSchema.Default.StringValue) {
		return errCustomVariableDefaultValueNotInOptions(
			varSchema.Type,
			varName,
			*varSchema.Default.StringValue,
		)
	}

	userProvidedValue := params.BlueprintVariable(varName)
	finalValue := fallbackToDefault(userProvidedValue, varSchema.Default)

	if finalValue == nil {
		return errRequiredVariableMissing(varName, varSchema)
	}

	if finalValue.StringValue == nil {
		return errVariableInvalidOrMissing(
			varSchema.Type,
			varName,
			finalValue,
			varSchema,
		)
	}

	if strings.TrimSpace(*finalValue.StringValue) == "" {
		return errVariableEmptyValue(
			varSchema.Type,
			varName,
		)
	}

	if !core.SliceContainsComparable(optionLabels, *finalValue.StringValue) {
		usingDefault := userProvidedValue == nil
		return errCustomVariableValueNotInOptions(
			varSchema.Type,
			varName,
			finalValue,
			usingDefault,
		)
	}

	if len(varSchema.AllowedValues) > 0 && !bpcore.IsInScalarList(finalValue, varSchema.AllowedValues) {
		usingDefault := userProvidedValue == nil
		return errVariableValueNotAllowed(
			varSchema.Type,
			varName,
			finalValue,
			varSchema.AllowedValues,
			usingDefault,
		)
	}

	return nil
}

func validateCustomVariableOptions(
	ctx context.Context,
	varName string,
	varSchema *schema.Variable,
	params bpcore.BlueprintParams,
	customVariableType provider.CustomVariableType,
) ([]string, error) {
	options, err := customVariableType.Options(ctx, params)
	if err != nil {
		return nil, errCustomVariableOptions(
			varName,
			varSchema,
			err,
		)
	}

	optionsSlice := optionsMapToSlice(options)
	if hasMixedTypes(optionsSlice) {
		return nil, errCustomVariableMixedTypes(
			varName,
			varSchema,
		)
	}

	optionLabels := keysToSlice(options)
	if len(varSchema.AllowedValues) > 0 {
		err := validateCustomVariableAllowedValues(
			ctx, varName, varSchema, params, optionLabels,
		)

		if err != nil {
			return nil, err
		}
	}

	return optionLabels, nil
}

func validateCustomVariableAllowedValues(
	ctx context.Context,
	varName string,
	varSchema *schema.Variable,
	params bpcore.BlueprintParams,
	optionLabels []string,
) error {
	// Collect all invalid allowed values in one go to help
	// speed up the debugging process.
	invalidAllowedValueErrors := []error{}
	for _, allowedValue := range varSchema.AllowedValues {
		if allowedValue.StringValue == nil {
			err := errVariableInvalidAllowedValue(
				varSchema.Type,
				allowedValue,
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

	invalidOptions := getInvalidOptions(varSchema.AllowedValues, optionLabels)
	if len(invalidOptions) > 0 {
		return errCustomVariableAllowedValuesNotInOptions(
			varSchema.Type,
			varName,
			invalidOptions,
		)
	}

	return nil
}

func getInvalidOptions(values []*bpcore.ScalarValue, optionLabels []string) []string {
	// Is is more important to reveal all invalid options as soon as possible to users
	// than to be efficient here, hence why we don't short circuit.
	invalidOptions := []string{}
	for _, value := range values {
		// Values with invalid types should have been caught before this point.
		if value.StringValue != nil && !core.SliceContainsComparable(optionLabels, *value.StringValue) {
			invalidOptions = append(invalidOptions, *value.StringValue)
		}
	}
	return invalidOptions
}

func hasMixedTypes(options []*bpcore.ScalarValue) bool {
	if len(options) == 0 {
		return false
	}

	currentType := (*schema.VariableType)(nil)
	hasMoreThanOneType := false
	i := 0
	for !hasMoreThanOneType && i < len(options) {
		varType := deriveVarType(options[i])
		if currentType != nil {
			if varType != *currentType {
				hasMoreThanOneType = true
			}
		} else {
			currentType = &varType
		}
		i += 1
	}

	return hasMoreThanOneType
}

func optionsMapToSlice(options map[string]*bpcore.ScalarValue) []*bpcore.ScalarValue {
	result := make([]*bpcore.ScalarValue, 0, len(options))
	for _, option := range options {
		result = append(result, option)
	}
	return result
}

// This could be useful as a general utility,
// might be worth moving out into the common package at some point.
func keysToSlice[Value any](mapping map[string]Value) []string {
	keys := make([]string, 0, len(mapping))
	for key := range mapping {
		keys = append(keys, key)
	}
	return keys
}
