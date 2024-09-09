package validation

import (
	"context"
	"fmt"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
)

// ValidateResourceName checks the validity of a resource name,
// primarily making sure that it does not contain any substitutions
// as per the spec.
func ValidateResourceName(mappingName string, resourceMap *schema.ResourceMap) error {
	if substitutions.ContainsSubstitution(mappingName) {
		return errMappingNameContainsSubstitution(
			mappingName,
			"resource",
			ErrorReasonCodeInvalidResource,
			getResourceSourceMeta(resourceMap, mappingName),
		)
	}
	return nil
}

// PreValidateResourceSpec pre-validates the resource specification against the blueprint
// specification. This primarily searches for invalid usage of substitutions in mapping keys.
// The main resource validation that invokes a user-provided resource implementation
// comes after this.
func PreValidateResourceSpec(
	ctx context.Context,
	resourceName string,
	resourceSchema *schema.Resource,
	resourceMap *schema.ResourceMap,
) error {
	if resourceSchema.Spec == nil {
		return nil
	}

	errors := preValidateMappingNode(ctx, resourceSchema.Spec, "resource", resourceName)
	if len(errors) > 0 {
		return errResourceSpecPreValidationFailed(
			errors,
			resourceName,
			getResourceSourceMeta(resourceMap, resourceName),
		)
	}

	return nil
}

// ValidateResource ensures that a given resource is valid as per the blueprint specification
// and the resource type specification definition exposed by the resource type provider.
func ValidateResource(
	ctx context.Context,
	name string,
	resource *schema.Resource,
	resourceMap *schema.ResourceMap,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}

	var errs []error

	validateTypeDiagnostics, validateTypeErr := validateResourceType(
		ctx,
		name,
		resource.Type,
		resourceMap,
		resourceRegistry,
	)
	diagnostics = append(diagnostics, validateTypeDiagnostics...)
	if validateTypeErr != nil {
		errs = append(errs, validateTypeErr)
	}

	validateMetadataDiagnostics, validateMetadataErr := validateResourceMetadata(
		ctx,
		name,
		resource.Metadata,
		bpSchema,
		params,
		funcRegistry,
		refChainCollector,
		resourceRegistry,
	)
	diagnostics = append(diagnostics, validateMetadataDiagnostics...)
	if validateMetadataErr != nil {
		errs = append(errs, validateMetadataErr)
	}

	validateResConditionDiagnostics, validateResConditionErr := validateResourceCondition(
		ctx,
		name,
		resource.Condition,
		bpSchema,
		params,
		funcRegistry,
		refChainCollector,
		resourceRegistry,
		/* depth */ 0,
	)
	diagnostics = append(diagnostics, validateResConditionDiagnostics...)
	if validateResConditionErr != nil {
		errs = append(errs, validateResConditionErr)
	}

	validateEachDiagnostics, validateEachErr := validateResourceEach(
		ctx,
		name,
		resource.Each,
		bpSchema,
		params,
		funcRegistry,
		refChainCollector,
		resourceRegistry,
	)
	diagnostics = append(diagnostics, validateEachDiagnostics...)
	if validateEachErr != nil {
		errs = append(errs, validateEachErr)
	}

	validateLSDiagnostics, validateLSErr := validateResourceLinkSelector(
		name,
		resource.LinkSelector,
	)
	diagnostics = append(diagnostics, validateLSDiagnostics...)
	if validateLSErr != nil {
		errs = append(errs, validateLSErr)
	}

	validateSpecDiagnostics, validateSpecErr := ValidateResourceSpec(
		ctx,
		name,
		resource.Type,
		resource,
		resourceMap.SourceMeta[name],
		bpSchema,
		params,
		funcRegistry,
		refChainCollector,
		resourceRegistry,
	)
	diagnostics = append(diagnostics, validateSpecDiagnostics...)
	if validateSpecErr != nil {
		errs = append(errs, validateSpecErr)
	}

	validateDescriptionDiagnostics, validateDescErr := validateDescription(
		ctx,
		fmt.Sprintf("resources.%s", name),
		resource.Description,
		bpSchema,
		params,
		funcRegistry,
		refChainCollector,
		resourceRegistry,
	)
	diagnostics = append(diagnostics, validateDescriptionDiagnostics...)
	if validateDescErr != nil {
		errs = append(errs, validateDescErr)
	}

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	return diagnostics, nil
}

func validateResourceType(
	ctx context.Context,
	resourceName string,
	resourceType string,
	resourceMap *schema.ResourceMap,
	resourceRegistry provider.ResourceRegistry,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}

	hasType, err := resourceRegistry.HasResourceType(ctx, resourceType)
	if err != nil {
		return diagnostics, err
	}

	if !hasType {
		return diagnostics, errResourceTypeNotSupported(
			resourceName,
			resourceType,
			getResourceSourceMeta(resourceMap, resourceName),
		)
	}

	return diagnostics, nil
}

func validateResourceMetadata(
	ctx context.Context,
	resourceName string,
	metadataSchema *schema.Metadata,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}

	if metadataSchema == nil {
		return diagnostics, nil
	}

	var errs []error

	displayNameDiagnostics, err := validateResourceMetadataDisplayName(
		ctx,
		resourceName,
		metadataSchema,
		bpSchema,
		params,
		funcRegistry,
		refChainCollector,
		resourceRegistry,
	)
	diagnostics = append(diagnostics, displayNameDiagnostics...)
	if err != nil {
		errs = append(errs, err)
	}

	labelDiagnostics, err := validateResourceMetadataLabels(
		resourceName,
		metadataSchema,
	)
	diagnostics = append(diagnostics, labelDiagnostics...)
	if err != nil {
		errs = append(errs, err)
	}

	annotationsDiagnostics, err := validateResourceMetadataAnnotations(
		ctx,
		resourceName,
		metadataSchema,
		bpSchema,
		params,
		funcRegistry,
		refChainCollector,
		resourceRegistry,
	)
	diagnostics = append(diagnostics, annotationsDiagnostics...)
	if err != nil {
		errs = append(errs, err)
	}

	customDiagnostics, err := ValidateMappingNode(
		ctx,
		fmt.Sprintf("resources.%s", resourceName),
		"metadata.custom",
		metadataSchema.Custom,
		bpSchema,
		params,
		funcRegistry,
		refChainCollector,
		resourceRegistry,
	)
	diagnostics = append(diagnostics, customDiagnostics...)
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	return diagnostics, nil
}

func validateResourceMetadataDisplayName(
	ctx context.Context,
	resourceName string,
	metadataSchema *schema.Metadata,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
) ([]*bpcore.Diagnostic, error) {
	if metadataSchema.DisplayName == nil {
		return []*bpcore.Diagnostic{}, nil
	}

	resourceIdentifier := fmt.Sprintf("resources.%s", resourceName)
	errs := []error{}
	diagnostics := []*bpcore.Diagnostic{}
	for _, stringOrSub := range metadataSchema.DisplayName.Values {
		if stringOrSub.SubstitutionValue != nil {
			resolvedType, subDiagnostics, err := ValidateSubstitution(
				ctx,
				stringOrSub.SubstitutionValue,
				nil,
				bpSchema,
				resourceIdentifier,
				params,
				funcRegistry,
				refChainCollector,
				resourceRegistry,
			)
			if err != nil {
				errs = append(errs, err)
			} else {
				diagnostics = append(diagnostics, subDiagnostics...)
				if resolvedType != string(substitutions.ResolvedSubExprTypeString) {
					errs = append(errs, errInvalidDisplayNameSubType(
						resourceIdentifier,
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

func validateResourceMetadataLabels(
	resourceName string,
	metadataSchema *schema.Metadata,
) ([]*bpcore.Diagnostic, error) {
	if metadataSchema.Labels == nil {
		return []*bpcore.Diagnostic{}, nil
	}

	errs := []error{}
	diagnostics := []*bpcore.Diagnostic{}
	for key, value := range metadataSchema.Labels.Values {
		if substitutions.ContainsSubstitution(key) {
			errs = append(errs, errLabelKeyContainsSubstitution(
				resourceName,
				key,
				metadataSchema.Labels.SourceMeta[key],
			))
		}

		if substitutions.ContainsSubstitution(value) {
			errs = append(errs, errLabelValueContainsSubstitution(
				resourceName,
				key,
				value,
				metadataSchema.Labels.SourceMeta[key],
			))
		}
	}

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	return diagnostics, nil
}

func validateResourceMetadataAnnotations(
	ctx context.Context,
	resourceName string,
	metadataSchema *schema.Metadata,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
) ([]*bpcore.Diagnostic, error) {
	if metadataSchema.Annotations == nil || metadataSchema.Annotations.Values == nil {
		return []*bpcore.Diagnostic{}, nil
	}

	resourceIdentifier := fmt.Sprintf("resources.%s", resourceName)
	errs := []error{}
	diagnostics := []*bpcore.Diagnostic{}
	for key, annotation := range metadataSchema.Annotations.Values {
		if substitutions.ContainsSubstitution(key) {
			errs = append(errs, errAnnotationKeyContainsSubstitution(
				resourceName,
				key,
				annotation.SourceMeta,
			))
		}

		annotationDiagnostics, err := validateMetadataAnnotation(
			ctx,
			resourceIdentifier,
			annotation,
			bpSchema,
			params,
			funcRegistry,
			refChainCollector,
			resourceRegistry,
		)
		diagnostics = append(diagnostics, annotationDiagnostics...)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	return diagnostics, nil
}

func validateResourceCondition(
	ctx context.Context,
	resourceName string,
	conditionSchema *schema.Condition,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
	depth int,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}

	if conditionSchema == nil && depth == 0 {
		return diagnostics, nil
	}

	if (conditionSchema == nil || allConditionValuesNil(conditionSchema)) && depth > 0 {
		// Nested conditions should not be empty.
		return diagnostics, errNestedResourceConditionEmpty(
			resourceName,
			conditionSchema.SourceMeta,
		)
	}

	var errs []error
	if conditionSchema.And != nil {
		for _, andCondition := range conditionSchema.And {
			andDiagnostics, err := validateResourceCondition(
				ctx,
				resourceName,
				andCondition,
				bpSchema,
				params,
				funcRegistry,
				refChainCollector,
				resourceRegistry,
				depth+1,
			)
			diagnostics = append(diagnostics, andDiagnostics...)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	if conditionSchema.Or != nil {
		for _, orCondition := range conditionSchema.Or {
			orDiagnostics, err := validateResourceCondition(
				ctx,
				resourceName,
				orCondition,
				bpSchema,
				params,
				funcRegistry,
				refChainCollector,
				resourceRegistry,
				depth+1,
			)
			diagnostics = append(diagnostics, orDiagnostics...)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	if conditionSchema.Not != nil {
		notDiagnostics, err := validateResourceCondition(
			ctx,
			resourceName,
			conditionSchema.Not,
			bpSchema,
			params,
			funcRegistry,
			refChainCollector,
			resourceRegistry,
			depth+1,
		)
		diagnostics = append(diagnostics, notDiagnostics...)
		if err != nil {
			return diagnostics, err
		}
	}

	conditionValDiagnostics, err := validateConditionValue(
		ctx,
		resourceName,
		conditionSchema.StringValue,
		bpSchema,
		params,
		funcRegistry,
		refChainCollector,
		resourceRegistry,
	)
	diagnostics = append(diagnostics, conditionValDiagnostics...)
	if err != nil {
		return diagnostics, err
	}

	return diagnostics, nil
}

func validateConditionValue(
	ctx context.Context,
	resourceName string,
	conditionValue *substitutions.StringOrSubstitutions,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
) ([]*bpcore.Diagnostic, error) {
	if conditionValue == nil {
		return []*bpcore.Diagnostic{}, nil
	}

	errs := []error{}
	diagnostics := []*bpcore.Diagnostic{}

	resourceIdentifier := fmt.Sprintf("resources.%s", resourceName)

	if len(conditionValue.Values) > 1 {
		return diagnostics, errInvalidSubTypeNotBoolean(
			resourceIdentifier,
			"condition",
			// StringOrSubstitutions with multiple values is an
			// interpolated string.
			string(substitutions.ResolvedSubExprTypeString),
			conditionValue.SourceMeta,
		)
	}

	for i, stringOrSub := range conditionValue.Values {
		nextLocation := getSubNextLocation(i, conditionValue.Values)

		if stringOrSub.SubstitutionValue != nil {
			resolvedType, subDiagnostics, err := ValidateSubstitution(
				ctx,
				stringOrSub.SubstitutionValue,
				nil,
				bpSchema,
				resourceIdentifier,
				params,
				funcRegistry,
				refChainCollector,
				resourceRegistry,
			)
			if err != nil {
				errs = append(errs, err)
			} else {
				diagnostics = append(diagnostics, subDiagnostics...)
				handleResolvedTypeExpectingBoolean(
					resolvedType,
					resourceIdentifier,
					stringOrSub,
					conditionValue,
					"condition",
					nextLocation,
					&diagnostics,
					&errs,
				)
			}
		}
	}

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	return diagnostics, nil
}

func handleResolvedTypeExpectingBoolean(
	resolvedType string,
	itemIdentifier string,
	stringOrSub *substitutions.StringOrSubstitution,
	value *substitutions.StringOrSubstitutions,
	valueContext string,
	nextLocation *source.Meta,
	diagnostics *[]*bpcore.Diagnostic,
	errs *[]error,
) {
	if resolvedType != string(substitutions.ResolvedSubExprTypeBoolean) &&
		resolvedType != string(substitutions.ResolvedSubExprTypeAny) {
		*errs = append(*errs, errInvalidSubTypeNotBoolean(
			itemIdentifier,
			valueContext,
			resolvedType,
			stringOrSub.SourceMeta,
		))
	} else if resolvedType == string(substitutions.ResolvedSubExprTypeAny) {
		// Any type will produce a warning diagnostic as any could match a boolean
		// value in a context where the developer is confident a boolean value will
		// be resolved.
		*diagnostics = append(
			*diagnostics,
			&bpcore.Diagnostic{
				Level: bpcore.DiagnosticLevelWarning,
				Message: fmt.Sprintf(
					"Substitution returns \"any\" type, this may produce "+
						"unexpected output in the %s, %ss are expected to be boolean values",
					valueContext,
					valueContext,
				),
				Range: toDiagnosticRange(value.SourceMeta, nextLocation),
			},
		)
	}
}

func validateResourceEach(
	ctx context.Context,
	resourceName string,
	each *substitutions.StringOrSubstitutions,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
) ([]*bpcore.Diagnostic, error) {
	if each == nil {
		return []*bpcore.Diagnostic{}, nil
	}

	errs := []error{}
	diagnostics := []*bpcore.Diagnostic{}

	resourceIdentifier := fmt.Sprintf("resources.%s", resourceName)

	if len(each.Values) > 1 {
		return diagnostics, errInvalidSubTypeNotArray(
			resourceIdentifier,
			"each",
			// StringOrSubstitutions with multiple values is an
			// interpolated string.
			string(substitutions.ResolvedSubExprTypeString),
			each.SourceMeta,
		)
	}

	for i, stringOrSub := range each.Values {
		nextLocation := getSubNextLocation(i, each.Values)

		if stringOrSub.SubstitutionValue != nil {
			resolvedType, subDiagnostics, err := ValidateSubstitution(
				ctx,
				stringOrSub.SubstitutionValue,
				nil,
				bpSchema,
				resourceIdentifier,
				params,
				funcRegistry,
				refChainCollector,
				resourceRegistry,
			)
			if err != nil {
				errs = append(errs, err)
			} else {
				diagnostics = append(diagnostics, subDiagnostics...)
				handleResolvedTypeExpectingArray(
					resolvedType,
					resourceIdentifier,
					stringOrSub,
					each,
					"each",
					nextLocation,
					&diagnostics,
					&errs,
				)
			}
		}
	}

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	return diagnostics, nil
}

func handleResolvedTypeExpectingArray(
	resolvedType string,
	itemIdentifier string,
	stringOrSub *substitutions.StringOrSubstitution,
	value *substitutions.StringOrSubstitutions,
	valueContext string,
	nextLocation *source.Meta,
	diagnostics *[]*bpcore.Diagnostic,
	errs *[]error,
) {
	if resolvedType != string(substitutions.ResolvedSubExprTypeArray) &&
		resolvedType != string(substitutions.ResolvedSubExprTypeAny) {
		*errs = append(*errs, errInvalidSubTypeNotArray(
			itemIdentifier,
			valueContext,
			resolvedType,
			stringOrSub.SourceMeta,
		))
	} else if resolvedType == string(substitutions.ResolvedSubExprTypeAny) {
		// Any type will produce a warning diagnostic as any could match an array,
		// an error will occur at runtime if the resolved value is not an array.
		*diagnostics = append(
			*diagnostics,
			&bpcore.Diagnostic{
				Level: bpcore.DiagnosticLevelWarning,
				Message: fmt.Sprintf(
					"Substitution returns \"any\" type, this may produce "+
						"unexpected output in %s, an array is expected",
					valueContext,
				),
				Range: toDiagnosticRange(value.SourceMeta, nextLocation),
			},
		)
	}
}

func validateResourceLinkSelector(
	resourceName string,
	linkSelector *schema.LinkSelector,
) ([]*bpcore.Diagnostic, error) {
	if linkSelector == nil || linkSelector.ByLabel == nil {
		return []*bpcore.Diagnostic{}, nil
	}

	errs := []error{}
	diagnostics := []*bpcore.Diagnostic{}
	for key, value := range linkSelector.ByLabel.Values {
		if substitutions.ContainsSubstitution(key) {
			errs = append(errs, errLinkSelectorKeyContainsSubstitution(
				resourceName,
				key,
				linkSelector.ByLabel.SourceMeta[key],
			))
		}

		if substitutions.ContainsSubstitution(value) {
			errs = append(errs, errLinkSelectorValueContainsSubstitution(
				resourceName,
				key,
				value,
				linkSelector.ByLabel.SourceMeta[key],
			))
		}
	}

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	return diagnostics, nil
}

func allConditionValuesNil(condition *schema.Condition) bool {
	return condition.And == nil && condition.Or == nil &&
		condition.Not == nil && condition.StringValue == nil
}

func getResourceSourceMeta(resourceMap *schema.ResourceMap, resourceName string) *source.Meta {
	if resourceMap == nil {
		return nil
	}

	return resourceMap.SourceMeta[resourceName]
}
