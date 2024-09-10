package schema

import (
	"fmt"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/source"
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
	// Range is the source range of the tree node in the source code.
	Range *source.Range
}

// SetRangeEnd sets the range end of a tree node and the last child.
// This is applied recursively to the last child of the last child and so on.
func (n *TreeNode) SetRangeEnd(end *source.Meta) {
	if n.Range == nil {
		n.Range = &source.Range{
			Start: n.Range.Start,
			End:   end,
		}
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
		Children:      []*TreeNode{},
	}

	versionNode := versionToTreeNode(blueprint.Version, root.Path)
	transformNode := transformToTreeNode(blueprint.Transform, root.Path)
	variablesNode := transformToVariablesNode(blueprint.Variables, root.Path)

	root.Children = append(root.Children, versionNode)
	root.Children = append(root.Children, transformNode)
	root.Children = append(root.Children, variablesNode)

	// TODO: based on the locations of the siblings, add them to the root node
	// in order of location, attaching end locations to the previous node
	// accordingly.

	// TODO: Once we have processed all of the blueprint, we can set the range
	// of the root node as we will have gathered the end position at this point.
	root.Range = &source.Range{
		Start: &source.Meta{
			Line:   1,
			Column: 1,
		},
		// End: endLocation,
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
		children = append(children, variableNode)
	}
	variablesNode.Children = children

	return variablesNode
}

func transformToVariableNode(varName string, variable *Variable, parentPath string, location *source.Meta) *TreeNode {
	variableNode := &TreeNode{
		Label:         varName,
		Path:          fmt.Sprintf("%s/variables/%s", parentPath, varName),
		Type:          TreeNodeTypeNonTerminal,
		SchemaElement: variable,
		Range: &source.Range{
			Start: location,
		},
	}

	return variableNode
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
