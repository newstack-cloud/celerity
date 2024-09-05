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

// ValidateDataSource ensures that a given data source matches the specification
// for all cases not handled during schema parsing.
func ValidateDataSource(
	ctx context.Context,
	name string,
	dataSource *schema.DataSource,
	dataSourceMap *schema.DataSourceMap,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}

	if dataSource.Filter == nil {
		return diagnostics, errDataSourceMissingFilter(
			name, getDataSourceMeta(dataSourceMap, name),
		)
	}

	if dataSource.Filter.Field == "" {
		return diagnostics, errDataSourceMissingFilterField(
			name, getDataSourceMeta(dataSourceMap, name),
		)
	}

	if dataSource.Filter.Search == nil || len(dataSource.Filter.Search.Values) == 0 {
		return diagnostics, errDataSourceMissingFilterSearch(
			name, getDataSourceMeta(dataSourceMap, name),
		)
	}

	if dataSource.Exports == nil || len(dataSource.Exports.Values) == 0 {
		return diagnostics, errDataSourceMissingExports(
			name, getDataSourceMeta(dataSourceMap, name),
		)
	}

	var errs []error

	validateDiagnostics, validateErr := validateDataSourceMetadata(
		ctx,
		name,
		dataSource.DataSourceMetadata,
		bpSchema,
		params,
		funcRegistry,
		refChainCollector,
		resourceRegistry,
	)
	diagnostics = append(diagnostics, validateDiagnostics...)
	if validateErr != nil {
		errs = append(errs, validateErr)
	}

	// todo: validate description
	// todo: validate filter
	// todo: validate exports

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	return diagnostics, nil
}

func validateDataSourceMetadata(
	ctx context.Context,
	dataSourceName string,
	metadataSchema *schema.DataSourceMetadata,
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

	displayNameDiagnostics, err := validateDataSourceMetadataDisplayName(
		ctx,
		dataSourceName,
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

	annotationsDiagnostics, err := validateDataSourceMetadataAnnotations(
		ctx,
		dataSourceName,
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
		fmt.Sprintf("datasources.%s", dataSourceName),
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

func validateDataSourceMetadataDisplayName(
	ctx context.Context,
	dataSourceName string,
	metadataSchema *schema.DataSourceMetadata,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
) ([]*bpcore.Diagnostic, error) {
	if metadataSchema.DisplayName == nil {
		return []*bpcore.Diagnostic{}, nil
	}

	dataSourceIdentifier := fmt.Sprintf("datasources.%s", dataSourceName)
	errs := []error{}
	diagnostics := []*bpcore.Diagnostic{}
	for _, stringOrSub := range metadataSchema.DisplayName.Values {
		if stringOrSub.SubstitutionValue != nil {
			resolvedType, subDiagnostics, err := ValidateSubstitution(
				ctx,
				stringOrSub.SubstitutionValue,
				nil,
				bpSchema,
				dataSourceIdentifier,
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
						dataSourceIdentifier,
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

func validateDataSourceMetadataAnnotations(
	ctx context.Context,
	dataSourceName string,
	metadataSchema *schema.DataSourceMetadata,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
) ([]*bpcore.Diagnostic, error) {
	if metadataSchema.Annotations == nil || metadataSchema.Annotations.Values == nil {
		return []*bpcore.Diagnostic{}, nil
	}

	dataSourceIdentifier := fmt.Sprintf("datasources.%s", dataSourceName)
	errs := []error{}
	diagnostics := []*bpcore.Diagnostic{}
	for _, annotation := range metadataSchema.Annotations.Values {
		annotationDiagnsoitcs, err := validateDataSourceMetadataAnnotation(
			ctx,
			dataSourceIdentifier,
			annotation,
			bpSchema,
			params,
			funcRegistry,
			refChainCollector,
			resourceRegistry,
		)
		diagnostics = append(diagnostics, annotationDiagnsoitcs...)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	return diagnostics, nil
}

func validateDataSourceMetadataAnnotation(
	ctx context.Context,
	dataSourceIdentifier string,
	annotation *substitutions.StringOrSubstitutions,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
) ([]*bpcore.Diagnostic, error) {
	if annotation == nil {
		return []*bpcore.Diagnostic{}, nil
	}

	errs := []error{}
	diagnostics := []*bpcore.Diagnostic{}
	for i, stringOrSub := range annotation.Values {
		nextLocation := getSubNextLocation(i, annotation.Values)

		if stringOrSub.SubstitutionValue != nil {
			resolvedType, subDiagnostics, err := ValidateSubstitution(
				ctx,
				stringOrSub.SubstitutionValue,
				nil,
				bpSchema,
				dataSourceIdentifier,
				params,
				funcRegistry,
				refChainCollector,
				resourceRegistry,
			)
			if err != nil {
				errs = append(errs, err)
			} else {
				diagnostics = append(diagnostics, subDiagnostics...)
				handleAnnotationResolvedType(
					resolvedType,
					dataSourceIdentifier,
					stringOrSub,
					annotation,
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

func handleAnnotationResolvedType(
	resolvedType string,
	dataSourceIdentifier string,
	stringOrSub *substitutions.StringOrSubstitution,
	annotation *substitutions.StringOrSubstitutions,
	nextLocation *source.Meta,
	diagnostics *[]*bpcore.Diagnostic,
	errs *[]error,
) {
	if isSubPrimitiveType(resolvedType) {
		*errs = append(*errs, errInvalidAnnotationSubType(
			dataSourceIdentifier,
			resolvedType,
			stringOrSub.SourceMeta,
		))
	} else if resolvedType != string(substitutions.ResolvedSubExprTypeAny) {
		// Any type will produce a warning diagnostic as any is likely to match
		// and will be stringified in the final annotation output, which is undesired
		// but not undefined behaviour.
		*diagnostics = append(
			*diagnostics,
			&bpcore.Diagnostic{
				Level: bpcore.DiagnosticLevelWarning,
				Message: fmt.Sprintf(
					"Substitution returns \"any\" type, this may produce " +
						"unexpected output in the annotation value, annotations are expected to be scalar values",
				),
				Range: toDiagnosticRange(annotation.SourceMeta, nextLocation),
			},
		)
	}
}

func getDataSourceMeta(varMap *schema.DataSourceMap, varName string) *source.Meta {
	if varMap == nil {
		return nil
	}

	return varMap.SourceMeta[varName]
}
