package container

import (
	"context"
	"fmt"
	"slices"
	"sort"
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
	sort.Slice(combined, func(i, j int) bool {
		nodeA := combined[i]
		nodeB := combined[j]

		if nodeA.Type() == DeploymentNodeTypeResource &&
			nodeB.Type() == DeploymentNodeTypeResource {
			return resourceAHasPriority(
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
			return resourceHasPriorityOverChild(
				nodeA.ChainLinkNode,
				nodeB.ChildNode,
				refChainCollector,
			)
		}

		if nodeA.Type() == DeploymentNodeTypeChild &&
			nodeB.Type() == DeploymentNodeTypeResource {
			return childHasPriorityOverResource(
				nodeA.ChildNode,
				nodeB.ChainLinkNode,
				refChainCollector,
			)
		}

		if nodeA.Type() == DeploymentNodeTypeChild &&
			nodeB.Type() == DeploymentNodeTypeChild {
			return childAHasPriority(
				nodeA.ChildNode,
				nodeB.ChildNode,
			)
		}

		return false
	})
	return combined, sortErr
}

func resourceAHasPriority(
	ctx context.Context,
	linkA *links.ChainLinkNode,
	linkB *links.ChainLinkNode,
	refChainCollector validation.RefChainCollector,
	params bpcore.BlueprintParams,
	sortErr *error,
) bool {

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
	var internalSortErr error
	isParentWithPriority := len(core.Filter(directParentsOfLinkB, hasPriorityOver(ctx, linkB, params, &internalSortErr))) > 0
	if internalSortErr != nil {
		*sortErr = internalSortErr
		return false
	}
	isChildWithPriority := len(core.Filter(linkB.LinksTo, hasPriorityOver(ctx, linkB, params, &internalSortErr))) > 0
	if internalSortErr != nil {
		*sortErr = internalSortErr
		return false
	}
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

	return linkBReferencesLinkA || ((linkAIsAncestor || linkAIsDescendant) && linkAHasPriority)
}

func resourceHasPriorityOverChild(
	resourceNode *links.ChainLinkNode,
	childNode *validation.ReferenceChainNode,
	refChainCollector validation.RefChainCollector,
) bool {
	resourceElementName := bpcore.ResourceElementID(resourceNode.ResourceName)
	resourceRef := refChainCollector.Chain(resourceElementName)
	if resourceRef == nil {
		return false
	}

	// If resource (A) references child (B) or any of B's descendants then A does not have priority.
	resourceReferencesChild := referencesResourceOrDescendants(resourceElementName, resourceRef.References, childNode)

	return !resourceReferencesChild
}

func childHasPriorityOverResource(
	childNode *validation.ReferenceChainNode,
	resourceNode *links.ChainLinkNode,
	refChainCollector validation.RefChainCollector,
) bool {
	resourceElementName := bpcore.ResourceElementID(resourceNode.ResourceName)
	resourceRef := refChainCollector.Chain(resourceElementName)
	if resourceRef == nil {
		return false
	}

	// If child (A) references resource (B) or any of B's descendants then B has priority.
	childReferencesResource := referencesResourceOrDescendants(childNode.ElementName, childNode.References, resourceRef)

	return !childReferencesResource
}

func childAHasPriority(
	childA *validation.ReferenceChainNode,
	childB *validation.ReferenceChainNode,
) bool {
	// If child A references child B or any of B's descendants then A does not have priority.
	childAReferencesChildB := referencesResourceOrDescendants(childA.ElementName, childA.References, childB)
	return !childAReferencesChildB
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

func hasPriorityOver(
	ctx context.Context,
	otherLink *links.ChainLinkNode,
	params bpcore.BlueprintParams,
	captureError *error,
) func(*links.ChainLinkNode, int) bool {
	return func(candidatePriorityLink *links.ChainLinkNode, index int) bool {
		linkImplementation, hasLinkImplementation := candidatePriorityLink.LinkImplementations[otherLink.ResourceName]
		candidatePriorityResource := provider.LinkPriorityResourceA
		if !hasLinkImplementation {
			// The relationship could be either way.
			linkImplementation, hasLinkImplementation = otherLink.LinkImplementations[candidatePriorityLink.ResourceName]
			candidatePriorityResource = provider.LinkPriorityResourceB
		}

		if !hasLinkImplementation {
			// Might be a good idea to refactor this so we can return an error
			// somehow as something will be wrong somewhere in the code
			// if there is no link implementation.
			return false
		}

		linkCtx := provider.NewLinkContextFromParams(params)
		priorityResourceOutput, err := linkImplementation.GetPriorityResource(
			ctx,
			&provider.LinkGetPriorityResourceInput{
				LinkContext: linkCtx,
			},
		)
		if err != nil {
			*captureError = err
			return false
		}

		kindOutput, err := linkImplementation.GetKind(ctx, &provider.LinkGetKindInput{
			LinkContext: linkCtx,
		})
		if err != nil {
			*captureError = err
			return false
		}
		isHardLink := kindOutput.Kind == provider.LinkKindHard
		return priorityResourceOutput.PriorityResource == candidatePriorityResource && isHardLink
	}
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

	for _, childSearchFor := range searchFor.References {
		if referencesResourceOrDescendants(referencedByElementName, searchIn, childSearchFor) {
			return true
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
