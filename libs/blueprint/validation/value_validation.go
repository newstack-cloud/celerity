package validation

import (
	"context"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/resourcehelpers"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	"github.com/two-hundred/celerity/libs/common/core"
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
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}

	valueTypeDiagnostics, err := validateValueType(
		valName,
		valSchema,
	)
	diagnostics = append(diagnostics, valueTypeDiagnostics...)
	if err != nil {
		return diagnostics, err
	}

	expectedResolveType := subValType(valSchema.Type)

	return validateValue(
		ctx,
		valName,
		valSchema,
		expectedResolveType,
		bpSchema,
		params,
		funcRegistry,
		refChainCollector,
		resourceRegistry,
	)
}

func validateValue(
	ctx context.Context,
	valName string,
	valSchema *schema.Value,
	expectedResolveType string,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}

	descriptionDiagnostics, err := validateValueDescription(
		ctx,
		valName,
		valSchema,
		bpSchema,
		params,
		funcRegistry,
		refChainCollector,
		resourceRegistry,
	)
	diagnostics = append(diagnostics, descriptionDiagnostics...)
	if err != nil {
		return diagnostics, err
	}

	valueDiagnostics, err := validateValueContent(
		ctx,
		expectedResolveType,
		valName,
		valSchema,
		bpSchema,
		params,
		funcRegistry,
		refChainCollector,
		resourceRegistry,
	)
	diagnostics = append(diagnostics, valueDiagnostics...)
	if err != nil {
		return valueDiagnostics, err
	}

	return diagnostics, nil
}

func validateValueDescription(
	ctx context.Context,
	valName string,
	valSchema *schema.Value,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
) ([]*bpcore.Diagnostic, error) {
	if valSchema.Description == nil {
		return []*bpcore.Diagnostic{}, nil
	}

	valIdentifier := bpcore.ValueElementID(valName)
	errs := []error{}
	diagnostics := []*bpcore.Diagnostic{}
	for _, stringOrSub := range valSchema.Description.Values {
		if stringOrSub.SubstitutionValue != nil {
			resolvedType, subDiagnostics, err := ValidateSubstitution(
				ctx,
				stringOrSub.SubstitutionValue,
				nil,
				bpSchema,
				/* usedInResourceDerivedFromTemplate */ false,
				valIdentifier,
				"description",
				params,
				funcRegistry,
				refChainCollector,
				resourceRegistry,
			)
			if err != nil {
				errs = append(errs, err)
			} else {
				diagnostics = append(diagnostics, subDiagnostics...)
				if !isSubPrimitiveType(resolvedType) {
					errs = append(errs, errInvalidDescriptionSubType(
						valIdentifier,
						resolvedType,
						stringOrSub.SourceMeta,
					))
				}
			}
		}
	}

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	return diagnostics, nil
}

func validateValueContent(
	ctx context.Context,
	expectedResolveType string,
	valName string,
	valSchema *schema.Value,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
) ([]*bpcore.Diagnostic, error) {
	if valSchema.Value == nil {
		return []*bpcore.Diagnostic{}, errMissingValueContent(valName, valSchema.SourceMeta)
	}

	valIdentifier := bpcore.ValueElementID(valName)
	errs := []error{}
	diagnostics := []*bpcore.Diagnostic{}

	// More than one value in a stringOrSubstitutions slice represents a string interpolation.
	if len(valSchema.Value.Values) > 1 &&
		expectedResolveType != string(substitutions.ResolvedSubExprTypeString) {
		return diagnostics, errValueIncorrectTypeInterpolatedString(
			valIdentifier,
			expectedResolveType,
			valSchema.SourceMeta,
		)
	}

	for _, stringOrSub := range valSchema.Value.Values {
		if stringOrSub.StringValue != nil {
			if expectedResolveType != string(substitutions.ResolvedSubExprTypeString) {
				errs = append(errs, errValueIncorrectTypeInterpolatedString(
					valIdentifier,
					expectedResolveType,
					stringOrSub.SourceMeta,
				))
			}
		}
		if stringOrSub.SubstitutionValue != nil {
			resolvedType, subDiagnostics, err := ValidateSubstitution(
				ctx,
				stringOrSub.SubstitutionValue,
				nil,
				bpSchema,
				/* usedInResourceDerivedFromTemplate */ false,
				valIdentifier,
				"value",
				params,
				funcRegistry,
				refChainCollector,
				resourceRegistry,
			)
			if err != nil {
				errs = append(errs, err)
			} else {
				diagnostics = append(diagnostics, subDiagnostics...)
				if resolvedType != expectedResolveType &&
					// Allow any type to account for functions like jsondecode() that can return any type.
					// This means the user is responsible for ensuring the type of the value is correct.
					resolvedType != string(substitutions.ResolvedSubExprTypeAny) {
					errs = append(errs, errInvalidValueSubType(
						valIdentifier,
						resolvedType,
						expectedResolveType,
						stringOrSub.SourceMeta,
					))
				}
			}
		}
	}

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	return diagnostics, nil
}

func validateValueType(
	valName string,
	valSchema *schema.Value,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}

	if valSchema.Type == nil || valSchema.Type.Value == "" {
		return diagnostics, errMissingValueType(valName, valSchema.SourceMeta)
	}

	if !core.SliceContains(schema.ValueTypes, valSchema.Type.Value) {
		return diagnostics, errInvalidValueType(
			valName,
			valSchema.Type,
			valSchema.SourceMeta,
		)
	}

	return diagnostics, nil
}

func getValSourceMeta(valMap *schema.ValueMap, varName string) *source.Meta {
	if valMap == nil {
		return nil
	}

	return valMap.SourceMeta[varName]
}
