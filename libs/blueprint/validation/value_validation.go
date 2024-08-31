package validation

import (
	"context"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
)

// ValidateValueName checks the validity of a value name,
// primarily making sure that it does not contain any substitutions
// as per the spec.
func ValidateValueName(mappingName string, valMap *schema.ValueMap) error {
	if substitutions.ContainsSubstitution(mappingName) {
		return errMappingNameContainsSubstitution(
			mappingName,
			"value",
			ErrorReasonCodeInvalidValue,
			getValSourceMeta(valMap, mappingName),
		)
	}
	return nil
}

// ValidateValue deals with validating a blueprint value
// against the supported value types in the blueprint
// specification.
func ValidateValue(
	ctx context.Context,
	valName string,
	valSchema *schema.Value,
	valMap *schema.ValueMap,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	if valSchema.Type.Value == schema.ValueTypeString {
		return validateStringValue(
			ctx, valName, valSchema, valMap, bpSchema, params,
		)
	}

	// if valSchema.Type == schema.ValueTypeInteger {
	// 	return validateIntegerValue(
	// 		ctx, varName, valSchema, valMap, params,
	// 	)
	// }

	// if valSchema.Type == schema.ValueTypeFloat {
	// 	return validateFloatValue(
	// 		ctx, varName, valSchema, valMap, params,
	// 	)
	// }

	// if valSchema.Type == schema.ValueTypeBoolean {
	// 	return validateBooleanValue(
	// 		ctx, varName, valSchema, valMap, params,
	// 	)
	// }

	// if valSchema.Type == schema.ValueTypeArray {
	// 	return validateArrayValue(
	// 		ctx, varName, valSchema, valMap, params,
	// 	)
	// }

	// if valSchema.Type == schema.ValueTypeObject {
	// 	return validateObjectValue(
	// 		ctx, varName, valSchema, valMap, params,
	// 	)
	// }

	return diagnostics, nil
}

func validateStringValue(
	ctx context.Context,
	valName string,
	valSchema *schema.Value,
	valMap *schema.ValueMap,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
) ([]*bpcore.Diagnostic, error) {

	return []*bpcore.Diagnostic{}, nil
}

func getValSourceMeta(valMap *schema.ValueMap, varName string) *source.Meta {
	if valMap == nil {
		return nil
	}

	return valMap.SourceMeta[varName]
}
