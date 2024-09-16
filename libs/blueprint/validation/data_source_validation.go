package validation

import (
	"context"
	"fmt"
	"slices"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/resourcehelpers"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	"github.com/two-hundred/celerity/libs/common/core"
)

// ValidateDataSourceName checks the validity of a data source name,
// primarily making sure that it does not contain any substitutions
// as per the spec.
func ValidateDataSourceName(mappingName string, dataSourceMap *schema.DataSourceMap) error {
	if substitutions.ContainsSubstitution(mappingName) {
		return errMappingNameContainsSubstitution(
			mappingName,
			"data source",
			ErrorReasonCodeInvalidResource,
			getDataSourceMeta(dataSourceMap, mappingName),
		)
	}
	return nil
}

// ValidateDataSource ensures that a given data source matches the specification.
func ValidateDataSource(
	ctx context.Context,
	name string,
	dataSource *schema.DataSource,
	dataSourceMap *schema.DataSourceMap,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
	dataSourceRegistry provider.DataSourceRegistry,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}

	var errs []error

	validateTypeDiagnostics, validateTypeErr := validateDataSourceType(
		ctx,
		name,
		dataSource.Type,
		dataSourceMap,
		dataSourceRegistry,
	)
	diagnostics = append(diagnostics, validateTypeDiagnostics...)
	if validateTypeErr != nil {
		errs = append(errs, validateTypeErr)
	}

	validateMetadataDiagnostics, validateMetadataErr := validateDataSourceMetadata(
		ctx,
		name,
		dataSource.DataSourceMetadata,
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

	validateDescriptionDiagnostics, validateDescErr := validateDescription(
		ctx,
		fmt.Sprintf("datasources.%s", name),
		dataSource.Description,
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

	// All validation after this point requires a data source type,
	// if one isn't set, we'll return the errors and diagnostics
	// collected so far.
	if dataSource.Type == nil {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	validateFilterDiagnostics, validateFilterErr := validateDataSourceFilter(
		ctx,
		name,
		dataSource.Type.Value,
		dataSource.Filter,
		dataSourceMap,
		bpSchema,
		params,
		funcRegistry,
		refChainCollector,
		resourceRegistry,
		dataSourceRegistry,
	)
	diagnostics = append(diagnostics, validateFilterDiagnostics...)
	if validateFilterErr != nil {
		errs = append(errs, validateFilterErr)
	}

	specDefinition, specDefErr := loadDataSourceSpecDefinition(
		ctx,
		dataSource.Type.Value,
		name,
		dataSource.SourceMeta,
		params,
		dataSourceRegistry,
	)
	if specDefErr != nil {
		errs = append(errs, specDefErr)
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	validateExportsDiagnostics, validateExportsErr := validateDataSourceExports(
		ctx,
		name,
		dataSource.Type.Value,
		dataSource.Exports,
		dataSourceMap,
		bpSchema,
		params,
		funcRegistry,
		refChainCollector,
		resourceRegistry,
		specDefinition,
	)
	diagnostics = append(diagnostics, validateExportsDiagnostics...)
	if validateExportsErr != nil {
		errs = append(errs, validateExportsErr)
	}

	customValidateOutput, err := dataSourceRegistry.CustomValidate(
		ctx,
		dataSource.Type.Value,
		&provider.DataSourceValidateInput{
			SchemaDataSource: dataSource,
			Params:           params,
		},
	)
	if err != nil {
		errs = append(errs, err)
	}
	if customValidateOutput != nil {
		diagnostics = append(diagnostics, customValidateOutput.Diagnostics...)
	}

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	return diagnostics, nil
}

func validateDataSourceType(
	ctx context.Context,
	dataSourceName string,
	dataSourceType *schema.DataSourceTypeWrapper,
	dataSourceMap *schema.DataSourceMap,
	dataSourceRegistry provider.DataSourceRegistry,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}

	if dataSourceType == nil {
		return diagnostics, errDataSourceMissingType(
			dataSourceName,
			getDataSourceMeta(dataSourceMap, dataSourceName),
		)
	}

	hasType, err := dataSourceRegistry.HasDataSourceType(ctx, dataSourceType.Value)
	if err != nil {
		return diagnostics, err
	}

	if !hasType {
		return diagnostics, errDataSourceTypeNotSupported(
			dataSourceName,
			dataSourceType.Value,
			getDataSourceMeta(dataSourceMap, dataSourceName),
		)
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
	resourceRegistry resourcehelpers.Registry,
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
	resourceRegistry resourcehelpers.Registry,
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
				if !isSubPrimitiveType(resolvedType) {
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
	resourceRegistry resourcehelpers.Registry,
) ([]*bpcore.Diagnostic, error) {
	if metadataSchema.Annotations == nil || metadataSchema.Annotations.Values == nil {
		return []*bpcore.Diagnostic{}, nil
	}

	dataSourceIdentifier := fmt.Sprintf("datasources.%s", dataSourceName)
	errs := []error{}
	diagnostics := []*bpcore.Diagnostic{}
	for key, annotation := range metadataSchema.Annotations.Values {
		if substitutions.ContainsSubstitution(key) {
			errs = append(errs, errDataSourceAnnotationKeyContainsSubstitution(
				dataSourceName,
				key,
				annotation.SourceMeta,
			))
		}

		annotationDiagnostics, err := validateMetadataAnnotation(
			ctx,
			dataSourceIdentifier,
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

func validateMetadataAnnotation(
	ctx context.Context,
	itemIdentifier string,
	annotation *substitutions.StringOrSubstitutions,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
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
				itemIdentifier,
				params,
				funcRegistry,
				refChainCollector,
				resourceRegistry,
			)
			if err != nil {
				errs = append(errs, err)
			} else {
				diagnostics = append(diagnostics, subDiagnostics...)
				handleResolvedTypeExpectingPrimitive(
					resolvedType,
					itemIdentifier,
					stringOrSub,
					annotation,
					"annotation",
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

func loadDataSourceSpecDefinition(
	ctx context.Context,
	dataSourceType string,
	dataSourceName string,
	location *source.Meta,
	params bpcore.BlueprintParams,
	dataSourceRegistry provider.DataSourceRegistry,
) (*provider.DataSourceSpecDefinition, error) {
	specDefOutput, err := dataSourceRegistry.GetSpecDefinition(
		ctx,
		dataSourceType,
		&provider.DataSourceGetSpecDefinitionInput{
			Params: params,
		},
	)
	if err != nil {
		return nil, err
	}

	if specDefOutput.SpecDefinition == nil {
		return nil, errDataSourceTypeMissingSpecDefinition(
			dataSourceName,
			dataSourceType,
			location,
		)
	}

	return specDefOutput.SpecDefinition, nil
}

func validateDataSourceFilter(
	ctx context.Context,
	name string,
	dataSourceType string,
	filter *schema.DataSourceFilter,
	dataSourceMap *schema.DataSourceMap,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
	dataSourceRegistry provider.DataSourceRegistry,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}

	if filter == nil {
		return diagnostics, errDataSourceMissingFilter(
			name, getDataSourceMeta(dataSourceMap, name),
		)
	}

	if filter.Field == nil || filter.Field.StringValue == nil || *filter.Field.StringValue == "" {
		return diagnostics, errDataSourceMissingFilterField(
			name, getDataSourceMeta(dataSourceMap, name),
		)
	}

	if filter.Operator == nil || filter.Operator.Value == "" {
		return diagnostics, errDataSourceMissingFilterOperator(
			name, getDataSourceMeta(dataSourceMap, name),
		)
	}

	if filter.Search == nil || len(filter.Search.Values) == 0 {
		return diagnostics, errDataSourceMissingFilterSearch(
			name, getDataSourceMeta(dataSourceMap, name),
		)
	}

	// Currently, only simple validation is provided for filter components.
	// This may be expanded in the future to include more complex validation
	// to check whether a given operator is supported for a specific field
	// based on a schema definition for filter fields.
	filterFieldsOutput, err := dataSourceRegistry.GetFilterFields(
		ctx,
		dataSourceType,
		&provider.DataSourceGetFilterFieldsInput{
			Params: params,
		},
	)
	if err != nil {
		return diagnostics, err
	}

	if len(filterFieldsOutput.Fields) == 0 {
		return diagnostics, errDataSourceTypeMissingFields(
			name,
			dataSourceType,
			filter.SourceMeta,
		)
	}

	if !slices.Contains(filterFieldsOutput.Fields, *filter.Field.StringValue) {
		return diagnostics, errDataSourceFilterFieldNotSupported(
			name,
			*filter.Field.StringValue,
			filter.SourceMeta,
		)
	}

	validateFilterOpDiagnostics, validateFilterOpErr := validateDataSourceFilterOperator(
		name,
		filter.Operator,
	)
	diagnostics = append(diagnostics, validateFilterOpDiagnostics...)
	if validateFilterOpErr != nil {
		return diagnostics, validateFilterOpErr
	}

	searchValidationDiagnostics, searchValidationErr := validateDataSourceFilterSearch(
		ctx,
		name,
		filter.Search,
		bpSchema,
		params,
		funcRegistry,
		refChainCollector,
		resourceRegistry,
	)
	diagnostics = append(diagnostics, searchValidationDiagnostics...)
	if searchValidationErr != nil {
		return diagnostics, searchValidationErr
	}

	return diagnostics, nil
}

func validateDataSourceFilterOperator(
	dataSourceName string,
	operator *schema.DataSourceFilterOperatorWrapper,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}

	if !core.SliceContains(schema.DataSourceFilterOperators, operator.Value) {
		return diagnostics, errInvalidDataSourceFilterOperator(dataSourceName, operator)
	}

	return diagnostics, nil
}

func validateDataSourceFilterSearch(
	ctx context.Context,
	dataSourceName string,
	search *schema.DataSourceFilterSearch,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
) ([]*bpcore.Diagnostic, error) {

	dataSourceIdentifier := fmt.Sprintf("datasources.%s", dataSourceName)
	errs := []error{}
	diagnostics := []*bpcore.Diagnostic{}
	for _, searchValue := range search.Values {
		searchValueDiagnostics, err := validateDataSourceFilterSearchValue(
			ctx,
			dataSourceIdentifier,
			searchValue,
			bpSchema,
			params,
			funcRegistry,
			refChainCollector,
			resourceRegistry,
		)
		diagnostics = append(diagnostics, searchValueDiagnostics...)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	return diagnostics, nil
}

func validateDataSourceFilterSearchValue(
	ctx context.Context,
	dataSourceIdentifier string,
	searchValue *substitutions.StringOrSubstitutions,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
) ([]*bpcore.Diagnostic, error) {
	if searchValue == nil {
		return []*bpcore.Diagnostic{}, nil
	}

	errs := []error{}
	diagnostics := []*bpcore.Diagnostic{}
	for i, stringOrSub := range searchValue.Values {
		nextLocation := getSubNextLocation(i, searchValue.Values)

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
				handleResolvedTypeExpectingPrimitive(
					resolvedType,
					dataSourceIdentifier,
					stringOrSub,
					searchValue,
					"search value",
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

func handleResolvedTypeExpectingPrimitive(
	resolvedType string,
	dataSourceIdentifier string,
	stringOrSub *substitutions.StringOrSubstitution,
	value *substitutions.StringOrSubstitutions,
	valueContext string,
	nextLocation *source.Meta,
	diagnostics *[]*bpcore.Diagnostic,
	errs *[]error,
) {
	if !isSubPrimitiveType(resolvedType) && resolvedType != string(substitutions.ResolvedSubExprTypeAny) {
		*errs = append(*errs, errInvalidSubType(
			dataSourceIdentifier,
			valueContext,
			resolvedType,
			stringOrSub.SourceMeta,
		))
	} else if resolvedType == string(substitutions.ResolvedSubExprTypeAny) {
		// Any type will produce a warning diagnostic as any is likely to match
		// and will be stringified in the final output, which is undesired
		// but not undefined behaviour.
		*diagnostics = append(
			*diagnostics,
			&bpcore.Diagnostic{
				Level: bpcore.DiagnosticLevelWarning,
				Message: fmt.Sprintf(
					"Substitution returns \"any\" type, this may produce "+
						"unexpected output in the %s, %ss are expected to be scalar values",
					valueContext,
					valueContext,
				),
				Range: toDiagnosticRange(value.SourceMeta, nextLocation),
			},
		)
	}
}

func validateDataSourceExports(
	ctx context.Context,
	dataSourceName string,
	dataSourceType string,
	exports *schema.DataSourceFieldExportMap,
	dataSourceMap *schema.DataSourceMap,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
	specDefinition *provider.DataSourceSpecDefinition,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	if exports == nil || len(exports.Values) == 0 {
		return diagnostics, errDataSourceMissingExports(
			dataSourceName,
			getDataSourceMeta(dataSourceMap, dataSourceName),
		)
	}

	errs := []error{}
	for exportName, export := range exports.Values {
		exportDiagnostics, err := validateDataSourceExport(
			ctx,
			dataSourceName,
			dataSourceType,
			export,
			exportName,
			/* wrapperLocation */ exports.SourceMeta[exportName],
			bpSchema,
			params,
			funcRegistry,
			refChainCollector,
			resourceRegistry,
			specDefinition,
		)
		diagnostics = append(diagnostics, exportDiagnostics...)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	return diagnostics, nil
}

func validateDataSourceExport(
	ctx context.Context,
	dataSourceName string,
	dataSourceType string,
	export *schema.DataSourceFieldExport,
	exportName string,
	wrapperLocation *source.Meta,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
	specDefinition *provider.DataSourceSpecDefinition,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	if export == nil {
		return diagnostics, errDataSourceExportEmpty(
			dataSourceName,
			exportName,
			wrapperLocation,
		)
	}

	finalExportName := exportName
	if export.AliasFor != nil &&
		export.AliasFor.StringValue != nil &&
		*export.AliasFor.StringValue != "" {
		finalExportName = *export.AliasFor.StringValue
	}
	fieldSchema, hasField := specDefinition.Fields[finalExportName]
	// Field schema may incorrectly set to nil by the data source provider.
	if !hasField || fieldSchema == nil {
		return diagnostics, errDataSourceExportFieldNotSupported(
			dataSourceName,
			dataSourceType,
			exportName,
			finalExportName,
			wrapperLocation,
		)
	}

	if export.Type == nil {
		return diagnostics, errDataSourceExportTypeMissing(
			dataSourceName,
			exportName,
			wrapperLocation,
		)
	}

	if !core.SliceContains(schema.DataSourceFieldTypes, export.Type.Value) {
		return diagnostics, errInvalidDataSourceFieldType(
			dataSourceName,
			exportName,
			export.Type,
		)
	}

	if !schemaMatchesDataSourceFieldType(fieldSchema, export.Type) {
		return diagnostics, errDataSourceExportFieldTypeMismatch(
			dataSourceName,
			exportName,
			finalExportName,
			string(fieldSchema.Type),
			string(export.Type.Value),
			wrapperLocation,
		)
	}

	diagnostics, err := validateDescription(
		ctx,
		fmt.Sprintf("datasources.%s.exports.%s", dataSourceName, exportName),
		export.Description,
		bpSchema,
		params,
		funcRegistry,
		refChainCollector,
		resourceRegistry,
	)
	if err != nil {
		return diagnostics, err
	}

	return diagnostics, nil
}

func schemaMatchesDataSourceFieldType(
	fieldSchema *provider.DataSourceSpecSchema,
	exportType *schema.DataSourceFieldTypeWrapper,
) bool {
	if fieldSchema == nil || exportType == nil {
		return false
	}

	if fieldSchema.Type == provider.DataSourceSpecTypeString &&
		exportType.Value == schema.DataSourceFieldTypeString {
		return true
	}

	if fieldSchema.Type == provider.DataSourceSpecTypeInteger &&
		exportType.Value == schema.DataSourceFieldTypeInteger {
		return true
	}

	if fieldSchema.Type == provider.DataSourceSpecTypeFloat &&
		exportType.Value == schema.DataSourceFieldTypeFloat {
		return true
	}

	if fieldSchema.Type == provider.DataSourceSpecTypeBoolean &&
		exportType.Value == schema.DataSourceFieldTypeBoolean {
		return true
	}

	if fieldSchema.Type == provider.DataSourceSpecTypeArray &&
		exportType.Value == schema.DataSourceFieldTypeArray {
		return true
	}

	return false
}

func getDataSourceMeta(varMap *schema.DataSourceMap, varName string) *source.Meta {
	if varMap == nil {
		return nil
	}

	return varMap.SourceMeta[varName]
}
