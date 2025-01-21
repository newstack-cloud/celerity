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
	"github.com/two-hundred/celerity/libs/common/core"
)

// OrderItemsForDeployment deals with creating a flat ordered
// slice of chain link nodes and child blueprints for change staging and deployments.
// Ordering is determined by the priority resource type specified
// in each link implementation and usage of references between resources and child blueprints.
// A reference is treated as a hard link where the priority resource or child blueprint
// is the one being referenced.
//
// It is a requirement for the input chains and child blueprints to not have any direct
// or transitive circular hard links.
// A hard link is when one resource type requires the other in a link
// relationship to be deployed first or where a reference is made to a
// resource or child blueprint.
//
// For the following set of chains and child blueprints:
//
// (lt)                = the linked to resource is the priority resource type.
// (lf)                = the linked from resource is the priority resource type.
// (rb:{referencedBy}) = referenced by other item.
//
// *All the links in the example below are hard links.
//
// Chain 1
// ├── ResourceA1
// │	 ├── ResourceA2 (lf)
// │	 │   ├── ResourceA4 (lt)
// │	 │   └── ResourceA5 (lt)
// │	 └── ResourceA3 (lf)
// │	 	 └── ResourceA6 (lf)
//
// Chain 2
// ├── ResourceB1
// │	 ├── ResourceB2 (lt)
// │	 │   ├── ResourceB4 (lt) (rb:ResourceA5)
// │     │   │   └── ResourceA6 (lt)
// │	 │   └── ResourceB5 (lt) (rb:Child2)
// │	 └── ResourceB3 (lf)
// │	 	 └── ResourceB6 (lt)
//
// Child1 (rb:ResourceA4)
//
// Child2 (rb:ResourceA5)
//
// We will want output like:
// [
//
//		Child1,
//		ResourceA4,
//		ResourceA1,
//		ResourceA3,
//		ResourceA6,
//		ResourceB4,
//		ResourceB5,
//	 	Child2,
//		ResourceA5,
//		ResourceA2,
//		ResourceB2,
//		ResourceB1,
//		ResourceB6,
//		ResourceB3
//
// ]
//
// What matters in the output is that resources are ordered by the priority
// definition of the links and based on references (treated the same as hard links),
// the order of items that have no direct or transitive relationship are irrelevant.
func OrderItemsForDeployment(
	ctx context.Context,
	chains []*links.ChainLinkNode,
	children []*validation.ReferenceChainNode,
	refChainCollector validation.RefChainCollector,
	params bpcore.BlueprintParams,
) ([]*DeploymentNode, error) {
	flattened := flattenChains(chains, []*links.ChainLinkNode{})
	combined := combineChainsAndChildren(flattened, children)
	var sortErr error

	// Compare each node to every other node to determine the order based on dependencies
	// that can occur by implicit links, references or explicit `dependsOn` declarations.
	// Each node must be compared to every other node as when trying to use an efficient
	// sort algorithm, important comparisons may be missed.
	sortCompareAll(combined, func(nodeA, nodeB *DeploymentNode) int {

		if nodeA.Type() == DeploymentNodeTypeResource &&
			nodeB.Type() == DeploymentNodeTypeResource {
			return checkResourceAPriority(
				ctx,
				nodeA.ChainLinkNode,
				nodeB.ChainLinkNode,
				refChainCollector,
				params,
				&sortErr,
			)
		}

		if nodeA.Type() == DeploymentNodeTypeResource &&
			nodeB.Type() == DeploymentNodeTypeChild {
			return checkResourcePriorityOverChild(
				nodeA.ChainLinkNode,
				nodeB.ChildNode,
				refChainCollector,
			)
		}

		if nodeA.Type() == DeploymentNodeTypeChild &&
			nodeB.Type() == DeploymentNodeTypeResource {
			return checkChildPriorityOverResource(
				nodeA.ChildNode,
				nodeB.ChainLinkNode,
				refChainCollector,
			)
		}

		if nodeA.Type() == DeploymentNodeTypeChild &&
			nodeB.Type() == DeploymentNodeTypeChild {
			return checkChildAHasPriority(
				nodeA.ChildNode,
				nodeB.ChildNode,
			)
		}

		return 1
	})
	return combined, sortErr
}

func checkResourceAPriority(
	ctx context.Context,
	linkA *links.ChainLinkNode,
	linkB *links.ChainLinkNode,
	refChainCollector validation.RefChainCollector,
	params bpcore.BlueprintParams,
	sortErr *error,
) int {
	pathsWithLinkA := core.Filter(linkB.Paths, isResourceAncestor(linkA.ResourceName))
	linkAIsAncestor := len(pathsWithLinkA) > 0

	pathsWithLinkB := core.Filter(linkA.Paths, isResourceAncestor(linkB.ResourceName))
	linkAIsDescendant := len(pathsWithLinkB) > 0

	directParentsOfLinkB := getDirectParentsForPaths(pathsWithLinkA, linkB)

	// link A has priority in two cases.
	// 1, if at least one of the direct parents of link B
	// (for which link A is an ancestor) is the priority resource type
	// in the link relationship.
	// 2, if at least one of the direct children of link B
	// (for which link A is a descendant) is the priority resource type
	// in the link relationship.
	parentPriorityInfo, internalSortErr := checkLinkPriority(ctx, directParentsOfLinkB, linkB, params)
	if internalSortErr != nil {
		*sortErr = internalSortErr
		return -1
	}
	isParentWithPriority := parentPriorityInfo.hasPriority

	// Only check children of link B for priority if they are ancestors of link A.
	filteredLinkBChildren := getAncestors(linkA, linkB.LinksTo)
	childPriorityInfo, internalSortErr := checkLinkPriority(ctx, filteredLinkBChildren, linkB, params)
	if internalSortErr != nil {
		*sortErr = internalSortErr
		return -1
	}
	isChildWithPriority := childPriorityInfo.hasPriority

	noPriorityBetweenLinks := !parentPriorityInfo.hasHardLinks && !childPriorityInfo.hasHardLinks

	// If A references B or any of B's descendants then A does not have priority regardless
	// of the link relationship. (An explicit reference is a dependency)
	linkAReferencesLinkB := linkResourceReferences(refChainCollector, linkA, linkB)
	linkAHasPriority := (isParentWithPriority || isChildWithPriority) && !linkAReferencesLinkB

	// If link B references link A but is not connected via a link relationship,
	// then link A has priority.
	// For example, let's say link A is an "orders" NoSQL table in a blueprint
	// and link B is a "createOrders" serverless function.
	// The "createOrders" function references the "orders" table in its environment variables
	// as the source for the table name made available to the function code.
	// There is no linkSelector initated link between the two resources, however, the "orders"
	// table (link A) needs to be deployed before the "createOrders" function (link B) so the function can source
	// the table name from the environment variables.
	linkBReferencesLinkA := linkResourceReferences(refChainCollector, linkB, linkA)

	if linkBReferencesLinkA || ((linkAIsAncestor || linkAIsDescendant) && linkAHasPriority) {
		return -1
	}

	if noPriorityBetweenLinks && !linkAReferencesLinkB {
		// When there is no priority between the links and there is no reference between the links,
		// the order of the links is irrelevant.
		return 0
	}

	// Resource B has priority over resource A.
	return 1
}

func checkResourcePriorityOverChild(
	resourceNode *links.ChainLinkNode,
	childNode *validation.ReferenceChainNode,
	refChainCollector validation.RefChainCollector,
) int {
	resourceElementName := bpcore.ResourceElementID(resourceNode.ResourceName)
	resourceRef := refChainCollector.Chain(resourceElementName)
	if resourceRef == nil {
		return 0
	}

	// If resource (A) references child (B) or any of B's descendants then B has priority over A.
	resourceReferencesChild := referencesResourceOrDescendants(resourceElementName, resourceRef.References, childNode)
	if resourceReferencesChild {
		// B has priority over A so they should be swapped
		// so B comes first.
		return 1
	}

	return -1
}

func checkChildPriorityOverResource(
	childNode *validation.ReferenceChainNode,
	resourceNode *links.ChainLinkNode,
	refChainCollector validation.RefChainCollector,
) int {
	resourceElementName := bpcore.ResourceElementID(resourceNode.ResourceName)
	resourceRef := refChainCollector.Chain(resourceElementName)
	if resourceRef == nil {
		return 0
	}

	// If child (A) references resource (B) or any of B's descendants then B has priority.
	childReferencesResource := referencesResourceOrDescendants(childNode.ElementName, childNode.References, resourceRef)
	if childReferencesResource {
		// The resource (B) has priority over the child (A) so they should be swapped
		// so B comes first.
		return 1
	}

	return -1
}

func checkChildAHasPriority(
	childA *validation.ReferenceChainNode,
	childB *validation.ReferenceChainNode,
) int {
	// If child A references child B or any of B's descendants then B has priority.
	childAReferencesChildB := referencesResourceOrDescendants(childA.ElementName, childA.References, childB)
	if childAReferencesChildB {
		// B has priority over A so they should be swapped
		// so B comes first.
		return 1
	}

	return -1
}

func getDirectParentsForPaths(paths []string, link *links.ChainLinkNode) []*links.ChainLinkNode {
	return core.Filter(link.LinkedFrom, isLastInAtLeastOnePath(paths))
}

func isLastInAtLeastOnePath(paths []string) func(*links.ChainLinkNode, int) bool {
	return func(candidateParentLink *links.ChainLinkNode, index int) bool {
		return len(core.Filter(paths, isLastInPath(candidateParentLink))) > 0
	}
}

func isLastInPath(link *links.ChainLinkNode) func(string, int) bool {
	return func(path string, index int) bool {
		return strings.HasSuffix(path, fmt.Sprintf("/%s", link.ResourceName))
	}
}

func getAncestors(
	descendantLink *links.ChainLinkNode,
	candidateAncestors []*links.ChainLinkNode,
) []*links.ChainLinkNode {
	return core.Filter(candidateAncestors, isAncestorOf(descendantLink))
}

func isAncestorOf(descendantLink *links.ChainLinkNode) func(*links.ChainLinkNode, int) bool {
	return func(candidateAncestor *links.ChainLinkNode, index int) bool {
		return slices.ContainsFunc(descendantLink.Paths, isAncestorPath(candidateAncestor))
	}
}

func isAncestorPath(ancestorLink *links.ChainLinkNode) func(string) bool {
	return func(descendantPath string) bool {
		return strings.HasSuffix(descendantPath, fmt.Sprintf("/%s", ancestorLink.ResourceName))
	}
}

type linkPriorityInfo struct {
	hasPriority  bool
	hasHardLinks bool
}

func checkLinkPriority(
	ctx context.Context,
	candidatePriorityLinks []*links.ChainLinkNode,
	otherLink *links.ChainLinkNode,
	params bpcore.BlueprintParams,
) (*linkPriorityInfo, error) {
	priorityInfo := &linkPriorityInfo{
		hasPriority:  false,
		hasHardLinks: false,
	}

	i := 0
	for !priorityInfo.hasPriority && i < len(candidatePriorityLinks) {
		candidatePriorityLink := candidatePriorityLinks[i]
		linkImplementation, hasLinkImplementation := candidatePriorityLink.LinkImplementations[otherLink.ResourceName]
		candidatePriorityResource := provider.LinkPriorityResourceA
		if !hasLinkImplementation {
			// The relationship could be either way.
			linkImplementation, hasLinkImplementation = otherLink.LinkImplementations[candidatePriorityLink.ResourceName]
			candidatePriorityResource = provider.LinkPriorityResourceB
		}

		if hasLinkImplementation {
			linkCtx := provider.NewLinkContextFromParams(params)
			priorityResourceOutput, err := linkImplementation.GetPriorityResource(
				ctx,
				&provider.LinkGetPriorityResourceInput{
					LinkContext: linkCtx,
				},
			)
			if err != nil {
				return nil, err
			}

			kindOutput, err := linkImplementation.GetKind(ctx, &provider.LinkGetKindInput{
				LinkContext: linkCtx,
			})
			if err != nil {
				return nil, err
			}
			isHardLink := kindOutput.Kind == provider.LinkKindHard
			if isHardLink {
				priorityInfo.hasHardLinks = true
			}
			priorityInfo.hasPriority = priorityResourceOutput.PriorityResource == candidatePriorityResource && isHardLink
		}

		i += 1
	}

	return priorityInfo, nil
}

func linkResourceReferences(
	refChainCollector validation.RefChainCollector,
	linkA *links.ChainLinkNode,
	linkB *links.ChainLinkNode,
) bool {
	resourceRefA := refChainCollector.Chain(bpcore.ResourceElementID(linkA.ResourceName))
	resourceRefB := refChainCollector.Chain(bpcore.ResourceElementID(linkB.ResourceName))

	if resourceRefA == nil || resourceRefB == nil {
		return false
	}

	return referencesResourceOrDescendants(resourceRefA.ElementName, resourceRefA.References, resourceRefB)
}

func referencesResourceOrDescendants(
	referencedByElementName string,
	searchIn []*validation.ReferenceChainNode,
	searchFor *validation.ReferenceChainNode,
) bool {
	if len(searchIn) == 0 || searchFor == nil {
		return false
	}

	if slices.ContainsFunc(searchIn, compareElementNameForSubRef(referencedByElementName, searchFor)) {
		return true
	}

	// Ensure we skip links as they are handled separately.
	// Checking for descendants must only be for explicit references of resources
	// or dependencies through the use of the `dependsOn` field.
	isLinkReference := slices.Contains(
		searchFor.Tags,
		validation.CreateLinkTag(referencedByElementName),
	)

	if !isLinkReference {
		for _, childSearchFor := range searchFor.References {
			// Avoid cyclic references.
			if childSearchFor.ElementName != referencedByElementName &&
				referencesResourceOrDescendants(referencedByElementName, searchIn, childSearchFor) {
				return true
			}
		}
	}

	return false
}

func compareElementNameForSubRef(referencedByElementName string, searchFor *validation.ReferenceChainNode) func(*validation.ReferenceChainNode) bool {
	return func(current *validation.ReferenceChainNode) bool {
		return current.ElementName == searchFor.ElementName &&
			// Only match if the reference has a "subRef:{referencedFrom}"
			// tag or a "dependencyOf:{referencedFrom}" tag.
			// Links are collected to combine cycle detection logic for
			// links, explicit dependencies and references during the validation phase.
			// References and explicit dependencies are treated as hard links.
			// Tags are used to differentiate between links, dependencies and references
			// to allow this logic to skip links that are handled separately.
			slices.ContainsFunc(searchFor.Tags, func(tag string) bool {
				return tag == validation.CreateSubRefTag(referencedByElementName) ||
					tag == validation.CreateDependencyRefTag(referencedByElementName)
			})
	}
}

func isResourceAncestor(resourceName string) func(string, int) bool {
	return func(path string, index int) bool {
		return strings.Contains(path, fmt.Sprintf("/%s", resourceName))
	}
}

func flattenChains(chains []*links.ChainLinkNode, flattenedAccum []*links.ChainLinkNode) []*links.ChainLinkNode {
	flattened := append([]*links.ChainLinkNode{}, flattenedAccum...)
	for _, chain := range chains {
		if !core.SliceContains(flattened, chain) {
			flattened = append(flattened, chain)
			if len(chain.LinksTo) > 0 {
				flattened = flattenChains(chain.LinksTo, flattened)
			}
		}
	}
	return flattened
}

func combineChainsAndChildren(
	chains []*links.ChainLinkNode,
	children []*validation.ReferenceChainNode,
) []*DeploymentNode {
	deploymentNodes := []*DeploymentNode{}
	for _, chain := range chains {
		deploymentNodes = append(deploymentNodes, &DeploymentNode{
			ChainLinkNode:      chain,
			DirectDependencies: []*DeploymentNode{},
		})
	}
	for _, child := range children {
		deploymentNodes = append(deploymentNodes, &DeploymentNode{
			ChildNode:          child,
			DirectDependencies: []*DeploymentNode{},
		})
	}
	return deploymentNodes
}

func sortCompareAll(
	nodes []*DeploymentNode,
	// If a < b, return a negative integer.
	// If a > b, return a positive integer.
	// If a == b, return 0.
	compare func(a, b *DeploymentNode) int,
) {
	n := len(nodes)
	for {
		swapped := false
		for i := 1; i < n; i += 1 {
			result := compare(nodes[i-1], nodes[i])
			if result > 0 {
				nodes[i-1], nodes[i] = nodes[i], nodes[i-1]
				swapped = true
			}
		}
		if !swapped {
			break
		}
	}
}

// DeploymentNode is a node that represents a resource or a child blueprint
// to be deployed (or staged for deployment).
type DeploymentNode struct {
	ChainLinkNode *links.ChainLinkNode
	ChildNode     *validation.ReferenceChainNode
	// DirectDependencies holds the direct dependencies of the given deployment
	// node.
	// This isn't populated upon creation of the deployment node,
	// as ordering of the nodes does not compare every node,
	// the dependencies list would be incomplete.
	// This is primarily used for the deployment process where the container
	// will populate the direct dependencies of each node as a part of the
	// preparation phase.
	DirectDependencies []*DeploymentNode
}

func (d *DeploymentNode) Name() string {
	if d.ChainLinkNode != nil {
		return bpcore.ResourceElementID(d.ChainLinkNode.ResourceName)
	}
	return d.ChildNode.ElementName
}

func (d *DeploymentNode) Type() DeploymentNodeType {
	if d.ChainLinkNode != nil {
		return DeploymentNodeTypeResource
	}

	if d.ChildNode != nil {
		return DeploymentNodeTypeChild
	}

	return ""
}

// DeploymentNodeType is the type of a deployment node extracted
// from a source blueprint.
type DeploymentNodeType string

const (
	// DeploymentNodeTypeResource is a deployment node that represents a resource
	// to be deployed.
	DeploymentNodeTypeResource DeploymentNodeType = "resource"

	// DeploymentNodeTypeChild is a deployment node that represents a child blueprint
	// to be deployed.
	DeploymentNodeTypeChild DeploymentNodeType = "child"
)
