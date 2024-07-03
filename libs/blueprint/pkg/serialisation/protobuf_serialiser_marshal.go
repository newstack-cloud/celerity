package serialisation

import (
	"github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schemapb"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/substitutions"
	"google.golang.org/protobuf/proto"
)

func (s *ProtobufExpandedBlueprintSerialiser) Marshal(blueprint *schema.Blueprint) ([]byte, error) {
	schemaPB, err := toSchemaPB(blueprint)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(schemaPB)
}

func toSchemaPB(blueprint *schema.Blueprint) (*schemapb.Blueprint, error) {
	variables, err := toVariablesPB(blueprint.Variables)
	if err != nil {
		return nil, err
	}

	includes, err := toIncludesPB(blueprint.Include)
	if err != nil {
		return nil, err
	}

	resources, err := toResourcesPB(blueprint.Resources)
	if err != nil {
		return nil, err
	}

	dataSources, err := toDataSourcesPB(blueprint.DataSources)
	if err != nil {
		return nil, err
	}

	exports, err := toExportsPB(blueprint.Exports)
	if err != nil {
		return nil, err
	}

	metadata, err := toMappingNodePB(blueprint.Metadata, true)
	if err != nil {
		return nil, err
	}

	transform := []string{}
	if blueprint.Transform != nil {
		transform = blueprint.Transform.Values
	}

	return &schemapb.Blueprint{
		Version:     blueprint.Version,
		Transform:   transform,
		Variables:   variables,
		Include:     includes,
		Resources:   resources,
		DataSources: dataSources,
		Exports:     exports,
		Metadata:    metadata,
	}, nil
}

func toVariablesPB(variables *schema.VariableMap) (map[string]*schemapb.Variable, error) {
	if variables == nil {
		return nil, nil
	}

	var variablesPB = make(map[string]*schemapb.Variable)
	for k, v := range variables.Values {
		defaultValue, err := toScalarValuePB(v.Default, true)
		if err != nil {
			return nil, err
		}

		allowedValues, err := toScalarValuesPB(v.AllowedValues)
		if err != nil {
			return nil, err
		}

		variablesPB[k] = &schemapb.Variable{
			Type:          string(v.Type),
			Description:   &v.Description,
			Secret:        v.Secret,
			Default:       defaultValue,
			AllowedValues: allowedValues,
		}
	}
	return variablesPB, nil
}

func toIncludesPB(includes map[string]*schema.Include) (map[string]*schemapb.Include, error) {
	var includesPB = make(map[string]*schemapb.Include)
	for k, v := range includes {
		path, err := toStringOrSubstitutionsPB(v.Path, false)
		if err != nil {
			return nil, err
		}

		variablesPB, err := toMappingNodePB(v.Variables, false)
		if err != nil {
			return nil, err
		}

		metadataPB, err := toMappingNodePB(v.Metadata, false)
		if err != nil {
			return nil, err
		}

		descriptionPB, err := toStringOrSubstitutionsPB(v.Description, true)
		if err != nil {
			return nil, err
		}

		includesPB[k] = &schemapb.Include{
			Path:        path,
			Variables:   variablesPB,
			Metadata:    metadataPB,
			Description: descriptionPB,
		}
	}

	return includesPB, nil
}

func toResourcesPB(resources map[string]*schema.Resource) (map[string]*schemapb.Resource, error) {
	resourcesPB := make(map[string]*schemapb.Resource)
	for k, v := range resources {
		resourcePB, err := toResourcePB(v)
		if err != nil {
			return nil, err
		}

		resourcesPB[k] = resourcePB
	}
	return resourcesPB, nil
}

func toDataSourcesPB(dataSources map[string]*schema.DataSource) (map[string]*schemapb.DataSource, error) {
	dataSourcesPB := make(map[string]*schemapb.DataSource)
	for k, v := range dataSources {
		dataSourcePB, err := toDataSourcePB(v)
		if err != nil {
			return nil, err
		}

		dataSourcesPB[k] = dataSourcePB
	}
	return dataSourcesPB, nil
}

func toDataSourcePB(dataSource *schema.DataSource) (*schemapb.DataSource, error) {
	descriptionPB, err := toStringOrSubstitutionsPB(dataSource.Description, true)
	if err != nil {
		return nil, err
	}

	metadataPB, err := toDataSourceMetadataPB(dataSource.DataSourceMetadata)
	if err != nil {
		return nil, err
	}

	filterPB, err := toDataSourceFilterPB(dataSource.Filter)
	if err != nil {
		return nil, err
	}

	exportsPB, err := toDataSourceFieldExports(dataSource.Exports)
	if err != nil {
		return nil, err
	}

	return &schemapb.DataSource{
		Type:        string(dataSource.Type),
		Metadata:    metadataPB,
		Filter:      filterPB,
		Exports:     exportsPB,
		Description: descriptionPB,
	}, nil
}

func toDataSourceMetadataPB(metadata *schema.DataSourceMetadata) (*schemapb.DataSourceMetadata, error) {
	displayNamePB, err := toStringOrSubstitutionsPB(metadata.DisplayName, true)
	if err != nil {
		return nil, err
	}

	annotationsPB, err := toAnnotationsPB(metadata.Annotations)
	if err != nil {
		return nil, err
	}

	customPB, err := toMappingNodePB(metadata.Custom, true)
	if err != nil {
		return nil, err
	}

	return &schemapb.DataSourceMetadata{
		DisplayName: displayNamePB,
		Annotations: annotationsPB,
		Custom:      customPB,
	}, nil
}

func toDataSourceFilterPB(filter *schema.DataSourceFilter) (*schemapb.DataSourceFilter, error) {
	searchPB, err := toDataSourceFilterSearchPB(filter.Search)
	if err != nil {
		return nil, err
	}

	return &schemapb.DataSourceFilter{
		Field:    filter.Field,
		Operator: string(filter.Operator.Value),
		Search:   searchPB,
	}, nil
}

func toDataSourceFilterSearchPB(
	search *schema.DataSourceFilterSearch,
) (*schemapb.DataSourceFilterSearch, error) {
	valuesPB := make([]*schemapb.StringOrSubstitutions, len(search.Values))
	for i, v := range search.Values {
		valuePB, err := toStringOrSubstitutionsPB(v, false)
		if err != nil {
			return nil, err
		}

		valuesPB[i] = valuePB
	}

	return &schemapb.DataSourceFilterSearch{
		Values: valuesPB,
	}, nil
}

func toDataSourceFieldExports(
	exports map[string]*schema.DataSourceFieldExport,
) (map[string]*schemapb.DataSourceFieldExport, error) {
	exportsPB := make(map[string]*schemapb.DataSourceFieldExport)
	for k, v := range exports {
		exportPB, err := toDataSourceFieldExportPB(v)
		if err != nil {
			return nil, err
		}

		exportsPB[k] = exportPB
	}

	return exportsPB, nil
}

func toDataSourceFieldExportPB(
	export *schema.DataSourceFieldExport,
) (*schemapb.DataSourceFieldExport, error) {
	descriptionPB, err := toStringOrSubstitutionsPB(export.Description, true)
	if err != nil {
		return nil, err
	}

	return &schemapb.DataSourceFieldExport{
		Type:        string(export.Type.Value),
		AliasFor:    export.AliasFor,
		Description: descriptionPB,
	}, nil
}

func toResourcePB(resource *schema.Resource) (*schemapb.Resource, error) {
	descriptionPB, err := toStringOrSubstitutionsPB(resource.Description, true)
	if err != nil {
		return nil, err
	}

	resourceMetadataPB, err := toResourceMetadataPB(resource.Metadata)
	if err != nil {
		return nil, err
	}

	specPB, err := toMappingNodePB(resource.Spec, false)
	if err != nil {
		return nil, err
	}

	return &schemapb.Resource{
		Type:         string(resource.Type),
		Description:  descriptionPB,
		Metadata:     resourceMetadataPB,
		LinkSelector: toLinkSelectorPB(resource.LinkSelector),
		Spec:         specPB,
	}, nil
}

func toExportsPB(exports map[string]*schema.Export) (map[string]*schemapb.Export, error) {
	exportsPB := make(map[string]*schemapb.Export)
	for k, v := range exports {
		exportPB, err := toExportPB(v)
		if err != nil {
			return nil, err
		}

		exportsPB[k] = exportPB
	}

	return exportsPB, nil
}

func toExportPB(export *schema.Export) (*schemapb.Export, error) {
	descriptionPB, err := toStringOrSubstitutionsPB(export.Description, true)
	if err != nil {
		return nil, err
	}

	return &schemapb.Export{
		Type:        string(export.Type),
		Field:       export.Field,
		Description: descriptionPB,
	}, nil
}

func toLinkSelectorPB(linkSelector *schema.LinkSelector) *schemapb.LinkSelector {
	if linkSelector == nil {
		return nil
	}

	return &schemapb.LinkSelector{
		ByLabel: linkSelector.ByLabel,
	}
}

func toResourceMetadataPB(metadata *schema.Metadata) (*schemapb.ResourceMetadata, error) {
	if metadata == nil {
		return nil, nil
	}

	displayNamePB, err := toStringOrSubstitutionsPB(metadata.DisplayName, true)
	if err != nil {
		return nil, err
	}

	annotationsPB, err := toAnnotationsPB(metadata.Annotations)
	if err != nil {
		return nil, err
	}

	customPB, err := toMappingNodePB(metadata.Custom, true)
	if err != nil {
		return nil, err
	}

	return &schemapb.ResourceMetadata{
		DisplayName: displayNamePB,
		Annotations: annotationsPB,
		Labels:      metadata.Labels,
		Custom:      customPB,
	}, nil
}

func toAnnotationsPB(
	annotations map[string]*substitutions.StringOrSubstitutions,
) (map[string]*schemapb.StringOrSubstitutions, error) {
	annotationsPB := make(map[string]*schemapb.StringOrSubstitutions)
	for k, v := range annotations {
		stringOrSubsPB, err := toStringOrSubstitutionsPB(v, false)
		if err != nil {
			return nil, err
		}

		annotationsPB[k] = stringOrSubsPB
	}

	return annotationsPB, nil
}

func toMappingNodePB(mappingNode *core.MappingNode, optional bool) (*schemapb.MappingNode, error) {
	if optional && mappingNode == nil {
		return nil, nil
	}

	if !optional && mappingNode == nil {
		return nil, errMappingNodeIsNil()
	}

	if mappingNode.Literal != nil {
		scalarPB, err := toScalarValuePB(mappingNode.Literal, false)
		if err != nil {
			return nil, err
		}

		return &schemapb.MappingNode{
			Literal: scalarPB,
		}, nil
	}

	if mappingNode.Fields != nil {
		return toMappingNodeFieldsPB(mappingNode.Fields)
	}

	if mappingNode.Items != nil {
		return toMappingNodeItemsPB(mappingNode.Items)
	}

	if mappingNode.StringWithSubstitutions != nil {
		stringOrSubsPB, err := toStringOrSubstitutionsPB(mappingNode.StringWithSubstitutions, false)
		if err != nil {
			return nil, err
		}

		return &schemapb.MappingNode{
			StringWithSubstitutions: stringOrSubsPB,
		}, nil
	}

	return nil, errMissingMappingNodeValue()
}

func toMappingNodeFieldsPB(fields map[string]*core.MappingNode) (*schemapb.MappingNode, error) {
	fieldsPB := make(map[string]*schemapb.MappingNode)
	for k, v := range fields {
		mappingNodePB, err := toMappingNodePB(v, true)
		if err != nil {
			return nil, err
		}

		fieldsPB[k] = mappingNodePB
	}

	return &schemapb.MappingNode{
		Fields: fieldsPB,
	}, nil
}

func toMappingNodeItemsPB(items []*core.MappingNode) (*schemapb.MappingNode, error) {
	itemsPB := make([]*schemapb.MappingNode, len(items))
	for i, item := range items {
		mappingNodePB, err := toMappingNodePB(item, true)
		if err != nil {
			return nil, err
		}

		itemsPB[i] = mappingNodePB
	}

	return &schemapb.MappingNode{
		Items: itemsPB,
	}, nil
}

func toStringOrSubstitutionsPB(
	stringOrSubstitutions *substitutions.StringOrSubstitutions,
	optional bool,
) (*schemapb.StringOrSubstitutions, error) {
	if optional && stringOrSubstitutions == nil {
		return nil, nil
	}

	if !optional && stringOrSubstitutions == nil {
		return nil, errStringOrSubstitutionsIsNil()
	}

	stringOrSubsPB := make([]*schemapb.StringOrSubstitution, len(stringOrSubstitutions.Values))
	for i, stringOrSub := range stringOrSubstitutions.Values {
		stringOrSubPB, err := toStringOrSubstitutionPB(stringOrSub)
		if err != nil {
			return nil, err
		}
		stringOrSubsPB[i] = stringOrSubPB
	}
	return &schemapb.StringOrSubstitutions{
		Values: stringOrSubsPB,
	}, nil
}

func toStringOrSubstitutionPB(
	stringOrSubstitution *substitutions.StringOrSubstitution,
) (*schemapb.StringOrSubstitution, error) {
	if stringOrSubstitution.StringValue != nil {
		return &schemapb.StringOrSubstitution{
			Value: &schemapb.StringOrSubstitution_StringValue{
				StringValue: *stringOrSubstitution.StringValue,
			},
		}, nil
	}

	if stringOrSubstitution.SubstitutionValue != nil {
		substitutionPB, err := toSubstitutionPB(stringOrSubstitution.SubstitutionValue)
		if err != nil {
			return nil, err
		}

		return &schemapb.StringOrSubstitution{
			Value: &schemapb.StringOrSubstitution_SubstitutionValue{
				SubstitutionValue: substitutionPB,
			},
		}, nil
	}

	return nil, errMissingStringOrSubstitutionValue()
}

func toSubstitutionsPB(substitutions []*substitutions.Substitution) ([]*schemapb.Substitution, error) {
	var substitutionsPB = make([]*schemapb.Substitution, len(substitutions))
	for i, s := range substitutions {
		substitutionPB, err := toSubstitutionPB(s)
		if err != nil {
			return nil, err
		}

		substitutionsPB[i] = substitutionPB
	}
	return substitutionsPB, nil
}

func toSubstitutionPB(substitution *substitutions.Substitution) (*schemapb.Substitution, error) {
	if substitution.Function != nil {
		return toSubstitutionFunctionPB(substitution.Function)
	}

	if substitution.Variable != nil {
		return toSubstitutionVariablePB(substitution.Variable)
	}

	if substitution.DataSourceProperty != nil {
		return toSubstitutionDataSourcePropertyPB(substitution.DataSourceProperty)
	}

	if substitution.ResourceProperty != nil {
		return toSubstitutionResourcePropertyPB(substitution.ResourceProperty)
	}

	if substitution.Child != nil {
		return toSubstitutionChildPB(substitution.Child)
	}

	if substitution.StringValue != nil {
		return &schemapb.Substitution{
			Sub: &schemapb.Substitution_StringValue{
				StringValue: *substitution.StringValue,
			},
		}, nil
	}

	if substitution.IntValue != nil {
		return &schemapb.Substitution{
			Sub: &schemapb.Substitution_IntValue{
				IntValue: *substitution.IntValue,
			},
		}, nil
	}

	if substitution.FloatValue != nil {
		return &schemapb.Substitution{
			Sub: &schemapb.Substitution_FloatValue{
				FloatValue: *substitution.FloatValue,
			},
		}, nil
	}

	if substitution.BoolValue != nil {
		return &schemapb.Substitution{
			Sub: &schemapb.Substitution_BoolValue{
				BoolValue: *substitution.BoolValue,
			},
		}, nil
	}

	return nil, errMissingSubstitutionValue()
}

func toSubstitutionVariablePB(
	substitutionVariable *substitutions.SubstitutionVariable,
) (*schemapb.Substitution, error) {
	return &schemapb.Substitution{
		Sub: &schemapb.Substitution_Variable{
			Variable: &schemapb.SubstitutionVariable{
				VariableName: substitutionVariable.VariableName,
			},
		},
	}, nil
}

func toSubstitutionResourcePropertyPB(
	substitutionResourceProperty *substitutions.SubstitutionResourceProperty,
) (*schemapb.Substitution, error) {
	path, err := toSubstitutionPathItems(substitutionResourceProperty.Path)
	if err != nil {
		return nil, err
	}

	return &schemapb.Substitution{
		Sub: &schemapb.Substitution_ResourceProperty{
			ResourceProperty: &schemapb.SubstitutionResourceProperty{
				ResourceName: substitutionResourceProperty.ResourceName,
				Path:         path,
			},
		},
	}, nil
}

func toSubstitutionPathItems(
	pathItems []*substitutions.SubstitutionPathItem,
) ([]*schemapb.SubstitutionPathItem, error) {
	var pathItemsPB = make([]*schemapb.SubstitutionPathItem, len(pathItems))
	for i, pathItem := range pathItems {
		pathItemPB, err := toSubstitutionPathItemPB(pathItem)
		if err != nil {
			return nil, err
		}

		pathItemsPB[i] = pathItemPB
	}
	return pathItemsPB, nil
}

func toSubstitutionPathItemPB(
	pathItem *substitutions.SubstitutionPathItem,
) (*schemapb.SubstitutionPathItem, error) {
	if pathItem.FieldName != "" {
		return &schemapb.SubstitutionPathItem{
			Item: &schemapb.SubstitutionPathItem_FieldName{
				FieldName: pathItem.FieldName,
			},
		}, nil
	}

	if pathItem.PrimitiveArrIndex != nil {
		return &schemapb.SubstitutionPathItem{
			Item: &schemapb.SubstitutionPathItem_PrimitiveArrIndex{
				PrimitiveArrIndex: *pathItem.PrimitiveArrIndex,
			},
		}, nil
	}

	return nil, errMissingSubstitutionPathItemValue()
}

func toSubstitutionDataSourcePropertyPB(
	substitutionDataSourceProperty *substitutions.SubstitutionDataSourceProperty,
) (*schemapb.Substitution, error) {
	return &schemapb.Substitution{
		Sub: &schemapb.Substitution_DataSourceProperty{
			DataSourceProperty: &schemapb.SubstitutionDataSourceProperty{
				DataSourceName:    substitutionDataSourceProperty.DataSourceName,
				FieldName:         substitutionDataSourceProperty.FieldName,
				PrimitiveArrIndex: substitutionDataSourceProperty.PrimitiveArrIndex,
			},
		},
	}, nil
}

func toSubstitutionChildPB(
	substitutionChild *substitutions.SubstitutionChild,
) (*schemapb.Substitution, error) {
	path, err := toSubstitutionPathItems(substitutionChild.Path)
	if err != nil {
		return nil, err
	}

	return &schemapb.Substitution{
		Sub: &schemapb.Substitution_Child{
			Child: &schemapb.SubstitutionChild{
				ChildName: substitutionChild.ChildName,
				Path:      path,
			},
		},
	}, nil
}

func toSubstitutionFunctionPB(
	substitutionFunction *substitutions.SubstitutionFunction,
) (*schemapb.Substitution, error) {
	arguments, err := toSubstitutionsPB(substitutionFunction.Arguments)
	if err != nil {
		return nil, err
	}

	return &schemapb.Substitution{
		Sub: &schemapb.Substitution_Function{
			Function: &schemapb.SubstitutionFunction{
				FunctionName: string(substitutionFunction.FunctionName),
				Arguments:    arguments,
			},
		},
	}, nil
}

func toScalarValuesPB(scalarValues []*core.ScalarValue) ([]*schemapb.ScalarValue, error) {
	var scalarValuesPB = make([]*schemapb.ScalarValue, len(scalarValues))
	for i, v := range scalarValues {
		scalarValuePB, err := toScalarValuePB(v, false)
		if err != nil {
			return nil, err
		}

		scalarValuesPB[i] = scalarValuePB
	}
	return scalarValuesPB, nil
}

func toScalarValuePB(scalarValue *core.ScalarValue, optional bool) (*schemapb.ScalarValue, error) {
	if optional && scalarValue == nil {
		return nil, nil
	}

	if scalarValue.StringValue != nil {
		return &schemapb.ScalarValue{
			Value: &schemapb.ScalarValue_StringValue{StringValue: *scalarValue.StringValue},
		}, nil
	}

	if scalarValue.IntValue != nil {
		return &schemapb.ScalarValue{
			Value: &schemapb.ScalarValue_IntValue{IntValue: int64(*scalarValue.IntValue)},
		}, nil
	}

	if scalarValue.FloatValue != nil {
		return &schemapb.ScalarValue{
			Value: &schemapb.ScalarValue_FloatValue{FloatValue: *scalarValue.FloatValue},
		}, nil
	}

	if scalarValue.BoolValue != nil {
		return &schemapb.ScalarValue{
			Value: &schemapb.ScalarValue_BoolValue{BoolValue: *scalarValue.BoolValue},
		}, nil
	}

	return nil, errMissingScalarValue()
}
