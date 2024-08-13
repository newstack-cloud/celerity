package serialisation

import (
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/schemapb"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	"google.golang.org/protobuf/proto"
)

func (s *ProtobufExpandedBlueprintSerialiser) Unmarshal(data []byte) (*schema.Blueprint, error) {
	blueprintPB := &schemapb.Blueprint{}
	err := proto.Unmarshal(data, blueprintPB)
	if err != nil {
		return nil, err
	}

	return fromSchemaPB(blueprintPB)
}

func fromSchemaPB(blueprintPB *schemapb.Blueprint) (*schema.Blueprint, error) {
	variables, err := fromVariablesPB(blueprintPB.Variables)
	if err != nil {
		return nil, err
	}

	includes, err := fromIncludesPB(blueprintPB.Include)
	if err != nil {
		return nil, err
	}

	resources, err := fromResourcesPB(blueprintPB.Resources)
	if err != nil {
		return nil, err
	}

	dataSources, err := fromDataSourcesPB(blueprintPB.DataSources)
	if err != nil {
		return nil, err
	}

	exports, err := fromExportsPB(blueprintPB.Exports)
	if err != nil {
		return nil, err
	}

	metadata, err := fromMappingNodePB(blueprintPB.Metadata, true)
	if err != nil {
		return nil, err
	}

	transform := (*schema.TransformValueWrapper)(nil)
	if blueprintPB.Transform != nil && len(blueprintPB.Transform) > 0 {
		transform = &schema.TransformValueWrapper{Values: blueprintPB.Transform}
	}

	return &schema.Blueprint{
		Version:     blueprintPB.Version,
		Transform:   transform,
		Variables:   variables,
		Include:     includes,
		Resources:   resources,
		DataSources: dataSources,
		Exports:     exports,
		Metadata:    metadata,
	}, nil
}

func fromVariablesPB(variablesPB map[string]*schemapb.Variable) (*schema.VariableMap, error) {
	if variablesPB == nil {
		return nil, nil
	}

	var variables = make(map[string]*schema.Variable)
	for k, v := range variablesPB {
		defaultValue, err := fromScalarValuePB(v.Default, true)
		if err != nil {
			return nil, err
		}

		allowedValues, err := fromScalarValuesPB(v.AllowedValues)
		if err != nil {
			return nil, err
		}

		variables[k] = &schema.Variable{
			Type:          schema.VariableType(v.Type),
			Description:   *v.Description,
			Secret:        v.Secret,
			Default:       defaultValue,
			AllowedValues: allowedValues,
		}
	}

	return &schema.VariableMap{
		Values: variables,
	}, nil
}

func fromIncludesPB(includesPB map[string]*schemapb.Include) (*schema.IncludeMap, error) {
	if includesPB == nil {
		return nil, nil
	}

	var includes = make(map[string]*schema.Include)
	for k, v := range includesPB {
		path, err := fromStringOrSubstitutionsPB(v.Path, false)
		if err != nil {
			return nil, err
		}

		variables, err := fromMappingNodePB(v.Variables, false)
		if err != nil {
			return nil, err
		}

		metadata, err := fromMappingNodePB(v.Metadata, false)
		if err != nil {
			return nil, err
		}

		description, err := fromStringOrSubstitutionsPB(v.Description, true)
		if err != nil {
			return nil, err
		}

		includes[k] = &schema.Include{
			Path:        path,
			Variables:   variables,
			Metadata:    metadata,
			Description: description,
		}
	}

	return &schema.IncludeMap{
		Values: includes,
	}, nil
}

func fromResourcesPB(resourcesPB map[string]*schemapb.Resource) (*schema.ResourceMap, error) {
	if resourcesPB == nil {
		return nil, nil
	}

	resources := make(map[string]*schema.Resource)
	for k, v := range resourcesPB {
		resource, err := fromResourcePB(v)
		if err != nil {
			return nil, err
		}

		resources[k] = resource
	}

	return &schema.ResourceMap{
		Values: resources,
	}, nil
}

func fromDataSourcesPB(dataSourcesPB map[string]*schemapb.DataSource) (*schema.DataSourceMap, error) {
	if dataSourcesPB == nil {
		return nil, nil
	}

	dataSources := make(map[string]*schema.DataSource)
	for k, v := range dataSourcesPB {
		dataSource, err := fromDataSourcePB(v)
		if err != nil {
			return nil, err
		}

		dataSources[k] = dataSource
	}

	return &schema.DataSourceMap{
		Values: dataSources,
	}, nil
}

func fromDataSourcePB(dataSourcePB *schemapb.DataSource) (*schema.DataSource, error) {
	description, err := fromStringOrSubstitutionsPB(dataSourcePB.Description, true)
	if err != nil {
		return nil, err
	}

	metadata, err := fromDataSourceMetadataPB(dataSourcePB.Metadata)
	if err != nil {
		return nil, err
	}

	filter, err := fromDataSourceFilterPB(dataSourcePB.Filter)
	if err != nil {
		return nil, err
	}

	exports, err := fromDataSourceFieldExports(dataSourcePB.Exports)
	if err != nil {
		return nil, err
	}

	return &schema.DataSource{
		Type:               dataSourcePB.Type,
		DataSourceMetadata: metadata,
		Filter:             filter,
		Exports:            exports,
		Description:        description,
	}, nil
}

func fromDataSourceMetadataPB(metadataPB *schemapb.DataSourceMetadata) (*schema.DataSourceMetadata, error) {
	displayName, err := fromStringOrSubstitutionsPB(metadataPB.DisplayName, true)
	if err != nil {
		return nil, err
	}

	annotations, err := fromAnnotationsPB(metadataPB.Annotations)
	if err != nil {
		return nil, err
	}

	custom, err := fromMappingNodePB(metadataPB.Custom, true)
	if err != nil {
		return nil, err
	}

	return &schema.DataSourceMetadata{
		DisplayName: displayName,
		Annotations: annotations,
		Custom:      custom,
	}, nil
}

func fromDataSourceFilterPB(filterPB *schemapb.DataSourceFilter) (*schema.DataSourceFilter, error) {
	search, err := fromDataSourceFilterSearchPB(filterPB.Search)
	if err != nil {
		return nil, err
	}

	return &schema.DataSourceFilter{
		Field: filterPB.Field,
		Operator: &schema.DataSourceFilterOperatorWrapper{
			Value: schema.DataSourceFilterOperator(filterPB.Operator),
		},
		Search: search,
	}, nil
}

func fromDataSourceFilterSearchPB(
	searchPB *schemapb.DataSourceFilterSearch,
) (*schema.DataSourceFilterSearch, error) {
	values := make([]*substitutions.StringOrSubstitutions, len(searchPB.Values))
	for i, v := range searchPB.Values {
		value, err := fromStringOrSubstitutionsPB(v, false)
		if err != nil {
			return nil, err
		}

		values[i] = value
	}

	return &schema.DataSourceFilterSearch{
		Values: values,
	}, nil
}

func fromDataSourceFieldExports(
	exportsPB map[string]*schemapb.DataSourceFieldExport,
) (*schema.DataSourceFieldExportMap, error) {
	if exportsPB == nil {
		return nil, nil
	}

	exports := make(map[string]*schema.DataSourceFieldExport)
	for k, v := range exportsPB {
		export, err := fromDataSourceFieldExportPB(v)
		if err != nil {
			return nil, err
		}

		exports[k] = export
	}

	return &schema.DataSourceFieldExportMap{
		Values: exports,
	}, nil
}

func fromDataSourceFieldExportPB(
	exportPB *schemapb.DataSourceFieldExport,
) (*schema.DataSourceFieldExport, error) {
	description, err := fromStringOrSubstitutionsPB(exportPB.Description, true)
	if err != nil {
		return nil, err
	}

	return &schema.DataSourceFieldExport{
		Type: &schema.DataSourceFieldTypeWrapper{
			Value: schema.DataSourceFieldType(exportPB.Type),
		},
		AliasFor:    exportPB.AliasFor,
		Description: description,
	}, nil
}

func fromResourcePB(resourcePB *schemapb.Resource) (*schema.Resource, error) {
	description, err := fromStringOrSubstitutionsPB(resourcePB.Description, true)
	if err != nil {
		return nil, err
	}

	resourceMetadata, err := fromResourceMetadataPB(resourcePB.Metadata)
	if err != nil {
		return nil, err
	}

	spec, err := fromMappingNodePB(resourcePB.Spec, false)
	if err != nil {
		return nil, err
	}

	return &schema.Resource{
		Type:         resourcePB.Type,
		Description:  description,
		Metadata:     resourceMetadata,
		LinkSelector: fromLinkSelectorPB(resourcePB.LinkSelector),
		Spec:         spec,
	}, nil
}

func fromResourceMetadataPB(metadataPB *schemapb.ResourceMetadata) (*schema.Metadata, error) {
	if metadataPB == nil {
		return nil, nil
	}

	displayName, err := fromStringOrSubstitutionsPB(metadataPB.DisplayName, true)
	if err != nil {
		return nil, err
	}

	annotations, err := fromAnnotationsPB(metadataPB.Annotations)
	if err != nil {
		return nil, err
	}

	custom, err := fromMappingNodePB(metadataPB.Custom, true)
	if err != nil {
		return nil, err
	}

	labels := (*schema.StringMap)(nil)
	if metadataPB.Labels != nil {
		labels = &schema.StringMap{
			Values: metadataPB.Labels,
		}
	}

	return &schema.Metadata{
		DisplayName: displayName,
		Annotations: annotations,
		Labels:      labels,
		Custom:      custom,
	}, nil
}

func fromAnnotationsPB(
	annotationsPB map[string]*schemapb.StringOrSubstitutions,
) (*schema.StringOrSubstitutionsMap, error) {
	if annotationsPB == nil {
		return nil, nil
	}

	annotations := make(map[string]*substitutions.StringOrSubstitutions)
	for k, v := range annotationsPB {
		stringOrSubs, err := fromStringOrSubstitutionsPB(v, false)
		if err != nil {
			return nil, err
		}

		annotations[k] = stringOrSubs
	}

	return &schema.StringOrSubstitutionsMap{
		Values: annotations,
	}, nil
}

func fromExportsPB(exportsPB map[string]*schemapb.Export) (*schema.ExportMap, error) {
	if exportsPB == nil {
		return nil, nil
	}

	exports := make(map[string]*schema.Export)
	for k, v := range exportsPB {
		export, err := fromExportPB(v)
		if err != nil {
			return nil, err
		}

		exports[k] = export
	}

	return &schema.ExportMap{
		Values: exports,
	}, nil
}

func fromLinkSelectorPB(linkSelectorPB *schemapb.LinkSelector) *schema.LinkSelector {
	if linkSelectorPB == nil {
		return nil
	}

	return &schema.LinkSelector{
		ByLabel: &schema.StringMap{
			Values: linkSelectorPB.ByLabel,
		},
	}
}

func fromExportPB(exportPB *schemapb.Export) (*schema.Export, error) {
	description, err := fromStringOrSubstitutionsPB(exportPB.Description, true)
	if err != nil {
		return nil, err
	}

	return &schema.Export{
		Type:        schema.ExportType(exportPB.Type),
		Field:       exportPB.Field,
		Description: description,
	}, nil
}

func fromMappingNodePB(mappingNodePB *schemapb.MappingNode, optional bool) (*core.MappingNode, error) {
	if optional && mappingNodePB == nil {
		return nil, nil
	}

	if !optional && mappingNodePB == nil {
		return nil, errMappingNodeIsNil()
	}

	if mappingNodePB.Literal != nil {
		scalar, err := fromScalarValuePB(mappingNodePB.Literal, false)
		if err != nil {
			return nil, err
		}

		return &core.MappingNode{
			Literal: scalar,
		}, nil
	}

	if mappingNodePB.Fields != nil {
		return fromMappingNodeFieldsPB(mappingNodePB.Fields)
	}

	if mappingNodePB.Items != nil {
		return fromMappingNodeItemsPB(mappingNodePB.Items)
	}

	if mappingNodePB.StringWithSubstitutions != nil {
		stringOrSubs, err := fromStringOrSubstitutionsPB(mappingNodePB.StringWithSubstitutions, false)
		if err != nil {
			return nil, err
		}

		return &core.MappingNode{
			StringWithSubstitutions: stringOrSubs,
		}, nil
	}

	return nil, errMissingMappingNodeValue()
}

func fromMappingNodeFieldsPB(fieldsPB map[string]*schemapb.MappingNode) (*core.MappingNode, error) {
	fields := make(map[string]*core.MappingNode)
	for k, v := range fieldsPB {
		mappingNode, err := fromMappingNodePB(v, true)
		if err != nil {
			return nil, err
		}

		fields[k] = mappingNode
	}

	return &core.MappingNode{
		Fields: fields,
	}, nil
}

func fromMappingNodeItemsPB(itemsPB []*schemapb.MappingNode) (*core.MappingNode, error) {
	items := make([]*core.MappingNode, len(itemsPB))
	for i, item := range itemsPB {
		mappingNode, err := fromMappingNodePB(item, true)
		if err != nil {
			return nil, err
		}

		items[i] = mappingNode
	}

	return &core.MappingNode{
		Items: items,
	}, nil
}

func fromStringOrSubstitutionsPB(
	stringOrSubstitutionsPB *schemapb.StringOrSubstitutions,
	optional bool,
) (*substitutions.StringOrSubstitutions, error) {
	if optional && stringOrSubstitutionsPB == nil {
		return nil, nil
	}

	if !optional && stringOrSubstitutionsPB == nil {
		return nil, errStringOrSubstitutionsIsNil()
	}

	stringOrSubs := make([]*substitutions.StringOrSubstitution, len(stringOrSubstitutionsPB.Values))
	for i, stringOrSub := range stringOrSubstitutionsPB.Values {
		stringOrSub, err := fromStringOrSubstitutionPB(stringOrSub)
		if err != nil {
			return nil, err
		}
		stringOrSubs[i] = stringOrSub
	}
	return &substitutions.StringOrSubstitutions{
		Values: stringOrSubs,
	}, nil
}

func fromStringOrSubstitutionPB(
	stringOrSubstitutionPB *schemapb.StringOrSubstitution,
) (*substitutions.StringOrSubstitution, error) {
	if strVal, isStr := stringOrSubstitutionPB.Value.(*schemapb.StringOrSubstitution_StringValue); isStr {
		return &substitutions.StringOrSubstitution{
			StringValue: &strVal.StringValue,
		}, nil
	}

	if subVal, isSub := stringOrSubstitutionPB.Value.(*schemapb.StringOrSubstitution_SubstitutionValue); isSub {
		substitution, err := fromSubstitutionPB(subVal.SubstitutionValue)
		if err != nil {
			return nil, err
		}

		return &substitutions.StringOrSubstitution{
			SubstitutionValue: substitution,
		}, nil
	}

	return nil, errMissingStringOrSubstitutionValue()
}

func fromSubstitutionsPB(substitutionsPB []*schemapb.Substitution) ([]*substitutions.Substitution, error) {
	var schemaSubstitutions = make([]*substitutions.Substitution, len(substitutionsPB))
	for i, s := range substitutionsPB {
		substitution, err := fromSubstitutionPB(s)
		if err != nil {
			return nil, err
		}

		schemaSubstitutions[i] = substitution
	}
	return schemaSubstitutions, nil
}

func fromSubstitutionPB(substitution *schemapb.Substitution) (*substitutions.Substitution, error) {
	if funcVal, isFunc := substitution.Sub.(*schemapb.Substitution_Function); isFunc {
		return fromSubstitutionFunctionPB(funcVal.Function)
	}

	if varVal, isVar := substitution.Sub.(*schemapb.Substitution_Variable); isVar {
		return fromSubstitutionVariablePB(varVal.Variable)
	}

	if dataSourceProp, isDataSourceProp := substitution.Sub.(*schemapb.Substitution_DataSourceProperty); isDataSourceProp {
		return fromSubstitutionDataSourcePropertyPB(dataSourceProp.DataSourceProperty)
	}

	if resourceProp, isResourceProp := substitution.Sub.(*schemapb.Substitution_ResourceProperty); isResourceProp {
		return fromSubstitutionResourcePropertyPB(resourceProp.ResourceProperty)
	}

	if child, isChild := substitution.Sub.(*schemapb.Substitution_Child); isChild {
		return fromSubstitutionChildPB(child.Child)
	}

	if strVal, isStr := substitution.Sub.(*schemapb.Substitution_StringValue); isStr {
		return &substitutions.Substitution{
			StringValue: &strVal.StringValue,
		}, nil
	}

	if intVal, isInt := substitution.Sub.(*schemapb.Substitution_IntValue); isInt {
		return &substitutions.Substitution{
			IntValue: &intVal.IntValue,
		}, nil
	}

	if floatVal, isFloat := substitution.Sub.(*schemapb.Substitution_FloatValue); isFloat {
		return &substitutions.Substitution{
			FloatValue: &floatVal.FloatValue,
		}, nil
	}

	if boolVal, isBool := substitution.Sub.(*schemapb.Substitution_BoolValue); isBool {
		return &substitutions.Substitution{
			BoolValue: &boolVal.BoolValue,
		}, nil
	}

	return nil, errMissingSubstitutionValue()
}

func fromSubstitutionVariablePB(
	substitutionVariablePB *schemapb.SubstitutionVariable,
) (*substitutions.Substitution, error) {
	return &substitutions.Substitution{
		Variable: &substitutions.SubstitutionVariable{
			VariableName: substitutionVariablePB.VariableName,
		},
	}, nil
}

func fromSubstitutionResourcePropertyPB(
	substitutionResourcePropertyPB *schemapb.SubstitutionResourceProperty,
) (*substitutions.Substitution, error) {
	path, err := fromSubstitutionPathItemsPB(substitutionResourcePropertyPB.Path)
	if err != nil {
		return nil, err
	}

	return &substitutions.Substitution{
		ResourceProperty: &substitutions.SubstitutionResourceProperty{
			ResourceName: substitutionResourcePropertyPB.ResourceName,
			Path:         path,
		},
	}, nil
}

func fromSubstitutionDataSourcePropertyPB(
	substitutionDataSourcePropertyPB *schemapb.SubstitutionDataSourceProperty,
) (*substitutions.Substitution, error) {
	return &substitutions.Substitution{
		DataSourceProperty: &substitutions.SubstitutionDataSourceProperty{
			DataSourceName:    substitutionDataSourcePropertyPB.DataSourceName,
			FieldName:         substitutionDataSourcePropertyPB.FieldName,
			PrimitiveArrIndex: substitutionDataSourcePropertyPB.PrimitiveArrIndex,
		},
	}, nil
}

func fromSubstitutionChildPB(
	substitutionChildPB *schemapb.SubstitutionChild,
) (*substitutions.Substitution, error) {
	path, err := fromSubstitutionPathItemsPB(substitutionChildPB.Path)
	if err != nil {
		return nil, err
	}

	return &substitutions.Substitution{
		Child: &substitutions.SubstitutionChild{
			ChildName: substitutionChildPB.ChildName,
			Path:      path,
		},
	}, nil
}

func fromSubstitutionPathItemsPB(
	pathItemsPB []*schemapb.SubstitutionPathItem,
) ([]*substitutions.SubstitutionPathItem, error) {
	var pathItems = make([]*substitutions.SubstitutionPathItem, len(pathItemsPB))
	for i, pathItemPB := range pathItemsPB {
		pathItem, err := fromSubstitutionPathItemPB(pathItemPB)
		if err != nil {
			return nil, err
		}

		pathItems[i] = pathItem
	}
	return pathItems, nil
}

func fromSubstitutionPathItemPB(
	pathItemPB *schemapb.SubstitutionPathItem,
) (*substitutions.SubstitutionPathItem, error) {
	if fieldNameVal, isFieldName := pathItemPB.Item.(*schemapb.SubstitutionPathItem_FieldName); isFieldName {
		return &substitutions.SubstitutionPathItem{
			FieldName: fieldNameVal.FieldName,
		}, nil
	}

	if indexVal, isIndex := pathItemPB.Item.(*schemapb.SubstitutionPathItem_PrimitiveArrIndex); isIndex {
		return &substitutions.SubstitutionPathItem{
			PrimitiveArrIndex: &indexVal.PrimitiveArrIndex,
		}, nil
	}

	return nil, errMissingSubstitutionPathItemValue()
}

func fromSubstitutionFunctionPB(
	substitutionFunctionPB *schemapb.SubstitutionFunction,
) (*substitutions.Substitution, error) {
	arguments, err := fromSubstitutionsPB(substitutionFunctionPB.Arguments)
	if err != nil {
		return nil, err
	}

	return &substitutions.Substitution{
		Function: &substitutions.SubstitutionFunction{
			FunctionName: substitutions.SubstitutionFunctionName(
				substitutionFunctionPB.FunctionName,
			),
			Arguments: arguments,
		},
	}, nil
}

func fromScalarValuePB(scalarValue *schemapb.ScalarValue, optional bool) (*core.ScalarValue, error) {
	if optional && scalarValue == nil {
		return nil, nil
	}

	if stringVal, isString := scalarValue.Value.(*schemapb.ScalarValue_StringValue); isString {
		return &core.ScalarValue{
			StringValue: &stringVal.StringValue,
		}, nil
	}

	if intWrapper, isInt := scalarValue.Value.(*schemapb.ScalarValue_IntValue); isInt {
		intVal := int(intWrapper.IntValue)
		return &core.ScalarValue{
			IntValue: &intVal,
		}, nil
	}

	if floatVal, isFloat := scalarValue.Value.(*schemapb.ScalarValue_FloatValue); isFloat {
		return &core.ScalarValue{
			FloatValue: &floatVal.FloatValue,
		}, nil
	}

	if boolVal, isBool := scalarValue.Value.(*schemapb.ScalarValue_BoolValue); isBool {
		return &core.ScalarValue{
			BoolValue: &boolVal.BoolValue,
		}, nil
	}

	return nil, errMissingScalarValue()
}

func fromScalarValuesPB(scalarValuesPB []*schemapb.ScalarValue) ([]*core.ScalarValue, error) {
	var scalarValues = make([]*core.ScalarValue, len(scalarValuesPB))
	for i, v := range scalarValuesPB {
		scalarValue, err := fromScalarValuePB(v, false)
		if err != nil {
			return nil, err
		}

		scalarValues[i] = scalarValue
	}
	return scalarValues, nil
}
