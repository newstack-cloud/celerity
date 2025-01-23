package refgraph

import (
	"fmt"
	"slices"
)

// ReferenceSortDirection is an enum type for the direction of topological sorting
// for reference chains.
type ReferenceSortDirection int

const (
	// ReferenceSortDirectionReferencedBy is the direction for sorting reference chains
	// based on the "referenced by" relationship.
	// For example, this could be used to order elements to be deployed in a blueprint,
	// making sure that resources are deployed after their dependencies.
	ReferenceSortDirectionReferencedBy ReferenceSortDirection = iota
	// ReferenceSortDirectionReferences is the direction for sorting reference chains
	// based on the "references" relationship.
	// For example, this could be used to order elements to be removed in a blueprint
	// for a blueprint, making sure that resources are removed before their dependencies.
	ReferenceSortDirectionReferences
)

// UniqueName is an interface for types that have a unique name.
type UniqueName interface {
	Name() string
}

// TopologicalSortReferences sorts a list of items based on the reference chains
// provided.
// Each item in the list is expected to have a unique name that corresponds the
// `ElementName` field in the reference chain nodes.
// The direction of the sort is determined by the direction parameter.
func TopologicalSortReferences[Item UniqueName](
	chains []*ReferenceChainNode,
	items []Item,
	direction ReferenceSortDirection,
	// The empty value that should be used when a reference chain node
	// does not match an item in the list.
	empty Item,
) ([]Item, error) {
	sorted := []Item{}

	for len(chains) > 0 {
		n := chains[0]
		chains = chains[1:]
		nItem, hasItem := getItemInList(n.ElementName, items, empty)
		if hasItem {
			sorted = append(sorted, nItem)
		}
		// Depending on the direction, edges are either "referenced by" or "references"
		// relationships in the graph.
		connectedNodes := getConnectedNodes(n, direction)
		for _, m := range connectedNodes {
			removeEdge(n, m, direction)
			if !hasEdges(m, getReverseDirection(direction)) {
				chains = append(chains, m)
			}
		}
	}

	if len(sorted) != len(items) {
		return nil, fmt.Errorf("circular reference detected")
	}

	return sorted, nil
}

func getItemInList[Item UniqueName](elementName string, list []Item, empty Item) (Item, bool) {
	for _, current := range list {
		if current.Name() == elementName {
			return current, true
		}
	}
	return empty, false
}

func hasEdges(
	checkEdgesForNode *ReferenceChainNode,
	direction ReferenceSortDirection,
) bool {
	connectedNodes := getConnectedNodes(checkEdgesForNode, direction)
	return len(connectedNodes) > 0
}

func removeEdge(
	node *ReferenceChainNode,
	otherNode *ReferenceChainNode,
	direction ReferenceSortDirection,
) {
	if direction == ReferenceSortDirectionReferencedBy {
		otherNodeIndex := slices.IndexFunc(node.ReferencedBy, compareChainNode(otherNode))
		if otherNodeIndex > -1 {
			node.ReferencedBy = append(
				node.ReferencedBy[:otherNodeIndex],
				node.ReferencedBy[otherNodeIndex+1:]...,
			)
		}
		// Chain nodes are bidirectional,
		// so we also need to remove the edge from the other node.
		nodeIndex := slices.IndexFunc(otherNode.References, compareChainNode(node))
		if nodeIndex > -1 {
			otherNode.References = append(
				otherNode.References[:nodeIndex],
				otherNode.References[nodeIndex+1:]...,
			)
		}
		return
	}

	otherNodeIndex := slices.IndexFunc(node.References, compareChainNode(otherNode))
	if otherNodeIndex > -1 {
		node.References = append(
			node.References[:otherNodeIndex],
			node.References[otherNodeIndex+1:]...,
		)
	}
	nodeIndex := slices.IndexFunc(otherNode.ReferencedBy, compareChainNode(node))
	if nodeIndex > -1 {
		otherNode.ReferencedBy = append(
			otherNode.ReferencedBy[:nodeIndex],
			otherNode.ReferencedBy[nodeIndex+1:]...,
		)
	}
}

func compareChainNode(
	searchFor *ReferenceChainNode,
) func(node *ReferenceChainNode) bool {
	return func(node *ReferenceChainNode) bool {
		return node.ElementName == searchFor.ElementName
	}
}

func getReverseDirection(direction ReferenceSortDirection) ReferenceSortDirection {
	if direction == ReferenceSortDirectionReferencedBy {
		return ReferenceSortDirectionReferences
	}
	return ReferenceSortDirectionReferencedBy
}

func getConnectedNodes(
	node *ReferenceChainNode,
	direction ReferenceSortDirection,
) []*ReferenceChainNode {
	// Make a copy of the node collection to allow for iterating over nodes
	// while modifying the original list (edges).
	if direction == ReferenceSortDirectionReferencedBy {
		return append([]*ReferenceChainNode{}, node.ReferencedBy...)
	}
	return append([]*ReferenceChainNode{}, node.References...)
}
