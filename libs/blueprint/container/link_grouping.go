package container

import (
	"context"
	"fmt"
	"slices"
	"strings"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/links"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/validation"
)

// GroupOrderedLinkNodes deals with grouping ordered links
// for change staging and deployments to make the process
// more efficient by concurrently staging and deploying unrelated resources.
// The input is expected to be an ordered list of link nodes.
// The output is a list of groups of chain link nodes that can be staged or deployed
// concurrently, maintaining the order of the provided list for chain link nodes that are
// connected.
func GroupOrderedLinkNodes(
	ctx context.Context,
	orderedLinkNodes []*links.ChainLinkNode,
	refChainCollector validation.RefChainCollector,
	params bpcore.BlueprintParams,
) ([][]*links.ChainLinkNode, error) {
	if len(orderedLinkNodes) == 0 {
		return [][]*links.ChainLinkNode{}, nil
	}

	currentGroupIndex := 0
	groups := [][]*links.ChainLinkNode{{}}
	nodeGroupMap := map[string]int{}

	for _, node := range orderedLinkNodes {
		hasReferenceInCurrentGroup := hasReferenceInGroup(
			node,
			refChainCollector,
			nodeGroupMap,
			currentGroupIndex,
		)
		hasHardLinkInCurrentGroup, err := hasHardLinkInGroup(
			ctx,
			node,
			nodeGroupMap,
			currentGroupIndex,
			params,
		)
		if err != nil {
			return nil, err
		}

		if hasReferenceInCurrentGroup || hasHardLinkInCurrentGroup {
			currentGroupIndex += 1
			newGroup := []*links.ChainLinkNode{node}
			groups = append(groups, newGroup)
		} else {
			groups[currentGroupIndex] = append(groups[currentGroupIndex], node)
		}

		nodeGroupMap[node.ResourceName] = currentGroupIndex
	}

	return groups, nil
}

func hasReferenceInGroup(
	node *links.ChainLinkNode,
	refChainCollector validation.RefChainCollector,
	nodeGroupMap map[string]int,
	currentGroupIndex int,
) bool {
	refChainNode := refChainCollector.Chain(fmt.Sprintf("resources.%s", node.ResourceName))
	if refChainNode == nil {
		return false
	}

	hasReferenceInGroup := false
	i := 0
	for !hasReferenceInGroup && i < len(refChainNode.References) {
		reference := refChainNode.References[i]
		resourceName := strings.TrimPrefix(reference.ElementName, "resources.")
		if groupIndex, ok := nodeGroupMap[resourceName]; ok {
			hasReferenceInGroup = groupIndex == currentGroupIndex &&
				slices.ContainsFunc(reference.Tags, func(tag string) bool {
					return tag == fmt.Sprintf("subRef:%s", refChainNode.ElementName) ||
						tag == fmt.Sprintf("dependencyOf:%s", refChainNode.ElementName)
				})
		}
		i += 1
	}

	return hasReferenceInGroup
}

func hasHardLinkInGroup(
	ctx context.Context,
	node *links.ChainLinkNode,
	nodeGroupMap map[string]int,
	currentGroupIndex int,
	params bpcore.BlueprintParams,
) (bool, error) {
	hasHardLinkInGroup := false
	relatedNodes := append(node.LinkedFrom, node.LinksTo...)
	i := 0
	for !hasHardLinkInGroup && i < len(relatedNodes) {
		relatedNode := relatedNodes[i]
		if groupIndex, ok := nodeGroupMap[relatedNode.ResourceName]; ok {
			linkImplementation, err := getLinkImplementation(node, relatedNode)
			if err != nil {
				return false, err
			}

			// Only check if the link is hard as the nodes passed in to GroupOrderedLinkNodes
			// are expected to be ordered taking into account the priority resource type
			// of the links.
			linkKindOutput, err := linkImplementation.GetKind(ctx, &provider.LinkGetKindInput{
				Params: params,
			})
			if err != nil {
				return false, err
			}

			hasHardLinkInGroup = groupIndex == currentGroupIndex &&
				linkKindOutput.Kind == provider.LinkKindHard
		}
		i += 1
	}

	return hasHardLinkInGroup, nil
}
