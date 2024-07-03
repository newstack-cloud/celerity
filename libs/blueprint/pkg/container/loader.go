package container

import (
	"context"
	"strings"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/links"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/provider"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/source"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/speccore"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/state"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/transform"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/validation"
	"github.com/two-hundred/celerity/libs/common/pkg/core"
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
	Load(ctx context.Context, blueprintSpecFile string, params bpcore.BlueprintParams) (BlueprintContainer, error)
	// Validate deals with validating a specification that lies on the local
	// file system without loading a blueprint container.
	// Provider and blueprint variables can be provided for enhanced
	// validation that also checks variables.
	Validate(ctx context.Context, blueprintSpecFile string, params bpcore.BlueprintParams) (links.SpecLinkInfo, error)
	// LoadString deals with loading a blueprint specification from a string
	// along with provider and blueprint variables.
	// Provider and blueprint variables can be provided to the blueprint container
	// methods at a later stage, you can provide an empty set of parameters when
	// loading a spec.
	LoadString(ctx context.Context, blueprintSpec string, inputFormat schema.SpecFormat, params bpcore.BlueprintParams) (BlueprintContainer, error)
	// ValidateString deals with validating a specification provided as a string
	// without loading a blueprint container.
	// Provider and blueprint variables can be provided for enhanced
	// validation that also checks variables.
	ValidateString(ctx context.Context, blueprintSpec string, inputFormat schema.SpecFormat, params bpcore.BlueprintParams) (links.SpecLinkInfo, error)
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
	providers             map[string]provider.Provider
	specTransformers      map[string]transform.SpecTransformer
	stateContainer        state.Container
	updateChan            chan Update
	validateRuntimeValues bool
	transformSpec         bool
}

// NewDefaultLoader creates a new instance of the default
// implementation of a blueprint container loader.
// The map of providers must be a map of provider resource type prefix
// to a provider.
// For example, for all resource types "aws/*" you would have a mapping
// "aws/" to the AWS provider.
// If there is no provider for the prefix of a resource type in a
// blueprint, it will fail.
//
// You can set validateRuntimeValues to false if you don't want to check
// the runtime values such as variables when loading blueprints.
// This is useful when you want to validate a blueprint spec without
// associating it with an instance.
// (e.g. validation for code editors or CLI dry runs)
//
// You can set transformSpec to false if you don't want to apply
// transformers to the blueprint spec.
// This is useful when you want to validate a blueprint spec without
// applying any transformations.
// (e.g. validation for code editors or CLI dry runs)
func NewDefaultLoader(
	providers map[string]provider.Provider,
	specTransformers map[string]transform.SpecTransformer,
	stateContainer state.Container,
	updateChan chan Update,
	validateRuntimeValues bool,
	transformSpec bool,
) Loader {
	return &defaultLoader{
		providers,
		specTransformers,
		stateContainer,
		updateChan,
		validateRuntimeValues,
		transformSpec,
	}
}

func (l *defaultLoader) Load(ctx context.Context, blueprintSpecFile string, params bpcore.BlueprintParams) (BlueprintContainer, error) {
	return l.loadSpecAndLinkInfo(ctx, blueprintSpecFile, params, schema.Load, deriveSpecFormat)
}

func (l *defaultLoader) Validate(ctx context.Context, blueprintSpecFile string, params bpcore.BlueprintParams) (links.SpecLinkInfo, error) {
	container, err := l.loadSpecAndLinkInfo(ctx, blueprintSpecFile, params, schema.Load, deriveSpecFormat)
	if err != nil {
		return nil, err
	}
	return container.SpecLinkInfo(), nil
}

func (l *defaultLoader) loadSpecAndLinkInfo(
	ctx context.Context,
	blueprintSpecOrFilePath string,
	params bpcore.BlueprintParams,
	schemaLoader schema.Loader,
	formatLoader func(string) (schema.SpecFormat, error),
) (BlueprintContainer, error) {
	blueprintSpec, err := l.loadSpec(ctx, blueprintSpecOrFilePath, params, schemaLoader, formatLoader)
	if err != nil {
		return nil, err
	}
	// spew.Fdump(os.Stderr, blueprintSpec)
	resourceProviderMap := l.createResourceProviderMap(blueprintSpec)
	linkInfo, err := links.NewDefaultLinkInfoProvider(resourceProviderMap, blueprintSpec)
	if err != nil {
		return nil, err
	}

	// Once we have loaded the link information,
	// we can check for invalid ${..} reference placeholders
	// throughout the blueprint.
	// This includes cyclic links introduced where a resource selects another through
	// a link and the other resource references a property of the first resource.
	err = l.checkForInvalidReferencePlaceholders(blueprintSpec, linkInfo)
	if err != nil {
		return nil, err
	}

	return NewDefaultBlueprintContainer(l.stateContainer, resourceProviderMap, blueprintSpec, linkInfo, l.updateChan), nil
}

func (l *defaultLoader) checkForInvalidReferencePlaceholders(blueprintSpec speccore.BlueprintSpec, linkInfo links.SpecLinkInfo) error {
	return nil
}

func (l *defaultLoader) createResourceProviderMap(blueprintSpec speccore.BlueprintSpec) map[string]provider.Provider {
	resourceProviderMap := map[string]provider.Provider{}
	for name := range blueprintSpec.Schema().Resources {
		namespace := strings.SplitAfter(name, "/")[0]
		resourceProviderMap[name] = l.providers[namespace]
	}
	return resourceProviderMap
}

func (l *defaultLoader) loadSpec(
	ctx context.Context,
	specOrFilePath string,
	params bpcore.BlueprintParams,
	loader schema.Loader,
	formatLoader func(string) (schema.SpecFormat, error),
) (*internalBlueprintSpec, error) {

	format, err := formatLoader(specOrFilePath)
	if err != nil {
		return nil, err
	}

	blueprintSchema, err := loader(specOrFilePath, format)
	if err != nil {
		return nil, err
	}

	err = l.validateVariables(ctx, blueprintSchema, params)
	if err != nil {
		return nil, err
	}

	err = l.validateIncludes(ctx, blueprintSchema)
	if err != nil {
		return nil, err
	}

	err = l.validateExports(ctx, blueprintSchema)
	if err != nil {
		return nil, err
	}

	if !l.transformSpec {
		return &internalBlueprintSpec{
			schema: blueprintSchema,
		}, nil
	}

	// todo: change l.validateResources to l.validateAbstractResources
	// to limit pre-transform validation to abstract resources provided by
	// transformers only.

	// Validate before transformations to include validation of high-level
	// resources that are expanded by transformers.
	err = l.validateResources(ctx, blueprintSchema, params)
	if err != nil {
		return nil, err
	}

	transformers, err := l.collectTransformers(blueprintSchema)
	if err != nil {
		return nil, err
	}
	for _, transformer := range transformers {
		blueprintSchema, err = transformer.Transform(ctx, blueprintSchema)
		if err != nil {
			return nil, err
		}
	}

	if len(transformers) > 0 {
		// Validate after transformations to help with catching bugs in transformer implementations.
		// This ultimately prevents transformers from expanding their abstractions into invalid
		// representations of the lower level resources.
		err = l.validateResources(ctx, blueprintSchema, params)
		if err != nil {
			return nil, err
		}
	}

	return &internalBlueprintSpec{
		schema: blueprintSchema,
	}, nil
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

func (l *defaultLoader) validateVariables(
	ctx context.Context,
	bpSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
) error {
	if bpSchema.Variables == nil {
		return nil
	}

	// To be as useful as possible, we'll collect and
	// report issues for all the problematic variables.
	variableErrors := map[string]error{}
	for name, varSchema := range bpSchema.Variables.Values {
		if core.SliceContains(schema.CoreVariableTypes, varSchema.Type) {
			err := validation.ValidateCoreVariable(
				ctx, name, varSchema, bpSchema.Variables, params, l.validateRuntimeValues,
			)
			if err != nil {
				variableErrors[name] = err
			}
		} else {
			err := l.validateCustomVariableType(ctx, name, varSchema, bpSchema.Variables, params)
			if err != nil {
				variableErrors[name] = err
			}
		}
	}

	if len(variableErrors) > 0 {
		return errVariableValidationError(variableErrors)
	}

	return nil
}

func (l *defaultLoader) validateIncludes(
	ctx context.Context,
	bpSchema *schema.Blueprint,
) error {
	if bpSchema.Include == nil {
		return nil
	}

	// We'll collect and report issues for all the problematic includes.
	includeErrors := map[string]error{}
	for name, includeSchema := range bpSchema.Include.Values {
		err := validation.ValidateInclude(ctx, name, includeSchema, bpSchema.Include)
		if err != nil {
			includeErrors[name] = err
		}
	}

	if len(includeErrors) > 0 {
		return errIncludeValidationError(includeErrors)
	}

	return nil
}

func (l *defaultLoader) validateExports(
	ctx context.Context,
	bpSchema *schema.Blueprint,
) error {
	// We'll collect and report issues for all the problematic exports.
	exportErrors := map[string]error{}
	for name, exportSchema := range bpSchema.Exports {
		err := validation.ValidateExport(ctx, name, exportSchema)
		if err != nil {
			exportErrors[name] = err
		}
	}

	if len(exportErrors) > 0 {
		return errExportValidationError(exportErrors)
	}

	return nil
}

func (l *defaultLoader) validateCustomVariableType(
	ctx context.Context,
	varName string,
	varSchema *schema.Variable,
	variables *schema.VariableMap,
	params bpcore.BlueprintParams,
) error {
	providerCustomVarType, err := l.deriveProviderCustomVarType(varSchema.Type)
	if err != nil {
		return err
	}
	return validation.ValidateCustomVariable(ctx, varName, varSchema, variables, params, providerCustomVarType)
}

func (l *defaultLoader) deriveProviderCustomVarType(variableType schema.VariableType) (provider.CustomVariableType, error) {
	// The provider should be keyed exactly by \w+\/ which is the custom type prefix.
	// Avoid using a regular expression as it is more efficient to split the string.
	parts := strings.SplitAfter(string(variableType), "/")
	if len(parts) == 0 {
		// return nil, errInvalidCustomVariableType(resourceType)
		return nil, nil
	}

	providerKey := parts[0]

	provider, ok := l.providers[providerKey]
	if !ok {
		// return nil, errMissingProviderForCustomVarType(providerKey, variableType)
		return nil, nil
	}

	customVarType := provider.CustomVariableType(string(variableType))
	if !ok {
		// return nil, errMissingCustomVariableType(providerKey, variableType)
		return nil, nil
	}

	return customVarType, nil
}

func (l *defaultLoader) validateResources(
	ctx context.Context,
	blueprintSchema *schema.Blueprint,
	params bpcore.BlueprintParams,
) error {
	// To be as useful as possible, we'll collect and
	// report issues for all the problematic resources.
	// resourceErrors := map[string]error{}
	// internalResourceSpecs := map[string]speccore.ResourceSchemaSpec{}
	// for name, resourceSchema := range blueprintSchema.Resources {
	// 	resourceConcreteSpec, err := l.validateResource(ctx, resourceSchema, params)
	// 	if err != nil {
	// 		resourceErrors[name] = err
	// 	} else {
	// 		internalResourceSpecs[name] = speccore.ResourceSchemaSpec{
	// 			Schema: resourceSchema,
	// 			Spec:   resourceConcreteSpec,
	// 		}
	// 	}
	// }

	// if len(resourceErrors) > 0 {
	// 	return nil, errResourceValidationError(resourceErrors)
	// }

	// return internalResourceSpecs, nil
	return nil
}

// func (l *defaultLoader) validateResource(
// 	ctx context.Context, resourceSchema *schema.Resource, params bpcore.BlueprintParams,
// ) error {
// 	providerResource, err := l.deriveProviderResource(resourceSchema.Type)
// 	if err != nil {
// 		return err
// 	}
// 	return providerResource.Validate(ctx, resourceSchema, params)
// }

// func (l *defaultLoader) deriveProviderResource(resourceType string) (provider.Resource, error) {
// 	// The provider should be keyed exactly by \w+\/ which is the resource prefix.
// 	// Avoid using a regular expression as it is more efficient to split the string.
// 	parts := strings.SplitAfter(resourceType, "/")
// 	if len(parts) == 0 {
// 		return nil, errInvalidResourceType(resourceType)
// 	}

// 	providerKey := parts[0]

// 	provider, ok := l.providers[providerKey]
// 	if !ok {
// 		return nil, errMissingProvider(providerKey, resourceType)
// 	}

// 	providerResource := provider.Resource(resourceType)
// 	if !ok {
// 		return nil, errMissingResource(providerKey, resourceType)
// 	}

// 	return providerResource, nil
// }

func (l *defaultLoader) LoadString(
	ctx context.Context, blueprintSpec string, inputFormat schema.SpecFormat, params bpcore.BlueprintParams,
) (BlueprintContainer, error) {
	return l.loadSpecAndLinkInfo(ctx, blueprintSpec, params, schema.LoadString, predefinedFormatFactory(inputFormat))
}

func (l *defaultLoader) ValidateString(
	ctx context.Context, blueprintSpec string, inputFormat schema.SpecFormat, params bpcore.BlueprintParams,
) (links.SpecLinkInfo, error) {

	container, err := l.loadSpecAndLinkInfo(ctx, blueprintSpec, params, schema.LoadString, predefinedFormatFactory(inputFormat))
	if err != nil {
		return nil, err
	}
	return container.SpecLinkInfo(), nil
}
