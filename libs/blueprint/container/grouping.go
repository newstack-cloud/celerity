package container

import (
	"context"
	"slices"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/links"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/validation"
)

// GroupOrderedNodes deals with grouping ordered deployment nodes
// for change staging and deployments to make the process
// more efficient by concurrently staging and deploying unrelated resources
// and child blueprints.
// The input is expected to be an ordered list of deployment nodes.
// The output is a list of groups of deployment nodes that can be staged or deployed
// concurrently, maintaining the order of the provided list for nodes that are
// connected.
func GroupOrderedNodes(
	ctx context.Context,
	orderedNodes []*DeploymentNode,
	refChainCollector validation.RefChainCollector,
	params bpcore.BlueprintParams,
) ([][]*DeploymentNode, error) {
	if len(orderedNodes) == 0 {
		return [][]*DeploymentNode{}, nil
	}

	currentGroupIndex := 0
	groups := [][]*DeploymentNode{{}}
	nodeGroupMap := map[string]int{}

	for _, node := range orderedNodes {
		hasReferenceInCurrentGroup := hasReferenceInGroup(
			node,
			refChainCollector,
			nodeGroupMap,
			currentGroupIndex,
		)

		var err error
		hasHardLinkInCurrentGroup := false
		if node.Type() == "resource" {
			hasHardLinkInCurrentGroup, err = hasHardLinkInGroup(
				ctx,
				node.ChainLinkNode,
				nodeGroupMap,
				currentGroupIndex,
				params,
			)
		}

		if err != nil {
			return nil, err
		}

		if hasReferenceInCurrentGroup || hasHardLinkInCurrentGroup {
			currentGroupIndex += 1
			newGroup := []*DeploymentNode{node}
			groups = append(groups, newGroup)
		} else {
			groups[currentGroupIndex] = append(groups[currentGroupIndex], node)
		}

		nodeGroupMap[node.Name()] = currentGroupIndex
	}

	return groups, nil
}

func hasReferenceInGroup(
	node *DeploymentNode,
	refChainCollector validation.RefChainCollector,
	nodeGroupMap map[string]int,
	currentGroupIndex int,
) bool {
	refChainNode := refChainCollector.Chain(node.Name())
	if refChainNode == nil {
		return false
	}

	hasReferenceInGroup := false
	i := 0
	for !hasReferenceInGroup && i < len(refChainNode.References) {
		reference := refChainNode.References[i]
		if groupIndex, ok := nodeGroupMap[reference.ElementName]; ok {
			hasReferenceInGroup = groupIndex == currentGroupIndex &&
				slices.ContainsFunc(reference.Tags, func(tag string) bool {
					return tag == validation.CreateSubRefTag(refChainNode.ElementName) ||
						tag == validation.CreateDependencyRefTag(refChainNode.ElementName)
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
		relatedElementName := bpcore.ResourceElementID(relatedNode.ResourceName)
		if groupIndex, ok := nodeGroupMap[relatedElementName]; ok {
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
