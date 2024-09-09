package container

import (
	"context"
	"fmt"
	"strings"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/links"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/resourcehelpers"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/speccore"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/blueprint/transform"
	"github.com/two-hundred/celerity/libs/blueprint/validation"
	"github.com/two-hundred/celerity/libs/common/core"
)

// Loader provides the interface for a service that deals
// with loading blueprints for which instances can be deployed.
// The loader also provides methods for validating a spec
// (and optionally its variables) without associating
// it with an instance.
type Loader interface {
	// Load deals with loading a blueprint specification from the local file system
	// along with provider and blueprint variables.
	// Provider and blueprint variables can be provided to the blueprint container
	// methods at a later stage, you can provide an empty set of parameters when
	// loading a spec.
	Load(
		ctx context.Context,
		blueprintSpecFile string,
		params bpcore.BlueprintParams,
	) (BlueprintContainer, error)

	// Validate deals with validating a specification that lies on the local
	// file system without loading a blueprint container.
	// Provider and blueprint variables can be provided for enhanced
	// validation that also checks variables.
	//
	// This also returns validation diagnostics for warning and info level
	// diagnostics that point out potential issues that may occur when executing
	// a blueprint. Diagnostics do not include errors, errors should be unpacked from
	// the returned error.
	Validate(
		ctx context.Context,
		blueprintSpecFile string,
		params bpcore.BlueprintParams,
	) (links.SpecLinkInfo, []*bpcore.Diagnostic, error)

	// LoadString deals with loading a blueprint specification from a string
	// along with provider and blueprint variables.
	// Provider and blueprint variables can be provided to the blueprint container
	// methods at a later stage, you can provide an empty set of parameters when
	// loading a spec.
	LoadString(
		ctx context.Context,
		blueprintSpec string,
		inputFormat schema.SpecFormat,
		params bpcore.BlueprintParams,
	) (BlueprintContainer, error)

	// ValidateString deals with validating a specification provided as a string
	// without loading a blueprint container.
	// Provider and blueprint variables can be provided for enhanced
	// validation that also checks variables.
	//
	// This also returns validation diagnostics for error, warning and info level
	// diagnostics that point out potential issues that may occur when executing
	// a blueprint. Diagnostics do not include all errors, errors should be unpacked from
	// the returned error in addition to the diagnostics.
	ValidateString(
		ctx context.Context,
		blueprintSpec string,
		inputFormat schema.SpecFormat,
		params bpcore.BlueprintParams,
	) (links.SpecLinkInfo, []*bpcore.Diagnostic, error)

	// LoadFromSchema deals with loading a blueprint specification from a schema
	// that has already been parsed along with provider and blueprint variables.
	// This is mostly useful for loading a blueprint from a schema cache to speed up
	// loading times, this is especially useful for blueprints that make use of a lot
	// ${..} substitutions and references.
	// Provider and blueprint variables can be provided to the blueprint container
	// methods at a later stage, you can provide an empty set of parameters when
	// loading a spec.
	LoadFromSchema(
		ctx context.Context,
		blueprintSchema *schema.Blueprint,
		params bpcore.BlueprintParams,
	) (BlueprintContainer, error)

	// ValidateFromSchema deals with validating a specification provided as a schema
	// without loading a blueprint container.
	// Provider and blueprint variables can be provided for enhanced
	// validation that also checks variables.
	//
	// This also returns validation diagnostics for error, warning and info level
	// diagnostics that point out potential issues that may occur when executing
	// a blueprint. Diagnostics do not include all errors, errors should be unpacked from
	// the returned error in addition to the diagnostics.
	ValidateFromSchema(
		ctx context.Context,
		blueprintSchema *schema.Blueprint,
		params bpcore.BlueprintParams,
	) (links.SpecLinkInfo, []*bpcore.Diagnostic, error)
}

type loadBlueprintInfo struct {
	specOrFilePath  string
	preloadedSchema *schema.Blueprint
}

// Stores the full blueprint schema and direct access to the
// mapping of resource names to their schemas for convenience.
// This is structure of the spec encapsulated by the blueprint container.
type internalBlueprintSpec struct {
	resourceSchemas map[string]*schema.Resource
	schema          *schema.Blueprint
}

func (s *internalBlueprintSpec) ResourceSchema(resourceName string) *schema.Resource {
	resourceSchema, ok := s.resourceSchemas[resourceName]
	if !ok {
		return nil
	}
	return resourceSchema
}

func (s *internalBlueprintSpec) Schema() *schema.Blueprint {
	return s.schema
}

type defaultLoader struct {
	providers              map[string]provider.Provider
	specTransformers       map[string]transform.SpecTransformer
	stateContainer         state.Container
	updateChan             chan Update
	validateRuntimeValues  bool
	validateAfterTransform bool
	transformSpec          bool
	refChainCollector      validation.RefChainCollector
	funcRegistry           provider.FunctionRegistry
	resourceRegistry       resourcehelpers.Registry
	dataSourceRegistry     provider.DataSourceRegistry
}

type LoaderOption func(loader *defaultLoader)

// WithLoaderValidateRuntimeValues sets the flag to determine whether
// runtime values should be validated when loading blueprints.
// This is useful when you want to validate a blueprint spec without
// associating it with an instance.
// (e.g. validation for code editors or CLI dry runs)
//
// When this option is not provided, the default value is false.
func WithLoaderValidateRuntimeValues(validateRuntimeValues bool) LoaderOption {
	return func(loader *defaultLoader) {
		loader.validateRuntimeValues = validateRuntimeValues
	}
}

// WithLoaderValidateAfterTransform sets the flag to determine whether
// resource validation should be performed after applying transformers to the
// blueprint spec.
// This is useful when you want to catch potential bugs in transformer implementations
// at the load/validate stage.
//
// When this option is not provided, the default value is false.
func WithLoaderValidateAfterTransform(validateAfterTransform bool) LoaderOption {
	return func(loader *defaultLoader) {
		loader.validateAfterTransform = validateAfterTransform
	}
}

// WithLoaderTransformSpec sets the flag to determine whether transformers should be applied
// to the blueprint spec when loading blueprints.
// This is useful when you want to validate a blueprint spec without
// applying any transformations.
// (e.g. validation for code editors or CLI dry runs)
//
// When this option is not provided, the default value is true.
func WithLoaderTransformSpec(transformSpec bool) LoaderOption {
	return func(loader *defaultLoader) {
		loader.transformSpec = transformSpec
	}
}

// NewDefaultLoader creates a new instance of the default
// implementation of a blueprint container loader.
// The map of providers must be a map of provider namespaces
// to the implementation.
// For example, for all resource types "aws/*" you would have a mapping
// namespace "aws" to the AWS provider.
// The namespace must be the prefix of resource, data source and custom
// variable types defined by the provider.
// If there is no provider for the prefix of a resource, data source or
// custom variable type in a blueprint, it will fail.
//
// You can provide options for multiple flags that can be set to determine
// how the loader should behave, such as whether to validate runtime values
// or validate after transformation when loading blueprints.
func NewDefaultLoader(
	providers map[string]provider.Provider,
	specTransformers map[string]transform.SpecTransformer,
	stateContainer state.Container,
	updateChan chan Update,
	refChainCollector validation.RefChainCollector,
	opts ...LoaderOption,
) Loader {
	resourceRegistry := resourcehelpers.NewRegistry(providers, specTransformers)
	funcRegistry := provider.NewFunctionRegistry(providers)
	dataSourceRegistry := provider.NewDataSourceRegistry(providers)
	loader := &defaultLoader{
		providers:          providers,
		specTransformers:   specTransformers,
		stateContainer:     stateContainer,
		updateChan:         updateChan,
		refChainCollector:  refChainCollector,
		funcRegistry:       funcRegistry,
		resourceRegistry:   resourceRegistry,
		dataSourceRegistry: dataSourceRegistry,
	}

	for _, opt := range opts {
		opt(loader)
	}

	return loader
}

func (l *defaultLoader) Load(ctx context.Context, blueprintSpecFile string, params bpcore.BlueprintParams) (BlueprintContainer, error) {
	loadInfo := &loadBlueprintInfo{
		specOrFilePath: blueprintSpecFile,
	}
	container, _, err := l.loadSpecAndLinkInfo(ctx, loadInfo, params, schema.Load, deriveSpecFormat)
	return container, err
}

func (l *defaultLoader) Validate(
	ctx context.Context,
	blueprintSpecFile string,
	params bpcore.BlueprintParams,
) (links.SpecLinkInfo, []*bpcore.Diagnostic, error) {
	loadInfo := &loadBlueprintInfo{
		specOrFilePath: blueprintSpecFile,
	}
	container, diagnostics, err := l.loadSpecAndLinkInfo(ctx, loadInfo, params, schema.Load, deriveSpecFormat)
	if err != nil {
		return nil, diagnostics, err
	}
	return container.SpecLinkInfo(), []*bpcore.Diagnostic{}, nil
}

func (l *defaultLoader) loadSpecAndLinkInfo(
	ctx context.Context,
	loadInfo *loadBlueprintInfo,
	params bpcore.BlueprintParams,
	schemaLoader schema.Loader,
	formatLoader func(string) (schema.SpecFormat, error),
) (BlueprintContainer, []*bpcore.Diagnostic, error) {
	blueprintSpec, diagnostics, err := l.loadSpec(ctx, loadInfo, params, schemaLoader, formatLoader)
	if err != nil {
		return nil, diagnostics, err
	}
	resourceProviderMap := l.createResourceProviderMap(blueprintSpec)
	linkInfo, err := links.NewDefaultLinkInfoProvider(resourceProviderMap, blueprintSpec, params)
	if err != nil {
		return nil, diagnostics, err
	}

	// Once we have loaded the link information,
	// we can capture links as references to include in checks
	// for reference/link cycles to catch the case where a resource selects another through
	// a link and the other resource references a property of the first resource.
	err = l.collectLinksAsReferences(ctx, linkInfo)
	if err != nil {
		return nil, diagnostics, err
	}

	refCycleRoots := l.refChainCollector.FindCircularReferences()
	if len(refCycleRoots) > 0 {
		return nil, diagnostics, validation.ErrReferenceCycles(refCycleRoots)
	}

	return NewDefaultBlueprintContainer(
		l.stateContainer,
		resourceProviderMap,
		blueprintSpec,
		linkInfo,
		diagnostics,
		l.updateChan,
	), diagnostics, nil
}

func (l *defaultLoader) collectLinksAsReferences(
	ctx context.Context,
	linkInfo links.SpecLinkInfo,
) error {
	chains, err := linkInfo.Links(ctx)
	if err != nil {
		return err
	}

	for _, chain := range chains {
		err = l.collectLinksFromChain(ctx, chain)
		if err != nil {
			return err
		}
	}

	return nil
}

func (l *defaultLoader) collectLinksFromChain(
	ctx context.Context,
	chain *links.ChainLink,
) error {
	referencedByResourceID := fmt.Sprintf("resources.%s", chain.ResourceName)
	for _, link := range chain.LinksTo {
		resourceID := fmt.Sprintf("resources.%s", link.ResourceName)
		err := l.refChainCollector.Collect(resourceID, link, referencedByResourceID)
		if err != nil {
			return err
		}
		for _, childChain := range link.LinksTo {
			// There is no risk of infinite recursion due to cyclic links as at this point,
			// any pure link cycles have been detected and reported.
			err = l.collectLinksFromChain(ctx, childChain)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (l *defaultLoader) createResourceProviderMap(blueprintSpec speccore.BlueprintSpec) map[string]provider.Provider {
	resourceProviderMap := map[string]provider.Provider{}
	resources := map[string]*schema.Resource{}
	if blueprintSpec.Schema().Resources != nil {
		resources = blueprintSpec.Schema().Resources.Values
	}

	for name := range resources {
		namespace := strings.SplitAfter(name, "/")[0]
		resourceProviderMap[name] = l.providers[namespace]
	}
	return resourceProviderMap
}

func (l *defaultLoader) loadSpec(
	ctx context.Context,
	loadInfo *loadBlueprintInfo,
	params bpcore.BlueprintParams,
	loader schema.Loader,
	formatLoader func(string) (schema.SpecFormat, error),
) (*internalBlueprintSpec, []*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}

	blueprintSchema, err := loadBlueprintSpec(loadInfo, formatLoader, loader)
	if err != nil {
		return nil, diagnostics, err
	}

	var bpValidationDiagnostics []*bpcore.Diagnostic
	validationErrors := []error{}
	bpValidationDiagnostics, err = l.validateBlueprint(ctx, blueprintSchema)
	diagnostics = append(diagnostics, bpValidationDiagnostics...)
	if err != nil {
		validationErrors = append(validationErrors, err)
	}

	var variableDiagnostics []*bpcore.Diagnostic
	variableDiagnostics, err = l.validateVariables(ctx, blueprintSchema, params)
	diagnostics = append(diagnostics, variableDiagnostics...)
	if err != nil {
		validationErrors = append(validationErrors, err)
	}

	var valueDiagnostics []*bpcore.Diagnostic
	valueDiagnostics, err = l.validateValues(ctx, blueprintSchema, params)
	diagnostics = append(diagnostics, valueDiagnostics...)
	if err != nil {
		validationErrors = append(validationErrors, err)
	}

	var includeDiagnostics []*bpcore.Diagnostic
	includeDiagnostics, err = l.validateIncludes(ctx, blueprintSchema, params)
	diagnostics = append(diagnostics, includeDiagnostics...)
	if err != nil {
		validationErrors = append(validationErrors, err)
	}

	var exportDiagnostics []*bpcore.Diagnostic
	exportDiagnostics, err = l.validateExports(ctx, blueprintSchema, params)
	diagnostics = append(diagnostics, exportDiagnostics...)
	if err != nil {
		validationErrors = append(validationErrors, err)
	}

	var metadataDiagnostics []*bpcore.Diagnostic
	metadataDiagnostics, err = l.validateMetadata(ctx, blueprintSchema, params)
	diagnostics = append(diagnostics, metadataDiagnostics...)
	if err != nil {
		validationErrors = append(validationErrors, err)
	}

	var dataSourceDiagnostics []*bpcore.Diagnostic
	dataSourceDiagnostics, err = l.validateDataSources(ctx, blueprintSchema, params)
	diagnostics = append(diagnostics, dataSourceDiagnostics...)
	if err != nil {
		validationErrors = append(validationErrors, err)
	}

	var resourceDiagnostics []*bpcore.Diagnostic
	resourceDiagnostics, err = l.validateResources(ctx, blueprintSchema, params)
	diagnostics = append(diagnostics, resourceDiagnostics...)
	if err != nil {
		validationErrors = append(validationErrors, err)
	}

	// Apply some validation to get diagnostics about non-standard transformers
	// that may not be present at runtime.
	var transformDiagnostics []*bpcore.Diagnostic
	transformDiagnostics, err = l.validateTransforms(ctx, blueprintSchema)
	diagnostics = append(diagnostics, transformDiagnostics...)
	if err != nil {
		validationErrors = append(validationErrors, err)
	}

	if !l.transformSpec {
		spec := &internalBlueprintSpec{
			schema: blueprintSchema,
		}
		if len(validationErrors) > 0 {
			return spec, diagnostics, validation.ErrMultipleValidationErrors(validationErrors)
		}
		return spec, diagnostics, nil
	}

	transformers, err := l.collectTransformers(blueprintSchema)
	if err != nil {
		return nil, diagnostics, validation.ErrMultipleValidationErrors(
			append(validationErrors, err),
		)
	}
	for _, transformer := range transformers {
		output, err := transformer.Transform(ctx, &transform.SpecTransformerTransformInput{
			InputBlueprint: blueprintSchema,
		})
		blueprintSchema = output.TransformedBlueprint
		if err != nil {
			return nil, diagnostics, validation.ErrMultipleValidationErrors(
				append(validationErrors, err),
			)
		}
	}

	if l.validateAfterTransform && len(transformers) > 0 {
		// Validate after transformations to help with catching bugs in transformer implementations.
		// This ultimately prevents transformers from expanding their abstractions into invalid
		// representations of the lower level resources.
		var resourceDiagnostics []*bpcore.Diagnostic
		resourceDiagnostics, err = l.validateResources(ctx, blueprintSchema, params)
		diagnostics = append(diagnostics, resourceDiagnostics...)
		if err != nil {
			validationErrors = append(validationErrors, err)
		}
	}

	if len(validationErrors) > 0 {
		return nil, diagnostics, validation.ErrMultipleValidationErrors(validationErrors)
	}

	return &internalBlueprintSpec{
		schema: blueprintSchema,
	}, diagnostics, nil
}

func (l *defaultLoader) collectTransformers(schema *schema.Blueprint) ([]transform.SpecTransformer, error) {
	usedBySpec := []transform.SpecTransformer{}
	missingTransformers := []string{}
	childErrors := []error{}
	for i, name := range schema.Transform.Values {
		transformer, exists := l.specTransformers[name]
		if exists {
			usedBySpec = append(usedBySpec, transformer)
		} else {
			missingTransformers = append(missingTransformers, name)
			sourceMeta := (*source.Meta)(nil)
			if len(schema.Transform.SourceMeta) > 0 {
				sourceMeta = schema.Transform.SourceMeta[i]
			}
			line, col := source.PositionFromSourceMeta(sourceMeta)
			childErrors = append(childErrors, errTransformerMissing(name, line, col))
		}
	}
	if len(missingTransformers) > 0 {
		firstSourceMeta := (*source.Meta)(nil)
		if len(schema.Transform.SourceMeta) > 0 {
			firstSourceMeta = schema.Transform.SourceMeta[0]
		}

		line, col := source.PositionFromSourceMeta(firstSourceMeta)
		return nil, errTransformersMissing(missingTransformers, childErrors, line, col)
	}
	return usedBySpec, nil
}

func (l *defaultLoader) validateBlueprint(ctx context.Context, bpSchema *schema.Blueprint) ([]*bpcore.Diagnostic, error) {
	return validation.ValidateBlueprint(ctx, bpSchema)
}

func (l *defaultLoader) validateTransforms(ctx context.Context, bpSchema *schema.Blueprint) ([]*bpcore.Diagnostic, error) {
	return validation.ValidateTransforms(ctx, bpSchema, l.transformSpec)
}

func (l *defaultLoader) validateVariables(
	ctx context.Context,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	if bpSchema.Variables == nil {
		return diagnostics, nil
	}

	// To be as useful as possible, we'll collect and
	// report issues for all the problematic variables.
	variableErrors := map[string][]error{}
	for name, varSchema := range bpSchema.Variables.Values {
		currentVarErrs := l.validateVariable(ctx, &diagnostics, name, varSchema, bpSchema, params)
		if len(currentVarErrs) > 0 {
			variableErrors[name] = currentVarErrs
		}
	}

	if len(variableErrors) > 0 {
		return diagnostics, errVariableValidationError(variableErrors)
	}

	return diagnostics, nil
}

func (l *defaultLoader) validateVariable(
	ctx context.Context,
	diagnostics *[]*bpcore.Diagnostic,
	name string,
	varSchema *schema.Variable,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
) []error {
	currentVarErrs := []error{}
	err := validation.ValidateVariableName(name, bpSchema.Variables)
	if err != nil {
		currentVarErrs = append(currentVarErrs, err)
	}
	if core.SliceContains(schema.CoreVariableTypes, varSchema.Type) {
		coreVarDiagnostics, err := validation.ValidateCoreVariable(
			ctx, name, varSchema, bpSchema.Variables, params, l.validateRuntimeValues,
		)
		if err != nil {
			currentVarErrs = append(currentVarErrs, err)
		}
		*diagnostics = append(*diagnostics, coreVarDiagnostics...)
	} else {
		customVarDiagnostics, err := l.validateCustomVariableType(ctx, name, varSchema, bpSchema.Variables, params)
		if err != nil {
			currentVarErrs = append(currentVarErrs, err)
		}
		*diagnostics = append(*diagnostics, customVarDiagnostics...)
	}

	l.refChainCollector.Collect(fmt.Sprintf("variables.%s", name), varSchema, "")
	return currentVarErrs
}

func (l *defaultLoader) validateValues(
	ctx context.Context,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	if bpSchema.Values == nil {
		return diagnostics, nil
	}

	valueErrors := map[string][]error{}
	for name, valSchema := range bpSchema.Values.Values {
		currentValErrs := l.validateValue(ctx, &diagnostics, name, valSchema, bpSchema, params)
		if len(currentValErrs) > 0 {
			valueErrors[name] = currentValErrs
		}
	}

	if len(valueErrors) > 0 {
		return diagnostics, errVariableValidationError(valueErrors)
	}

	return diagnostics, nil
}

func (l *defaultLoader) validateValue(
	ctx context.Context,
	diagnostics *[]*bpcore.Diagnostic,
	name string,
	valSchema *schema.Value,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
) []error {
	currentValErrs := []error{}
	err := validation.ValidateValueName(name, bpSchema.Values)
	if err != nil {
		currentValErrs = append(currentValErrs, err)
	}

	resultDiagnostics, err := validation.ValidateValue(
		ctx,
		name,
		valSchema,
		bpSchema,
		params,
		l.funcRegistry,
		l.refChainCollector,
		l.resourceRegistry,
	)
	if err != nil {
		currentValErrs = append(currentValErrs, err)
	}
	*diagnostics = append(*diagnostics, resultDiagnostics...)

	l.refChainCollector.Collect(fmt.Sprintf("values.%s", name), valSchema, "")
	return currentValErrs
}

func (l *defaultLoader) validateIncludes(
	ctx context.Context,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	if bpSchema.Include == nil {
		return diagnostics, nil
	}

	// We'll collect and report issues for all the problematic includes.
	includeErrors := map[string]error{}
	for name, includeSchema := range bpSchema.Include.Values {
		includeDiagnostics, err := validation.ValidateInclude(
			ctx,
			name,
			includeSchema,
			bpSchema.Include,
			bpSchema,
			params,
			l.funcRegistry,
			l.refChainCollector,
			l.resourceRegistry,
		)
		if err != nil {
			includeErrors[name] = err
		}
		diagnostics = append(diagnostics, includeDiagnostics...)
	}

	if len(includeErrors) > 0 {
		return diagnostics, errIncludeValidationError(includeErrors)
	}

	return diagnostics, nil
}

func (l *defaultLoader) validateExports(
	ctx context.Context,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	if bpSchema.Exports == nil {
		return diagnostics, nil
	}

	// We'll collect and report issues for all the problematic exports.
	exportErrors := map[string]error{}
	for name, exportSchema := range bpSchema.Exports.Values {
		exportDiagnostics, err := validation.ValidateExport(
			ctx,
			name,
			exportSchema,
			bpSchema.Exports,
			bpSchema,
			params,
			l.funcRegistry,
			l.refChainCollector,
			l.resourceRegistry,
		)
		if err != nil {
			exportErrors[name] = err
		}
		diagnostics = append(diagnostics, exportDiagnostics...)
	}

	if len(exportErrors) > 0 {
		return diagnostics, errExportValidationError(exportErrors)
	}

	return diagnostics, nil
}

func (l *defaultLoader) validateMetadata(
	ctx context.Context,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
) ([]*bpcore.Diagnostic, error) {
	if bpSchema.Metadata == nil {
		return []*bpcore.Diagnostic{}, nil
	}

	return validation.ValidateMappingNode(
		ctx,
		"root",
		"metadata",
		bpSchema.Metadata,
		bpSchema,
		params,
		l.funcRegistry,
		l.refChainCollector,
		l.resourceRegistry,
	)
}

func (l *defaultLoader) validateCustomVariableType(
	ctx context.Context,
	varName string,
	varSchema *schema.Variable,
	variables *schema.VariableMap,
	params bpcore.BlueprintParams,
) ([]*bpcore.Diagnostic, error) {
	providerCustomVarType, err := l.deriveProviderCustomVarType(ctx, varName, varSchema)
	if err != nil {
		return []*bpcore.Diagnostic{}, err
	}
	return validation.ValidateCustomVariable(ctx, varName, varSchema, variables, params, providerCustomVarType)
}

func (l *defaultLoader) deriveProviderCustomVarType(ctx context.Context, varName string, varSchema *schema.Variable) (provider.CustomVariableType, error) {
	// The provider should be keyed exactly by \w+\/ which is the custom type prefix.
	// Avoid using a regular expression as it is more efficient to split the string.
	parts := strings.SplitAfter(string(varSchema.Type), "/")
	if len(parts) == 0 {
		line, col := source.PositionFromSourceMeta(varSchema.SourceMeta)
		return nil, errInvalidCustomVariableType(varName, varSchema.Type, line, col)
	}

	providerKey := parts[0]

	provider, ok := l.providers[providerKey]
	if !ok {
		line, col := source.PositionFromSourceMeta(varSchema.SourceMeta)
		return nil, errMissingProviderForCustomVarType(providerKey, varName, varSchema.Type, line, col)
	}

	customVarType, err := provider.CustomVariableType(ctx, string(varSchema.Type))
	if err != nil {
		return nil, err
	}

	return customVarType, nil
}

func (l *defaultLoader) validateDataSources(
	ctx context.Context,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	if bpSchema.DataSources == nil {
		return diagnostics, nil
	}
	// To be as useful as possible, we'll collect and
	// report issues for all the problematic resources.
	dataSourceErrors := map[string][]error{}
	for name, dataSourceSchema := range bpSchema.DataSources.Values {
		currentDataSourceErrs := l.validateDataSource(
			ctx, &diagnostics, name, dataSourceSchema, bpSchema, params,
		)
		if len(currentDataSourceErrs) > 0 {
			dataSourceErrors[name] = currentDataSourceErrs
		}
	}

	if len(dataSourceErrors) > 0 {
		return diagnostics, errDataSourceValidationError(dataSourceErrors)
	}

	return diagnostics, nil
}

func (l *defaultLoader) validateDataSource(
	ctx context.Context,
	diagnostics *[]*bpcore.Diagnostic,
	name string,
	dataSourceSchema *schema.DataSource,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
) []error {
	currentDataSourceErrs := []error{}
	err := validation.ValidateDataSourceName(name, bpSchema.DataSources)
	if err != nil {
		currentDataSourceErrs = append(currentDataSourceErrs, err)
	}

	var validateDataSourceDiagnostics []*bpcore.Diagnostic
	validateDataSourceDiagnostics, err = validation.ValidateDataSource(
		ctx,
		name,
		dataSourceSchema,
		bpSchema.DataSources,
		bpSchema,
		params,
		l.funcRegistry,
		l.refChainCollector,
		l.resourceRegistry,
		l.dataSourceRegistry,
	)
	*diagnostics = append(*diagnostics, validateDataSourceDiagnostics...)
	if err != nil {
		currentDataSourceErrs = append(currentDataSourceErrs, err)
	}

	l.refChainCollector.Collect(fmt.Sprintf("datasources.%s", name), dataSourceSchema, "")
	return currentDataSourceErrs
}

func (l *defaultLoader) validateResources(
	ctx context.Context,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	if bpSchema.Resources == nil {
		return diagnostics, nil
	}
	// To be as useful as possible, we'll collect and
	// report issues for all the problematic resources.
	resourceErrors := map[string][]error{}
	for name, resourceSchema := range bpSchema.Resources.Values {
		currentResouceErrs := l.validateResource(
			ctx, &diagnostics, name, resourceSchema, bpSchema, params,
		)
		if len(currentResouceErrs) > 0 {
			resourceErrors[name] = currentResouceErrs
		}
	}

	if len(resourceErrors) > 0 {
		return diagnostics, errResourceValidationError(resourceErrors)
	}

	return diagnostics, nil
}

func (l *defaultLoader) validateResource(
	ctx context.Context,
	diagnostics *[]*bpcore.Diagnostic,
	name string,
	resourceSchema *schema.Resource,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
) []error {
	currentResouceErrs := []error{}
	err := validation.ValidateResourceName(name, bpSchema.Resources)
	if err != nil {
		currentResouceErrs = append(currentResouceErrs, err)
	}
	err = validation.PreValidateResourceSpec(ctx, name, resourceSchema, bpSchema.Resources)
	if err != nil {
		currentResouceErrs = append(currentResouceErrs, err)
	}

	var validateResourceDiagnostics []*bpcore.Diagnostic
	validateResourceDiagnostics, err = validation.ValidateResource(
		ctx,
		name,
		resourceSchema,
		bpSchema.Resources,
		bpSchema,
		params,
		l.funcRegistry,
		l.refChainCollector,
		l.resourceRegistry,
	)
	*diagnostics = append(*diagnostics, validateResourceDiagnostics...)
	if err != nil {
		currentResouceErrs = append(currentResouceErrs, err)
	}

	l.refChainCollector.Collect(fmt.Sprintf("resources.%s", name), resourceSchema, "")
	return currentResouceErrs
}

func (l *defaultLoader) LoadString(
	ctx context.Context,
	blueprintSpec string,
	inputFormat schema.SpecFormat,
	params bpcore.BlueprintParams,
) (BlueprintContainer, error) {
	loadInfo := &loadBlueprintInfo{
		specOrFilePath: blueprintSpec,
	}
	container, _, err := l.loadSpecAndLinkInfo(ctx, loadInfo, params, schema.LoadString, predefinedFormatFactory(inputFormat))
	return container, err
}

func (l *defaultLoader) ValidateString(
	ctx context.Context,
	blueprintSpec string,
	inputFormat schema.SpecFormat,
	params bpcore.BlueprintParams,
) (links.SpecLinkInfo, []*bpcore.Diagnostic, error) {
	loadInfo := &loadBlueprintInfo{
		specOrFilePath: blueprintSpec,
	}
	container, diagnostics, err := l.loadSpecAndLinkInfo(ctx, loadInfo, params, schema.LoadString, predefinedFormatFactory(inputFormat))
	if err != nil {
		return nil, diagnostics, err
	}
	return container.SpecLinkInfo(), container.Diagnostics(), nil
}

func (l *defaultLoader) LoadFromSchema(
	ctx context.Context,
	blueprintSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
) (BlueprintContainer, error) {
	loadInfo := &loadBlueprintInfo{
		preloadedSchema: blueprintSchema,
	}
	container, _, err := l.loadSpecAndLinkInfo(
		ctx,
		loadInfo,
		params,
		/* schemaLoader */ nil,
		/* formatLoader */ nil,
	)
	return container, err
}

func (l *defaultLoader) ValidateFromSchema(
	ctx context.Context,
	blueprintSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
) (links.SpecLinkInfo, []*bpcore.Diagnostic, error) {
	loadInfo := &loadBlueprintInfo{
		preloadedSchema: blueprintSchema,
	}
	container, diagnostics, err := l.loadSpecAndLinkInfo(
		ctx,
		loadInfo,
		params,
		/* schemaLoader */ nil,
		/* formatLoader */ nil,
	)
	if err != nil {
		return nil, diagnostics, err
	}
	return container.SpecLinkInfo(), container.Diagnostics(), nil
}

func loadBlueprintSpec(
	loadInfo *loadBlueprintInfo,
	formatLoader func(string) (schema.SpecFormat, error),
	loader schema.Loader,
) (*schema.Blueprint, error) {
	if loadInfo.preloadedSchema != nil {
		return loadInfo.preloadedSchema, nil
	}

	format, err := formatLoader(loadInfo.specOrFilePath)
	if err != nil {
		return nil, err
	}

	return loader(loadInfo.specOrFilePath, format)
}
