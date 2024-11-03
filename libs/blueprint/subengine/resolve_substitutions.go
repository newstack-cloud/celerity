package subengine

import (
	"context"
	"fmt"
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/resourcehelpers"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/speccore"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
)

// SubstitutionResolver is an interface that provides methods to resolve
// substitutions in components of a blueprint specification.
// Resolving involves taking a parsed representation of a substitution, resolving referenced
// values and executing functions to produce a final output.
type SubstitutionResolver interface {
	// ResolveInResource resolves substitutions in a resource.
	// The index parameter is used to select the index of the resource to resolve
	// when the resource is a template (i.e. has an `each` property).
	// For resource definitions that are not templates, the index should always be 0.
	ResolveInResource(
		ctx context.Context,
		resourceName string,
		resource *schema.Resource,
		index int,
	) (*provider.ResolvedResource, error)
	// ResolveResourceEach resolves the substitution in the `each` property of a resource
	// that is expected to resolve to a list of items that will be mapped to a planned and
	// eventually deployed resource.
	ResolveResourceEach(
		ctx context.Context,
		resourceName string,
		resource *schema.Resource,
	) ([]*core.MappingNode, error)
	// ResolveInDataSource resolves substitutions in a data source.
	ResolveInDataSource(
		ctx context.Context,
		dataSourceName string,
		dataSource *schema.DataSource,
	) (*provider.ResolvedDataSource, error)
	// ResolveInMappingNode resolves substitutions in a mapping node, primarily used
	// for the top-level blueprint metadata.
	ResolveInMappingNode(
		ctx context.Context,
		currentElementName string,
		mappingNode *core.MappingNode,
	) (*core.MappingNode, error)
	// ResolveInValue resolves substitutions in a value.
	ResolveInValue(
		ctx context.Context,
		valueName string,
		value *schema.Value,
	) (*ResolvedValue, error)
	// ResolveInInclude resolves substitutions in an include.
	ResolveInInclude(
		ctx context.Context,
		includeName string,
		include *schema.Include,
	) (*ResolvedInclude, error)
	// ResolveInExport resolves substitutions in an export.
	ResolveInExport(
		ctx context.Context,
		exportName string,
		export *schema.Export,
	) (*ResolvedExport, error)
}

type defaultSubstitutionResolver struct {
	funcRegistry        provider.FunctionRegistry
	resourceRegistry    resourcehelpers.Registry
	dataSourceRegistry  provider.DataSourceRegistry
	stateContainer      state.Container
	spec                speccore.BlueprintSpec
	params              core.BlueprintParams
	valueCache          *core.Cache[*ResolvedValue]
	dataSourceCache     *core.Cache[*provider.ResolvedDataSource]
	dataSourceDataCache *core.Cache[map[string]*core.MappingNode]
	resourceCache       *core.Cache[[]*provider.ResolvedResource]
	resourceStateCache  *core.Cache[*state.ResourceState]
}

// NewDefaultSubstitutionResolver creates a new default implementation
// of a substitution resolver.
func NewDefaultSubstitutionResolver(
	funcRegistry provider.FunctionRegistry,
	resourceRegistry resourcehelpers.Registry,
	dataSourceRegistry provider.DataSourceRegistry,
	stateContainer state.Container,
	// The resource cache is passed down from the container as resources
	// are resolved before references to them are resolved.
	// The substitution resolver can safely assume that ordering is taken care of
	// so that a resolved resource is available when needed.
	resourceCache *core.Cache[[]*provider.ResolvedResource],
	spec speccore.BlueprintSpec,
	params core.BlueprintParams,
) SubstitutionResolver {
	return &defaultSubstitutionResolver{
		funcRegistry:        funcRegistry,
		resourceRegistry:    resourceRegistry,
		dataSourceRegistry:  dataSourceRegistry,
		stateContainer:      stateContainer,
		spec:                spec,
		params:              params,
		valueCache:          core.NewCache[*ResolvedValue](),
		dataSourceCache:     core.NewCache[*provider.ResolvedDataSource](),
		dataSourceDataCache: core.NewCache[map[string]*core.MappingNode](),
		resourceCache:       resourceCache,
		resourceStateCache:  core.NewCache[*state.ResourceState](),
	}
}

func (r *defaultSubstitutionResolver) ResolveResourceEach(
	ctx context.Context,
	resourceName string,
	resource *schema.Resource,
) ([]*core.MappingNode, error) {
	elementName := fmt.Sprintf("resources.%s", resourceName)
	eachResolved, err := r.resolveSubstitutions(ctx, elementName, resource.Each)
	if err != nil {
		return nil, err
	}

	isArray := mappingNodeIsArray(eachResolved)
	if isArray && len(eachResolved.Items) == 0 {
		return nil, errEmptyResourceEach(elementName, resourceName)
	} else if !isArray {
		return nil, errResourceEachNotArray(elementName, resourceName, eachResolved)
	}

	return eachResolved.Items, nil
}

func (r *defaultSubstitutionResolver) ResolveInResource(
	ctx context.Context,
	resourceName string,
	resource *schema.Resource,
	index int,
) (*provider.ResolvedResource, error) {
	elementName := fmt.Sprintf("resources.%s", resourceName)
	resolvedDescription, err := r.resolveInDescription(ctx, elementName, resource.Description)
	if err != nil {
		return nil, err
	}

	return &provider.ResolvedResource{
		Type:         resource.Type,
		Description:  resolvedDescription,
		LinkSelector: resource.LinkSelector,
	}, nil
}

func (r *defaultSubstitutionResolver) resolveInDescription(
	ctx context.Context,
	elementName string,
	description *substitutions.StringOrSubstitutions,
) (*core.MappingNode, error) {
	if description == nil {
		return nil, nil
	}

	if isStringLiteral(description) {
		return &core.MappingNode{
			Literal: &core.ScalarValue{
				StringValue: description.Values[0].StringValue,
			},
		}, nil
	}

	return r.resolveSubstitutions(ctx, elementName, description)
}

func (r *defaultSubstitutionResolver) ResolveInDataSource(
	ctx context.Context,
	dataSourceName string,
	dataSource *schema.DataSource,
) (*provider.ResolvedDataSource, error) {
	return nil, nil
}

func (r *defaultSubstitutionResolver) ResolveInMappingNode(
	ctx context.Context,
	currentElementName string,
	mappingNode *core.MappingNode,
) (*core.MappingNode, error) {
	return nil, nil
}

func (r *defaultSubstitutionResolver) ResolveInValue(
	ctx context.Context,
	valueName string,
	value *schema.Value,
) (*ResolvedValue, error) {
	return nil, nil
}

func (r *defaultSubstitutionResolver) ResolveInInclude(
	ctx context.Context,
	includeName string,
	include *schema.Include,
) (*ResolvedInclude, error) {
	return nil, nil
}

func (r *defaultSubstitutionResolver) ResolveInExport(
	ctx context.Context,
	exportName string,
	export *schema.Export,
) (*ResolvedExport, error) {
	return nil, nil
}

func (r *defaultSubstitutionResolver) resolveSubstitutions(
	ctx context.Context,
	elementName string,
	stringOrSubs *substitutions.StringOrSubstitutions,
) (*core.MappingNode, error) {

	isStringInterpolation := len(stringOrSubs.Values) > 1
	if !isStringInterpolation {
		// A scoped function registry with a call stack created for
		// the substitution.
		scopedFunctionRegistry := r.funcRegistry.ForCallContext()
		return r.resolveSubstitution(
			ctx,
			elementName,
			stringOrSubs.Values[0],
			scopedFunctionRegistry,
		)
	}

	sb := &strings.Builder{}
	for _, value := range stringOrSubs.Values {
		// A scoped function registry with a call stack created for
		// each individual substitution.
		scopedFunctionRegistry := r.funcRegistry.ForCallContext()
		resolvedValue, err := r.resolveSubstitution(ctx, elementName, value, scopedFunctionRegistry)
		if err != nil {
			return nil, err
		}

		if stringValue, err := resolvedValueToString(resolvedValue); err == nil {
			sb.WriteString(stringValue)
		} else {
			return nil, errInvalidInterpolationSubType(elementName, resolvedValue)
		}
	}

	resolvedStr := sb.String()
	return &core.MappingNode{
		Literal: &core.ScalarValue{
			StringValue: &resolvedStr,
		},
	}, nil
}

func (r *defaultSubstitutionResolver) resolveSubstitution(
	ctx context.Context,
	elementName string,
	value *substitutions.StringOrSubstitution,
	scopedFunctionRegistry provider.FunctionRegistry,
) (*core.MappingNode, error) {
	if value.StringValue != nil {
		return &core.MappingNode{
			Literal: &core.ScalarValue{
				StringValue: value.StringValue,
			},
		}, nil
	}

	if value.SubstitutionValue != nil {
		return r.resolveSubstitutionValue(
			ctx,
			elementName,
			value.SubstitutionValue,
			scopedFunctionRegistry,
		)
	}

	return nil, errEmptySubstitutionValue(elementName)
}

func (r *defaultSubstitutionResolver) resolveSubstitutionValue(
	ctx context.Context,
	elementName string,
	substitutionValue *substitutions.Substitution,
	scopedFunctionRegistry provider.FunctionRegistry,
) (*core.MappingNode, error) {
	if substitutionValue.StringValue != nil {
		return &core.MappingNode{
			Literal: &core.ScalarValue{
				StringValue: substitutionValue.StringValue,
			},
		}, nil
	}

	if substitutionValue.IntValue != nil {
		intVal := int(*substitutionValue.IntValue)
		return &core.MappingNode{
			Literal: &core.ScalarValue{
				IntValue: &intVal,
			},
		}, nil
	}

	if substitutionValue.FloatValue != nil {
		return &core.MappingNode{
			Literal: &core.ScalarValue{
				FloatValue: substitutionValue.FloatValue,
			},
		}, nil
	}

	if substitutionValue.BoolValue != nil {
		return &core.MappingNode{
			Literal: &core.ScalarValue{
				BoolValue: substitutionValue.BoolValue,
			},
		}, nil
	}

	if substitutionValue.Variable != nil {
		return r.resolveVariable(elementName, substitutionValue.Variable)
	}

	if substitutionValue.ValueReference != nil {
		return r.resolveValue(
			ctx,
			elementName,
			substitutionValue.ValueReference,
		)
	}

	if substitutionValue.DataSourceProperty != nil {
		return r.resolveDataSourceProperty(
			ctx,
			elementName,
			substitutionValue.DataSourceProperty,
		)
	}

	if substitutionValue.ResourceProperty != nil {
		return r.resolveResourceProperty(
			ctx,
			elementName,
			substitutionValue.ResourceProperty,
		)
	}

	return nil, errEmptySubstitutionValue(elementName)
}

func (r *defaultSubstitutionResolver) resolveVariable(
	elementName string,
	variable *substitutions.SubstitutionVariable,
) (*core.MappingNode, error) {

	varValue := r.params.BlueprintVariable(variable.VariableName)
	if varValue == nil {
		specVar := getVariable(variable.VariableName, r.spec.Schema())
		if specVar == nil {
			return nil, errMissingVariable(elementName, variable.VariableName)
		}

		if specVar.Default != nil {
			return &core.MappingNode{
				Literal: specVar.Default,
			}, nil
		}
	}

	return &core.MappingNode{
		Literal: varValue,
	}, nil
}

func (r *defaultSubstitutionResolver) resolveValue(
	ctx context.Context,
	elementName string,
	value *substitutions.SubstitutionValueReference,
) (*core.MappingNode, error) {
	cached, hasValue := r.valueCache.Get(value.ValueName)
	if hasValue {
		return cached.Value, nil
	}

	computed, err := r.computeValue(ctx, elementName, value)
	if err != nil {
		return nil, err
	}

	r.valueCache.Set(value.ValueName, computed)

	return computed.Value, nil
}

func (r *defaultSubstitutionResolver) computeValue(
	ctx context.Context,
	elementName string,
	value *substitutions.SubstitutionValueReference,
) (*ResolvedValue, error) {
	valueSpec := getValue(value.ValueName, r.spec.Schema())
	if valueSpec == nil {
		return nil, errMissingValue(elementName, value.ValueName)
	}

	return r.ResolveInValue(ctx, value.ValueName, valueSpec)
}

func (r *defaultSubstitutionResolver) resolveDataSourceProperty(
	ctx context.Context,
	elementName string,
	dataSourceProperty *substitutions.SubstitutionDataSourceProperty,
) (*core.MappingNode, error) {
	cached, hasValue := r.dataSourceDataCache.Get(dataSourceProperty.DataSourceName)
	if hasValue {
		return extractDataSourceProperty(elementName, cached, dataSourceProperty)
	}

	resolvedDataSource, err := r.resolveDataSource(ctx, elementName, dataSourceProperty)
	if err != nil {
		return nil, err
	}

	dataOutput, err := r.dataSourceRegistry.Fetch(
		ctx,
		resolvedDataSource.Type.Value,
		&provider.DataSourceFetchInput{
			DataSourceWithResolvedSubs: resolvedDataSource,
			Params:                     r.params,
		},
	)
	if err != nil {
		return nil, err
	}

	r.dataSourceDataCache.Set(dataSourceProperty.DataSourceName, dataOutput.Data)

	return extractDataSourceProperty(elementName, cached, dataSourceProperty)
}

func extractDataSourceProperty(
	parentElementName string,
	data map[string]*core.MappingNode,
	prop *substitutions.SubstitutionDataSourceProperty,
) (*core.MappingNode, error) {
	if data == nil {
		return nil, errEmptyDataSourceData(parentElementName, prop.DataSourceName)
	}

	value, hasValue := data[prop.FieldName]
	if !hasValue {
		return nil, errMissingDataSourceProperty(parentElementName, prop.DataSourceName, prop.FieldName)
	}

	finalValue := value
	if prop.PrimitiveArrIndex != nil {
		if value.Items == nil {
			return nil, errDataSourcePropNotArray(parentElementName, prop.DataSourceName, prop.FieldName)
		}

		if int(*prop.PrimitiveArrIndex) >= len(value.Items) {
			return nil, errDataSourcePropArrayIndexOutOfBounds(
				parentElementName,
				prop.DataSourceName,
				prop.FieldName,
				int(*prop.PrimitiveArrIndex),
			)
		}

		finalValue = value.Items[*prop.PrimitiveArrIndex]
	}

	return finalValue, nil
}

func (r *defaultSubstitutionResolver) resolveDataSource(
	ctx context.Context,
	elementName string,
	prop *substitutions.SubstitutionDataSourceProperty,
) (*provider.ResolvedDataSource, error) {
	cached, hasDataSource := r.dataSourceCache.Get(prop.DataSourceName)
	if hasDataSource {
		return cached, nil
	}

	computed, err := r.computeDataSource(ctx, elementName, prop)
	if err != nil {
		return nil, err
	}

	r.dataSourceCache.Set(prop.DataSourceName, computed)

	return computed, nil
}

func (r *defaultSubstitutionResolver) computeDataSource(
	ctx context.Context,
	elementName string,
	prop *substitutions.SubstitutionDataSourceProperty,
) (*provider.ResolvedDataSource, error) {
	dataSourceSpec := getDataSource(prop.DataSourceName, r.spec.Schema())
	if dataSourceSpec == nil {
		return nil, errMissingDataSource(elementName, prop.DataSourceName)
	}

	return r.ResolveInDataSource(ctx, prop.DataSourceName, dataSourceSpec)
}

func (r *defaultSubstitutionResolver) resolveResourceProperty(
	ctx context.Context,
	elementName string,
	resourceProperty *substitutions.SubstitutionResourceProperty,
) (*core.MappingNode, error) {
	if len(resourceProperty.Path) > 1 && resourceProperty.Path[0].FieldName == "spec" {
		return r.resolveResourceSpecProperty(ctx, elementName, resourceProperty)
	}

	// if len(resourceProperty.Path) > 1 && resourceProperty.Path[0].FieldName == "metadata" {
	// 	return r.resolveResourceMetadataProperty(ctx, elementName, resourceProperty)
	// }

	// if len(resourceProperty.Path) == 0 {
	// 	return r.resolveResourceID(ctx, elementName, resourceProperty)
	// }

	renderedPath, err := substitutions.SubResourcePropertyToString(resourceProperty)
	if err != nil {
		return nil, err
	}

	return nil, errInvalidResourcePropertyPath(elementName, renderedPath)
}

func (r *defaultSubstitutionResolver) resolveResourceSpecProperty(
	ctx context.Context,
	elementName string,
	prop *substitutions.SubstitutionResourceProperty,
) (*core.MappingNode, error) {
	resources, hasResource := r.resourceCache.Get(prop.ResourceName)
	if !hasResource || len(resources) == 0 {
		return nil, errResourceNotResolved(elementName, prop.ResourceName)
	}

	current, err := selectResourceForProperty(elementName, prop, resources)
	if err != nil {
		return nil, err
	}

	pathExists := true
	i := 1
	for pathExists && current != nil && i < len(prop.Path) {
		pathItem := prop.Path[i]
		if pathItem.FieldName != "" && current.Fields != nil {
			current = current.Fields[pathItem.FieldName]
		} else if pathItem.ArrayIndex != nil &&
			len(current.Items) > int(*pathItem.ArrayIndex) {
			current = current.Items[*pathItem.ArrayIndex]
		} else {
			pathExists = false
		}

		i += 1
	}

	if !pathExists || current == nil {
		renderedPath, err := substitutions.SubResourcePropertyToString(prop)
		if err != nil {
			return nil, err
		}

		return nil, errInvalidResourcePropertyPath(elementName, renderedPath)
	}

	return current, nil
}

func selectResourceForProperty(
	parentElementName string,
	prop *substitutions.SubstitutionResourceProperty,
	resources []*provider.ResolvedResource,
) (*core.MappingNode, error) {
	current := resources[0].Spec
	if prop.ResourceEachTemplateIndex != nil {
		if len(resources) <= int(*prop.ResourceEachTemplateIndex) {
			return nil, errResourceEachIndexOutOfBounds(
				parentElementName,
				prop.ResourceName,
				int(*prop.ResourceEachTemplateIndex),
			)
		}

		current = resources[*prop.ResourceEachTemplateIndex].Spec
	}

	return current, nil
}

func getVariable(
	variableName string,
	schema *schema.Blueprint,
) *schema.Variable {
	if schema.Variables == nil {
		return nil
	}

	return schema.Variables.Values[variableName]
}

func getValue(
	valueName string,
	schema *schema.Blueprint,
) *schema.Value {
	if schema.Values == nil {
		return nil
	}

	return schema.Values.Values[valueName]
}

func getDataSource(
	valueName string,
	schema *schema.Blueprint,
) *schema.DataSource {
	if schema.DataSources == nil {
		return nil
	}

	return schema.DataSources.Values[valueName]
}

func isStringLiteral(s *substitutions.StringOrSubstitutions) bool {
	return len(s.Values) == 1 && s.Values[0].StringValue != nil
}

func resolvedValueToString(
	value *core.MappingNode,
) (string, error) {
	if value.Literal == nil {
		return "", fmt.Errorf("only literal values can be converted to a string")
	}

	if value.Literal.StringValue != nil {
		return *value.Literal.StringValue, nil
	}

	if value.Literal.IntValue != nil {
		return fmt.Sprintf("%d", *value.Literal.IntValue), nil
	}

	if value.Literal.FloatValue != nil {
		return fmt.Sprintf("%f", *value.Literal.FloatValue), nil
	}

	if value.Literal.BoolValue != nil {
		return fmt.Sprintf("%t", *value.Literal.BoolValue), nil
	}

	return "", fmt.Errorf("expected a scalar string, int, float or bool value")
}

func mappingNodeIsArray(node *core.MappingNode) bool {
	return node.Items != nil
}
