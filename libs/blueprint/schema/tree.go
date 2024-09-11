package schema

import (
	"fmt"
	"slices"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	"github.com/two-hundred/celerity/libs/common/core"
)

// TreeNode is a tree representation of a blueprint
// schema that is in sequential source order, from left to right.
// As a blueprint is made up of an augmentation of a host document
// language (e.g. YAML) and the embedded substitution language, there
// is no natural AST or AST-like representation of a blueprint.
//
// TreeNode is derived from a parsed schema to provide a sequential representation
// of elements in a blueprint that makes it easier to determine accurate ranges
// for diagnostics, completions and other language server features.
//
// The unordered maps used in the schema structs are not suitable for
// efficiently determining where elements start and end which is essential for tools
// such as language servers.
//
// A tree should only be used when source location information is made available
// by the host document language parser. (e.g. YAML)
type TreeNode struct {
	// Label is the label of the tree node.
	// For a node that represents a named field in a schema element,
	// the label is the name of the field. (e.g. "variables" or "resources")
	// For a node that represents an element in a list, the label is the index
	// of the element in the list.
	// For a node that represents an element in a mapping, the label is the key
	// of the element in the mapping.
	Label string
	// Path contains the path to the tree node relative to the root.
	// For example, the path to a variable type node could be
	// "/variables/myVar/type".
	// "/" is used as the separator as "." can be in names of elements.
	// The path is made up of the labels of the nodes in the tree.
	Path string
	// Type is the type of the tree node.
	Type TreeNodeType
	// Children is a list of child nodes in the tree.
	Children []*TreeNode
	// SchemaElement is the schema element that this tree node represents.
	SchemaElement interface{}
	// Range is the source range of the tree node in the blueprint source document.
	Range *source.Range
}

// SetRangeEnd sets the range end of a tree node and the last child.
// This is applied recursively to the last child of the last child and so on.
func (n *TreeNode) SetRangeEnd(end *source.Meta) {
	n.Range = &source.Range{
		Start: n.Range.Start,
		End:   end,
	}

	if len(n.Children) > 0 {
		n.Children[len(n.Children)-1].SetRangeEnd(end)
	}
}

// TreeNodeType is the type of a tree node.
type TreeNodeType int

const (
	// TreeNodeTypeNonTerminal is a non-terminal node of the tree
	// of a blueprint.
	TreeNodeTypeNonTerminal TreeNodeType = iota
	// TreeNodeTypeLeaf is a leaf node of the tree
	// of a blueprint.
	TreeNodeTypeLeaf
)

// SchemaToTree converts a blueprint schema to a tree representation.
func SchemaToTree(blueprint *Blueprint) *TreeNode {
	if blueprint == nil {
		return nil
	}

	root := &TreeNode{
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: blueprint,
	}

	children := []*TreeNode{}

	versionNode := versionToTreeNode(blueprint.Version, root.Path)
	if versionNode != nil {
		children = append(children, versionNode)
	}

	transformNode := transformToTreeNode(blueprint.Transform, root.Path)
	if transformNode != nil {
		children = append(children, transformNode)
	}

	variablesNode := transformToVariablesNode(blueprint.Variables, root.Path)
	if variablesNode != nil {
		children = append(children, variablesNode)
	}

	valuesNode := transformToValuesNode(blueprint.Values, root.Path)
	if valuesNode != nil {
		children = append(children, valuesNode)
	}

	includesNode := transformToIncludesNode(blueprint.Include, root.Path)
	if includesNode != nil {
		children = append(children, includesNode)
	}

	resourcesNode := transformToResourcesNode(blueprint.Resources, root.Path)
	if resourcesNode != nil {
		children = append(children, resourcesNode)
	}

	dataSourcesNode := transformToDataSourcesNode(blueprint.DataSources, root.Path)
	if dataSourcesNode != nil {
		children = append(children, dataSourcesNode)
	}

	exportsNode := transformToExportsNode(blueprint.Exports, root.Path)
	if exportsNode != nil {
		children = append(children, exportsNode)
	}

	metadataNode := transformToMappingNode("metadata", blueprint.Metadata, root.Path, nil)
	if metadataNode != nil {
		children = append(children, metadataNode)
	}

	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	root.Children = children

	root.Range = &source.Range{
		Start: &source.Meta{
			Line:   1,
			Column: 1,
		},
		// Root node doesn't have an end location, the range for the very last leaf node
		// will extend to the end of the document.
	}

	return root
}

func versionToTreeNode(version *bpcore.ScalarValue, parentPath string) *TreeNode {
	if version == nil {
		return nil
	}

	versionNode := &TreeNode{
		Label:         "version",
		Path:          fmt.Sprintf("%s/version", parentPath),
		Type:          TreeNodeTypeLeaf,
		SchemaElement: version,
		Range: &source.Range{
			Start: version.SourceMeta,
		},
	}

	return versionNode
}

func transformToTreeNode(transform *TransformValueWrapper, parentPath string) *TreeNode {
	if transform == nil || len(transform.Values) == 0 {
		return nil
	}

	transformNode := &TreeNode{
		Label:         "transform",
		Path:          fmt.Sprintf("%s/transform", parentPath),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: transform,
		Range: &source.Range{
			Start: transform.SourceMeta[0],
		},
	}

	children := make([]*TreeNode, len(transform.Values))
	for i, value := range transform.Values {
		child := &TreeNode{
			Label:         fmt.Sprintf("%d", i),
			Path:          fmt.Sprintf("%s/transform/%d", parentPath, i),
			Type:          TreeNodeTypeLeaf,
			SchemaElement: value,
			Range: &source.Range{
				Start: transform.SourceMeta[i],
			},
		}
		children[i] = child
	}
	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	transformNode.Children = children

	return transformNode
}

func transformToVariablesNode(variables *VariableMap, parentPath string) *TreeNode {
	if variables == nil || len(variables.Values) == 0 {
		return nil
	}

	variablesNode := &TreeNode{
		Label:         "variables",
		Path:          fmt.Sprintf("%s/variables", parentPath),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: variables,
		Range: &source.Range{
			Start: minPosition(core.MapToSlice(variables.SourceMeta)),
		},
	}

	children := []*TreeNode{}
	for varName, variable := range variables.Values {
		variableNode := transformToVariableNode(
			varName,
			variable,
			parentPath,
			variables.SourceMeta[varName],
		)
		if variableNode != nil {
			children = append(children, variableNode)
		}
	}
	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	variablesNode.Children = children

	return variablesNode
}

func transformToVariableNode(varName string, variable *Variable, parentPath string, location *source.Meta) *TreeNode {
	variableNode := &TreeNode{
		Label:         varName,
		Path:          fmt.Sprintf("%s/%s", parentPath, varName),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: variable,
		Range: &source.Range{
			Start: location,
		},
	}

	children := []*TreeNode{}

	varTypeNode := transformToVariableTypeNode(variable.Type, variableNode.Path)
	if varTypeNode != nil {
		children = append(children, varTypeNode)
	}

	descriptionNode := transformToScalarNode(
		"description",
		variable.Description,
		variableNode.Path,
	)
	if descriptionNode != nil {
		children = append(children, descriptionNode)
	}

	secretNode := transformToScalarNode(
		"secret",
		variable.Secret,
		variableNode.Path,
	)
	if secretNode != nil {
		children = append(children, secretNode)
	}

	defaultNode := transformToScalarNode(
		"default",
		variable.Default,
		variableNode.Path,
	)
	if defaultNode != nil {
		children = append(children, defaultNode)
	}

	allowedValuesNode := transformToScalarsNode(
		"allowedValues",
		variable.AllowedValues,
		variableNode.Path,
	)
	if allowedValuesNode != nil {
		children = append(children, allowedValuesNode)
	}

	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	variableNode.Children = children

	return variableNode
}

func transformToVariableTypeNode(varType *VariableTypeWrapper, parentPath string) *TreeNode {
	if varType == nil {
		return nil
	}

	return &TreeNode{
		Label:         "type",
		Path:          fmt.Sprintf("%s/type", parentPath),
		Type:          TreeNodeTypeLeaf,
		SchemaElement: varType,
		Range: &source.Range{
			Start: varType.SourceMeta,
		},
	}
}

func transformToValuesNode(values *ValueMap, parentPath string) *TreeNode {
	if values == nil || len(values.Values) == 0 {
		return nil
	}

	valuesNode := &TreeNode{
		Label:         "values",
		Path:          fmt.Sprintf("%s/values", parentPath),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: values,
		Range: &source.Range{
			Start: minPosition(core.MapToSlice(values.SourceMeta)),
		},
	}

	children := []*TreeNode{}
	for valName, val := range values.Values {
		valNode := transformToValueNode(
			valName,
			val,
			parentPath,
			values.SourceMeta[valName],
		)
		if valNode != nil {
			children = append(children, valNode)
		}
	}
	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	valuesNode.Children = children

	return valuesNode
}

func transformToValueNode(valName string, value *Value, parentPath string, location *source.Meta) *TreeNode {
	valueNode := &TreeNode{
		Label:         valName,
		Path:          fmt.Sprintf("%s/%s", parentPath, valName),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: value,
		Range: &source.Range{
			Start: location,
		},
	}

	children := []*TreeNode{}

	valTypeNode := transformToValueTypeNode(value.Type, valueNode.Path)
	if valTypeNode != nil {
		children = append(children, valTypeNode)
	}

	contentNode := transformToStringSubsNode(
		"value",
		value.Value,
		valueNode.Path,
	)
	if contentNode != nil {
		children = append(children, contentNode)
	}

	descriptionNode := transformToStringSubsNode(
		"description",
		value.Description,
		valueNode.Path,
	)
	if descriptionNode != nil {
		children = append(children, descriptionNode)
	}

	secretNode := transformToScalarNode(
		"secret",
		value.Secret,
		valueNode.Path,
	)
	if secretNode != nil {
		children = append(children, secretNode)
	}

	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	valueNode.Children = children

	return valueNode
}

func transformToValueTypeNode(valType *ValueTypeWrapper, parentPath string) *TreeNode {
	if valType == nil {
		return nil
	}

	return &TreeNode{
		Label:         "type",
		Path:          fmt.Sprintf("%s/type", parentPath),
		Type:          TreeNodeTypeLeaf,
		SchemaElement: valType,
		Range: &source.Range{
			Start: valType.SourceMeta,
		},
	}
}

func transformToIncludesNode(includes *IncludeMap, parentPath string) *TreeNode {
	if includes == nil || len(includes.Values) == 0 {
		return nil
	}

	includesNode := &TreeNode{
		Label:         "includes",
		Path:          fmt.Sprintf("%s/includes", parentPath),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: includes,
		Range: &source.Range{
			Start: minPosition(core.MapToSlice(includes.SourceMeta)),
		},
	}

	children := []*TreeNode{}
	for includeName, val := range includes.Values {
		includeNode := transformToIncludeNode(
			includeName,
			val,
			parentPath,
			includes.SourceMeta[includeName],
		)
		if includeNode != nil {
			children = append(children, includeNode)
		}
	}
	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	includesNode.Children = children

	return includesNode
}

func transformToIncludeNode(includeName string, include *Include, parentPath string, location *source.Meta) *TreeNode {
	includeNode := &TreeNode{
		Label:         includeName,
		Path:          fmt.Sprintf("%s/%s", parentPath, includeName),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: include,
		Range: &source.Range{
			Start: location,
		},
	}

	children := []*TreeNode{}

	pathNode := transformToStringSubsNode("path", include.Path, includeNode.Path)
	if pathNode != nil {
		children = append(children, pathNode)
	}

	variablesNode := transformToMappingNode("variables", include.Variables, includeNode.Path, nil)
	if variablesNode != nil {
		children = append(children, variablesNode)
	}

	metadataNode := transformToMappingNode("metadata", include.Metadata, includeNode.Path, nil)
	if metadataNode != nil {
		children = append(children, metadataNode)
	}

	descriptionNode := transformToStringSubsNode("description", include.Description, includeNode.Path)
	if descriptionNode != nil {
		children = append(children, descriptionNode)
	}

	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	includeNode.Children = children

	return includeNode
}

func transformToResourcesNode(resources *ResourceMap, parentPath string) *TreeNode {
	if resources == nil || len(resources.Values) == 0 {
		return nil
	}

	resourcesNode := &TreeNode{
		Label:         "resources",
		Path:          fmt.Sprintf("%s/resources", parentPath),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: resources,
		Range: &source.Range{
			Start: minPosition(core.MapToSlice(resources.SourceMeta)),
		},
	}

	children := []*TreeNode{}
	for resourceName, val := range resources.Values {
		resourceNode := transformToResourceNode(
			resourceName,
			val,
			parentPath,
			resources.SourceMeta[resourceName],
		)
		if resourceNode != nil {
			children = append(children, resourceNode)
		}
	}
	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	resourcesNode.Children = children

	return resourcesNode
}

func transformToResourceNode(resourceName string, resource *Resource, parentPath string, location *source.Meta) *TreeNode {
	resourceNode := &TreeNode{
		Label:         resourceName,
		Path:          fmt.Sprintf("%s/%s", parentPath, resourceName),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: resource,
		Range: &source.Range{
			Start: location,
		},
	}

	children := []*TreeNode{}

	resourceTypeNode := transformToResourceTypeNode(resource.Type, resourceNode.Path)
	if resourceTypeNode != nil {
		children = append(children, resourceTypeNode)
	}

	descriptionNode := transformToStringSubsNode("description", resource.Description, resourceNode.Path)
	if descriptionNode != nil {
		children = append(children, descriptionNode)
	}

	metadataNode := transformToResourceMetadataNode(resource.Metadata, resourceNode.Path)
	if metadataNode != nil {
		children = append(children, metadataNode)
	}

	conditionNode := transformToResourceConditionNode("condition", resource.Condition, resourceNode.Path)
	if conditionNode != nil {
		children = append(children, conditionNode)
	}

	eachNode := transformToStringSubsNode("each", resource.Each, resourceNode.Path)
	if eachNode != nil {
		children = append(children, eachNode)
	}

	linkSelectorNode := transformToResourceLinkSelectorNode(resource.LinkSelector, resourceNode.Path)
	if linkSelectorNode != nil {
		children = append(children, linkSelectorNode)
	}

	specNode := transformToMappingNode("spec", resource.Spec, resourceNode.Path, nil)
	if specNode != nil {
		children = append(children, specNode)
	}

	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	resourceNode.Children = children

	return resourceNode
}

func transformToResourceLinkSelectorNode(linkSelector *LinkSelector, parentPath string) *TreeNode {
	if linkSelector == nil {
		return nil
	}

	linkSelectorNode := &TreeNode{
		Label:         "linkSelector",
		Path:          fmt.Sprintf("%s/linkSelector", parentPath),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: linkSelector,
		Range: &source.Range{
			Start: linkSelector.SourceMeta,
		},
	}

	if linkSelector.ByLabel == nil {
		linkSelectorNode.Type = TreeNodeTypeLeaf
		return linkSelectorNode
	}

	byLabelNode := transformToStringMapNode("byLabel", linkSelector.ByLabel, linkSelectorNode.Path)
	if byLabelNode != nil {
		linkSelectorNode.Children = []*TreeNode{byLabelNode}
	} else {
		linkSelectorNode.Type = TreeNodeTypeLeaf
	}

	return linkSelectorNode
}

func transformToResourceConditionNode(label string, condition *Condition, parentPath string) *TreeNode {
	if condition == nil {
		return nil
	}

	conditionNode := &TreeNode{
		Label:         label,
		Path:          fmt.Sprintf("%s/%s", parentPath, label),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: condition,
		Range: &source.Range{
			Start: condition.SourceMeta,
		},
	}

	children := []*TreeNode{}

	if condition.StringValue != nil {
		exprNode := transformToStringSubsNode("expr", condition.StringValue, conditionNode.Path)
		if exprNode != nil {
			children = append(children, exprNode)
		}
	}

	if condition.And != nil {
		andNode := transformToConditionListNode("and", condition.And, conditionNode.Path)
		if andNode != nil {
			children = append(children, andNode)
		}
	}

	if condition.Or != nil {
		orNode := transformToConditionListNode("or", condition.Or, conditionNode.Path)
		if orNode != nil {
			children = append(children, orNode)
		}
	}

	if condition.Not != nil {
		notNode := transformToConditionNotNode(condition.Not, conditionNode.Path)
		if notNode != nil {
			children = append(children, notNode)
		}
	}

	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	conditionNode.Children = children

	return conditionNode
}

func transformToConditionListNode(label string, conditions []*Condition, parentPath string) *TreeNode {
	if len(conditions) == 0 {
		return nil
	}

	condListNode := &TreeNode{
		Label:         label,
		Path:          fmt.Sprintf("%s/%s", parentPath, label),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: conditions,
		Range: &source.Range{
			Start: conditions[0].SourceMeta,
		},
	}

	children := make([]*TreeNode, len(conditions))
	for i, cond := range conditions {
		child := transformToResourceConditionNode(fmt.Sprintf("%d", i), cond, condListNode.Path)
		if child != nil {
			children[i] = child
		}
	}

	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	condListNode.Children = children

	return condListNode
}

func transformToConditionNotNode(toNegate *Condition, parentPath string) *TreeNode {
	if toNegate == nil {
		return nil
	}

	notNode := &TreeNode{
		Label:         "not",
		Path:          fmt.Sprintf("%s/not", parentPath),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: toNegate,
		Range: &source.Range{
			Start: toNegate.SourceMeta,
		},
	}

	child := transformToResourceConditionNode("0", toNegate, notNode.Path)
	if child != nil {
		notNode.Children = []*TreeNode{child}
	}

	return notNode
}

func transformToResourceTypeNode(resType *ResourceTypeWrapper, parentPath string) *TreeNode {
	if resType == nil {
		return nil
	}

	return &TreeNode{
		Label:         "type",
		Path:          fmt.Sprintf("%s/type", parentPath),
		Type:          TreeNodeTypeLeaf,
		SchemaElement: resType,
		Range: &source.Range{
			Start: resType.SourceMeta,
		},
	}
}

func transformToResourceMetadataNode(metadata *Metadata, parentPath string) *TreeNode {
	if metadata == nil {
		return nil
	}

	metadataNode := &TreeNode{
		Label:         "metadata",
		Path:          fmt.Sprintf("%s/metadata", parentPath),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: metadata,
		Range: &source.Range{
			Start: metadata.SourceMeta,
		},
	}

	children := []*TreeNode{}

	displayNameNode := transformToStringSubsNode("displayName", metadata.DisplayName, metadataNode.Path)
	if displayNameNode != nil {
		children = append(children, displayNameNode)
	}

	annotationsNode := trnasformToStringSubsMapNode("annotations", metadata.Annotations, metadataNode.Path)
	if annotationsNode != nil {
		children = append(children, annotationsNode)
	}

	labelsNode := transformToStringMapNode("labels", metadata.Labels, metadataNode.Path)
	if labelsNode != nil {
		children = append(children, labelsNode)
	}

	customNode := transformToMappingNode("custom", metadata.Custom, metadataNode.Path, nil)
	if customNode != nil {
		children = append(children, customNode)
	}

	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	metadataNode.Children = children

	return metadataNode
}

func transformToDataSourcesNode(dataSources *DataSourceMap, parentPath string) *TreeNode {
	if dataSources == nil || len(dataSources.Values) == 0 {
		return nil
	}

	dataSourcesNode := &TreeNode{
		Label:         "datasources",
		Path:          fmt.Sprintf("%s/datasources", parentPath),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: dataSources,
		Range: &source.Range{
			Start: minPosition(core.MapToSlice(dataSources.SourceMeta)),
		},
	}

	children := []*TreeNode{}
	for dataSourceName, val := range dataSources.Values {
		dataSourceNode := transformToDataSourceNode(
			dataSourceName,
			val,
			parentPath,
			dataSources.SourceMeta[dataSourceName],
		)
		if dataSourceNode != nil {
			children = append(children, dataSourceNode)
		}
	}
	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	dataSourcesNode.Children = children

	return dataSourcesNode
}

func transformToDataSourceNode(dataSourceName string, dataSource *DataSource, parentPath string, location *source.Meta) *TreeNode {
	dataSourceNode := &TreeNode{
		Label:         dataSourceName,
		Path:          fmt.Sprintf("%s/%s", parentPath, dataSourceName),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: dataSource,
		Range: &source.Range{
			Start: location,
		},
	}

	children := []*TreeNode{}

	dataSourceTypeNode := transformToDataSourceTypeNode(dataSource.Type, dataSourceNode.Path)
	if dataSourceTypeNode != nil {
		children = append(children, dataSourceTypeNode)
	}

	dataSourceMetadataNode := transformToDataSourceMetataNode(dataSource.DataSourceMetadata, dataSourceNode.Path)
	if dataSourceMetadataNode != nil {
		children = append(children, dataSourceMetadataNode)
	}

	dataSourceFilterNode := transformToDataSourceFilterNode(dataSource.Filter, dataSourceNode.Path)
	if dataSourceFilterNode != nil {
		children = append(children, dataSourceFilterNode)
	}

	dataSourceFieldExportsNode := transformToDataSourceFieldExportsNode(dataSource.Exports, dataSourceNode.Path)
	if dataSourceFieldExportsNode != nil {
		children = append(children, dataSourceFieldExportsNode)
	}

	descriptionNode := transformToStringSubsNode("description", dataSource.Description, dataSourceNode.Path)
	if descriptionNode != nil {
		children = append(children, descriptionNode)
	}

	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	dataSourceNode.Children = children

	return dataSourceNode
}

func transformToDataSourceTypeNode(dataSourceType *DataSourceTypeWrapper, parentPath string) *TreeNode {
	if dataSourceType == nil {
		return nil
	}

	return &TreeNode{
		Label:         "type",
		Path:          fmt.Sprintf("%s/type", parentPath),
		Type:          TreeNodeTypeLeaf,
		SchemaElement: dataSourceType,
		Range: &source.Range{
			Start: dataSourceType.SourceMeta,
		},
	}
}

func transformToDataSourceMetataNode(metadata *DataSourceMetadata, parentPath string) *TreeNode {
	if metadata == nil {
		return nil
	}

	metadataNode := &TreeNode{
		Label:         "metadata",
		Path:          fmt.Sprintf("%s/metadata", parentPath),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: metadata,
		Range: &source.Range{
			Start: metadata.SourceMeta,
		},
	}

	children := []*TreeNode{}

	displayNameNode := transformToStringSubsNode("displayName", metadata.DisplayName, metadataNode.Path)
	if displayNameNode != nil {
		children = append(children, displayNameNode)
	}

	annotationsNode := trnasformToStringSubsMapNode("annotations", metadata.Annotations, metadataNode.Path)
	if annotationsNode != nil {
		children = append(children, annotationsNode)
	}

	customNode := transformToMappingNode("custom", metadata.Custom, metadataNode.Path, nil)
	if customNode != nil {
		children = append(children, customNode)
	}

	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	metadataNode.Children = children

	return metadataNode
}

func transformToDataSourceFilterNode(filter *DataSourceFilter, parentPath string) *TreeNode {
	if filter == nil {
		return nil
	}

	filterNode := &TreeNode{
		Label:         "filter",
		Path:          fmt.Sprintf("%s/filter", parentPath),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: filter,
		Range: &source.Range{
			Start: filter.SourceMeta,
		},
	}

	children := []*TreeNode{}

	fieldNode := transformToScalarNode("field", filter.Field, filterNode.Path)
	if fieldNode != nil {
		children = append(children, fieldNode)
	}

	opNode := transformToFilterOperatorNode(filter.Operator, filterNode.Path)
	if opNode != nil {
		children = append(children, opNode)
	}

	searchNode := transformToFilterSearchNode(filter.Search, filterNode.Path)
	if searchNode != nil {
		children = append(children, searchNode)
	}

	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	filterNode.Children = children

	return filterNode
}

func transformToFilterOperatorNode(operator *DataSourceFilterOperatorWrapper, parentPath string) *TreeNode {
	if operator == nil {
		return nil
	}

	opNode := &TreeNode{
		Label:         "operator",
		Path:          fmt.Sprintf("%s/operator", parentPath),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: operator,
		Range: &source.Range{
			Start: operator.SourceMeta,
		},
		Children: []*TreeNode{
			{
				Label:         string(operator.Value),
				Path:          fmt.Sprintf("%s/operator/%s", parentPath, operator.Value),
				Type:          TreeNodeTypeLeaf,
				SchemaElement: operator.Value,
				Range: &source.Range{
					Start: operator.SourceMeta,
				},
			},
		},
	}

	return opNode
}

func transformToFilterSearchNode(search *DataSourceFilterSearch, parentPath string) *TreeNode {
	if search == nil {
		return nil
	}

	searchNode := &TreeNode{
		Label:         "search",
		Path:          fmt.Sprintf("%s/search", parentPath),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: search,
		Range: &source.Range{
			Start: search.SourceMeta,
		},
	}

	if len(search.Values) == 0 {
		searchNode.Type = TreeNodeTypeLeaf
		return searchNode
	}

	children := make([]*TreeNode, len(search.Values))
	for i, val := range search.Values {
		child := transformToStringSubsNode(fmt.Sprintf("%d", i), val, searchNode.Path)
		if child != nil {
			children[i] = child
		}
	}

	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	searchNode.Children = children

	return searchNode
}

func transformToDataSourceFieldExportsNode(exports *DataSourceFieldExportMap, parentPath string) *TreeNode {
	if exports == nil || len(exports.Values) == 0 {
		return nil
	}

	exportsNode := &TreeNode{
		Label:         "exports",
		Path:          fmt.Sprintf("%s/exports", parentPath),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: exports,
		Range: &source.Range{
			Start: minPosition(core.MapToSlice(exports.SourceMeta)),
		},
	}

	children := []*TreeNode{}
	for exportName, val := range exports.Values {
		exportNode := transformToDataSourceFieldExportNode(
			exportName,
			val,
			parentPath,
			exports.SourceMeta[exportName],
		)
		if exportNode != nil {
			children = append(children, exportNode)
		}
	}

	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	exportsNode.Children = children

	return exportsNode
}

func transformToDataSourceFieldExportNode(exportName string, export *DataSourceFieldExport, parentPath string, location *source.Meta) *TreeNode {
	if export == nil {
		return nil
	}

	fieldExportNode := &TreeNode{
		Label:         exportName,
		Path:          fmt.Sprintf("%s/%s", parentPath, exportName),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: export,
		Range: &source.Range{
			Start: location,
		},
	}

	children := []*TreeNode{}

	exportTypeNode := transformToDataSourceFieldExportTypeNode(export.Type, fieldExportNode.Path)
	if exportTypeNode != nil {
		children = append(children, exportTypeNode)
	}

	aliasForNode := transformToScalarNode("aliasFor", export.AliasFor, fieldExportNode.Path)
	if aliasForNode != nil {
		children = append(children, aliasForNode)
	}

	descriptionNode := transformToStringSubsNode("description", export.Description, fieldExportNode.Path)
	if descriptionNode != nil {
		children = append(children, descriptionNode)
	}

	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	fieldExportNode.Children = children

	return fieldExportNode
}

func transformToDataSourceFieldExportTypeNode(dataSourceType *DataSourceFieldTypeWrapper, parentPath string) *TreeNode {
	if dataSourceType == nil {
		return nil
	}

	return &TreeNode{
		Label:         "type",
		Path:          fmt.Sprintf("%s/type", parentPath),
		Type:          TreeNodeTypeLeaf,
		SchemaElement: dataSourceType,
		Range: &source.Range{
			Start: dataSourceType.SourceMeta,
		},
	}
}

func transformToExportsNode(exports *ExportMap, parentPath string) *TreeNode {
	if exports == nil || len(exports.Values) == 0 {
		return nil
	}

	exportsNode := &TreeNode{
		Label:         "exports",
		Path:          fmt.Sprintf("%s/exports", parentPath),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: exports,
		Range: &source.Range{
			Start: minPosition(core.MapToSlice(exports.SourceMeta)),
		},
	}

	children := []*TreeNode{}
	for exportName, val := range exports.Values {
		exportNode := transformToExportNode(
			exportName,
			val,
			parentPath,
			exports.SourceMeta[exportName],
		)
		if exportNode != nil {
			children = append(children, exportNode)
		}
	}
	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	exportsNode.Children = children

	return exportsNode
}

func transformToExportNode(exportName string, export *Export, parentPath string, location *source.Meta) *TreeNode {
	exportNode := &TreeNode{
		Label:         exportName,
		Path:          fmt.Sprintf("%s/%s", parentPath, exportName),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: export,
		Range: &source.Range{
			Start: location,
		},
	}

	children := []*TreeNode{}

	exportTypeNode := transformToExportTypeNode(export.Type, exportNode.Path)
	if exportTypeNode != nil {
		children = append(children, exportTypeNode)
	}

	fieldNode := transformToScalarNode("field", export.Field, exportNode.Path)
	if fieldNode != nil {
		children = append(children, fieldNode)
	}

	descriptionNode := transformToStringSubsNode("description", export.Description, exportNode.Path)
	if descriptionNode != nil {
		children = append(children, descriptionNode)
	}

	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	exportNode.Children = children

	return exportNode
}

func transformToExportTypeNode(exportType *ExportTypeWrapper, parentPath string) *TreeNode {
	if exportType == nil {
		return nil
	}

	return &TreeNode{
		Label:         "type",
		Path:          fmt.Sprintf("%s/type", parentPath),
		Type:          TreeNodeTypeLeaf,
		SchemaElement: exportType,
		Range: &source.Range{
			Start: exportType.SourceMeta,
		},
	}
}

func trnasformToStringSubsMapNode(label string, subsMap *StringOrSubstitutionsMap, parentPath string) *TreeNode {
	if subsMap == nil || len(subsMap.Values) == 0 {
		return nil
	}

	subsMapNode := &TreeNode{
		Label:         label,
		Path:          fmt.Sprintf("%s/%s", parentPath, label),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: subsMap,
		Range: &source.Range{
			Start: minPosition(core.MapToSlice(subsMap.SourceMeta)),
		},
	}

	children := []*TreeNode{}
	for key, value := range subsMap.Values {
		child := transformToStringSubsNode(
			key,
			value,
			subsMapNode.Path,
		)
		if child != nil {
			children = append(children, child)
		}
	}

	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	subsMapNode.Children = children

	return subsMapNode
}

func transformToStringMapNode(label string, stringMap *StringMap, parentPath string) *TreeNode {
	if stringMap == nil || len(stringMap.Values) == 0 {
		return nil
	}

	subsMapNode := &TreeNode{
		Label:         label,
		Path:          fmt.Sprintf("%s/%s", parentPath, label),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: stringMap,
		Range: &source.Range{
			Start: minPosition(core.MapToSlice(stringMap.SourceMeta)),
		},
	}

	children := []*TreeNode{}
	for key, value := range stringMap.Values {
		child := transformToStringNode(
			value,
			stringMap.SourceMeta[key],
			subsMapNode.Path,
		)
		if child != nil {
			children = append(children, child)
		}
	}

	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	subsMapNode.Children = children

	return subsMapNode
}

func transformToMappingNode(
	label string,
	mappingNode *bpcore.MappingNode,
	parentPath string,
	// This is expected to be provided for elements in `MappingNode.Fields` to capture
	// the start location of the key/attribute name in the source document
	// as the start location.
	startPosition *source.Meta,
) *TreeNode {
	if mappingNode == nil {
		return nil
	}

	mappingNodePath := fmt.Sprintf("%s/%s", parentPath, label)
	finalStartPosition := startPosition
	if finalStartPosition == nil {
		finalStartPosition = mappingNode.SourceMeta
	}
	mappingTreeNode := &TreeNode{
		Label:         label,
		Path:          mappingNodePath,
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: mappingNode,
		Range: &source.Range{
			Start: finalStartPosition,
		},
	}

	if mappingNode.Literal != nil {
		literalNode := transformToScalarNode("scalar", mappingNode.Literal, mappingNodePath)
		if literalNode != nil {
			mappingTreeNode.Children = []*TreeNode{literalNode}
		}
	}

	if mappingNode.StringWithSubstitutions != nil {
		stringSubsNode := transformToStringSubsNode(
			"stringSubs",
			mappingNode.StringWithSubstitutions,
			mappingNodePath,
		)
		if stringSubsNode != nil {
			mappingTreeNode.Children = []*TreeNode{stringSubsNode}
		}
	}

	if mappingNode.Fields != nil {
		children := []*TreeNode{}
		for key, value := range mappingNode.Fields {
			node := transformToMappingNode(
				key, value, mappingNodePath, mappingNode.FieldsSourceMeta[key],
			)
			if node != nil {
				children = append(children, node)
			}
		}

		sortTreeNodes(children)
		setSortedNodesRangeEnd(children)
		mappingTreeNode.Children = children
	}

	if mappingNode.Items != nil {
		children := make([]*TreeNode, len(mappingNode.Items))
		for i, item := range mappingNode.Items {
			node := transformToMappingNode(
				fmt.Sprintf("%d", i), item, mappingNodePath, nil,
			)
			if node != nil {
				children[i] = node
			}
		}

		sortTreeNodes(children)
		setSortedNodesRangeEnd(children)
		mappingTreeNode.Children = children
	}

	return mappingTreeNode
}

func transformToScalarsNode(label string, scalars []*bpcore.ScalarValue, parentPath string) *TreeNode {
	if len(scalars) == 0 {
		return nil
	}

	scalarsPath := fmt.Sprintf("%s/%s", parentPath, label)
	scalarsNode := &TreeNode{
		Label: label,
		Path:  scalarsPath,
		Type:  TreeNodeTypeNonTerminal,
		Range: &source.Range{
			Start: minPosition(core.Map(
				scalars,
				func(scalar *bpcore.ScalarValue, _ int) *source.Meta {
					return scalar.SourceMeta
				}),
			),
		},
	}

	children := make([]*TreeNode, len(scalars))
	for i, scalar := range scalars {
		scalarNode := transformToScalarNode(
			fmt.Sprintf("%d", i),
			scalar,
			scalarsPath,
		)
		if scalarNode != nil {
			children[i] = scalarNode
		}
	}

	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	scalarsNode.Children = children

	return scalarsNode
}

func transformToStringSubsNode(label string, subs *substitutions.StringOrSubstitutions, parentPath string) *TreeNode {
	if subs == nil {
		return nil
	}

	stringSubsNode := &TreeNode{
		Label:         label,
		Path:          fmt.Sprintf("%s/%s", parentPath, label),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: subs,
		Range: &source.Range{
			Start: subs.SourceMeta,
		},
	}

	children := make([]*TreeNode, len(subs.Values))
	for i, value := range subs.Values {
		var child *TreeNode
		if value.StringValue != nil {
			child = transformToStringNode(*value.StringValue, value.SourceMeta, stringSubsNode.Path)
		} else if value.SubstitutionValue != nil {
			child = transformToSubstitutionNode(value.SubstitutionValue, stringSubsNode.Path)
		}

		if child != nil {
			children[i] = child
		}
	}

	sortTreeNodes(children)
	setSortedNodesRangeEnd(children)
	stringSubsNode.Children = children

	return stringSubsNode
}

func transformToSubstitutionNode(value *substitutions.Substitution, parentPath string) *TreeNode {

	if value.StringValue != nil {
		return transformToStringNode(
			*value.StringValue,
			value.SourceMeta,
			parentPath,
		)
	}

	if value.IntValue != nil {
		return transformToSubIntNode(
			*value.IntValue,
			value.SourceMeta,
			parentPath,
		)
	}

	if value.BoolValue != nil {
		return transformToSubBoolNode(
			*value.BoolValue,
			value.SourceMeta,
			parentPath,
		)
	}

	if value.FloatValue != nil {
		return transformToSubFloatNode(
			*value.FloatValue,
			value.SourceMeta,
			parentPath,
		)
	}

	if value.Child != nil {
		return transformToSubChildNode(
			value.Child,
			parentPath,
		)
	}

	if value.Variable != nil {
		return transformToSubVariableNode(
			value.Variable,
			parentPath,
		)
	}

	if value.ValueReference != nil {
		return transformToSubValueRefNode(
			value.ValueReference,
			parentPath,
		)
	}

	if value.ResourceProperty != nil {
		return transformToSubResourcePropNode(
			value.ResourceProperty,
			parentPath,
		)
	}

	if value.DataSourceProperty != nil {
		return transformToSubDataSourcePropNode(
			value.DataSourceProperty,
			parentPath,
		)
	}

	if value.ElemReference != nil {
		return transformToSubElemRefNode(
			value.ElemReference,
			parentPath,
		)
	}

	if value.ElemIndexReference != nil {
		return transformToSubElemIndexRefNode(
			value.ElemIndexReference,
			parentPath,
		)
	}

	if value.Function != nil {
		return transformToSubFunctionNode(
			value.Function,
			parentPath,
		)
	}

	return nil
}

func transformToSubFunctionNode(value *substitutions.SubstitutionFunctionExpr, parentPath string) *TreeNode {

	treeNodeType := TreeNodeTypeNonTerminal
	if len(value.Arguments) == 0 {
		treeNodeType = TreeNodeTypeLeaf
	}

	root := &TreeNode{
		Label:         string(value.FunctionName),
		Path:          fmt.Sprintf("%s/functionCall/%s", parentPath, value.FunctionName),
		Type:          treeNodeType,
		SchemaElement: value,
		Range: &source.Range{
			Start: value.SourceMeta,
		},
	}

	children := make([]*TreeNode, len(value.Arguments))
	for i, arg := range value.Arguments {
		child := transformToSubFunctionArgNode(arg, i, root.Path)
		if child != nil {
			children[i] = child
		}
	}

	// Args are already in order so sorting is not needed.
	setSortedNodesRangeEnd(children)
	root.Children = children

	return root
}

func transformToSubFunctionArgNode(value *substitutions.SubstitutionFunctionArg, index int, parentPath string) *TreeNode {
	if value == nil {
		return nil
	}

	return &TreeNode{
		Label:         fmt.Sprintf("%d", index),
		Path:          fmt.Sprintf("%s/arg/%d", parentPath, index),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: value,
		Range: &source.Range{
			Start: value.SourceMeta,
		},
		Children: []*TreeNode{
			transformToSubstitutionNode(value.Value, parentPath),
		},
	}
}

func transformToSubChildNode(value *substitutions.SubstitutionChild, parentPath string) *TreeNode {

	return &TreeNode{
		Label:         value.ChildName,
		Path:          fmt.Sprintf("%s/childRef/%s", parentPath, value.ChildName),
		Type:          TreeNodeTypeLeaf,
		SchemaElement: value,
		Range: &source.Range{
			Start: value.SourceMeta,
		},
	}
}

func transformToSubVariableNode(value *substitutions.SubstitutionVariable, parentPath string) *TreeNode {

	return &TreeNode{
		Label:         value.VariableName,
		Path:          fmt.Sprintf("%s/varRef/%s", parentPath, value.VariableName),
		Type:          TreeNodeTypeLeaf,
		SchemaElement: value,
		Range: &source.Range{
			Start: value.SourceMeta,
		},
	}
}

func transformToSubValueRefNode(
	value *substitutions.SubstitutionValueReference,
	parentPath string,
) *TreeNode {

	return &TreeNode{
		Label:         value.ValueName,
		Path:          fmt.Sprintf("%s/valRef/%s", parentPath, value.ValueName),
		Type:          TreeNodeTypeLeaf,
		SchemaElement: value,
		Range: &source.Range{
			Start: value.SourceMeta,
		},
	}
}

func transformToSubResourcePropNode(
	value *substitutions.SubstitutionResourceProperty,
	parentPath string,
) *TreeNode {

	return &TreeNode{
		Label:         value.ResourceName,
		Path:          fmt.Sprintf("%s/resourceRef/%s", parentPath, value.ResourceName),
		Type:          TreeNodeTypeLeaf,
		SchemaElement: value,
		Range: &source.Range{
			Start: value.SourceMeta,
		},
	}
}

func transformToSubDataSourcePropNode(
	value *substitutions.SubstitutionDataSourceProperty,
	parentPath string,
) *TreeNode {

	return &TreeNode{
		Label:         value.DataSourceName,
		Path:          fmt.Sprintf("%s/datasourceRef/%s", parentPath, value.DataSourceName),
		Type:          TreeNodeTypeLeaf,
		SchemaElement: value,
		Range: &source.Range{
			Start: value.SourceMeta,
		},
	}
}

func transformToSubElemRefNode(
	value *substitutions.SubstitutionElemReference,
	parentPath string,
) *TreeNode {

	return &TreeNode{
		Label:         "elemRef",
		Path:          fmt.Sprintf("%s/elemRef", parentPath),
		Type:          TreeNodeTypeLeaf,
		SchemaElement: value,
		Range: &source.Range{
			Start: value.SourceMeta,
		},
	}
}

func transformToSubElemIndexRefNode(
	value *substitutions.SubstitutionElemIndexReference,
	parentPath string,
) *TreeNode {

	return &TreeNode{
		Label:         "elemIndexRef",
		Path:          fmt.Sprintf("%s/elemIndexRef", parentPath),
		Type:          TreeNodeTypeLeaf,
		SchemaElement: value,
		Range: &source.Range{
			Start: value.SourceMeta,
		},
	}
}

func transformToSubIntNode(value int64, location *source.Meta, parentPath string) *TreeNode {

	return &TreeNode{
		Label:         "int",
		Path:          fmt.Sprintf("%s/int", parentPath),
		Type:          TreeNodeTypeLeaf,
		SchemaElement: value,
		Range: &source.Range{
			Start: location,
		},
	}
}

func transformToSubFloatNode(value float64, location *source.Meta, parentPath string) *TreeNode {

	return &TreeNode{
		Label:         "float",
		Path:          fmt.Sprintf("%s/float", parentPath),
		Type:          TreeNodeTypeLeaf,
		SchemaElement: value,
		Range: &source.Range{
			Start: location,
		},
	}
}

func transformToSubBoolNode(value bool, location *source.Meta, parentPath string) *TreeNode {

	return &TreeNode{
		Label:         "bool",
		Path:          fmt.Sprintf("%s/bool", parentPath),
		Type:          TreeNodeTypeLeaf,
		SchemaElement: value,
		Range: &source.Range{
			Start: location,
		},
	}
}

func transformToStringNode(value string, location *source.Meta, parentPath string) *TreeNode {

	return &TreeNode{
		Label:         "string",
		Path:          fmt.Sprintf("%s/string", parentPath),
		Type:          TreeNodeTypeLeaf,
		SchemaElement: value,
		Range: &source.Range{
			Start: location,
		},
	}
}

func transformToScalarNode(label string, scalar *bpcore.ScalarValue, parentPath string) *TreeNode {
	if scalar == nil {
		return nil
	}

	return &TreeNode{
		Label:         label,
		Path:          fmt.Sprintf("%s/%s", parentPath, label),
		Type:          TreeNodeTypeLeaf,
		SchemaElement: scalar,
		Range: &source.Range{
			Start: scalar.SourceMeta,
		},
	}
}

func setSortedNodesRangeEnd(nodes []*TreeNode) {
	if len(nodes) == 0 {
		return
	}

	for i, node := range nodes {
		if i < len(nodes)-1 {
			nextNode := nodes[i+1]
			if node.Range != nil && nextNode.Range != nil {
				node.SetRangeEnd(nextNode.Range.Start)
			}
		}
	}
}

func sortTreeNodes(nodes []*TreeNode) {
	if len(nodes) == 0 {
		return
	}

	slices.SortFunc(nodes, func(a, b *TreeNode) int {
		if a.Range == nil || b.Range == nil ||
			a.Range.Start == nil || b.Range.Start == nil {
			return 0
		}

		if a.Range.Start.Line < b.Range.Start.Line {
			return -1
		}

		if a.Range.Start.Line == b.Range.Start.Line &&
			a.Range.Start.Column < b.Range.Start.Column {
			return -1
		}

		if a.Range.Start.Line == b.Range.Start.Line &&
			a.Range.Start.Column == b.Range.Start.Column {
			return 0
		}

		return 1
	})
}

func minPosition(positions []*source.Meta) *source.Meta {
	if len(positions) == 0 {
		return nil
	}

	min := positions[0]
	for _, pos := range positions {
		if pos.Line < min.Line || (pos.Line == min.Line && pos.Column < min.Column) {
			min = pos
		}
	}

	return min
}
