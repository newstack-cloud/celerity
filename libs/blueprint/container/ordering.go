package container

import (
	"fmt"
	"sort"
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/links"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/common/core"
)

// OrderLinksForDeployment deals with creating a flat ordered
// slice of chain links for stage changing and deployments.
// Ordering is determined by the priority resource type specified
// in each link implementation.
// It is a requirement for the input chains not to have any direct
// or transitive circular hard links.
// A hard link is when one resource type requires the other in a link
// relationship to be deployed first.
//
// For the following set of chains:
//
// (lt) = the linked to resource is the priority resource type.
// (lf) = the linked from resource is the priority resource type.
//
// *All the links in the example below are hard links.
//
// Chain 1
// ├── ResourceA1
// │	 ├── ResourceA2 (lf)
// │	 │   ├── ResourceA4 (lt)
// │	 │   └── ResourceA5 (lt)
// │	 └── ResourceA3 (lf)
// │	 	   └── ResourceA6 (lf)
//
// Chain 2
// ├── ResourceB1
// │	 ├── ResourceB2 (lt)
// │	 │   ├── ResourceB4 (lt)
// │   │   │   └── ResourceA6 (lt)
// │	 │   └── ResourceB5 (lt)
// │	 └── ResourceB3 (lf)
// │	 	   └── ResourceB6 (lt)
//
// We will want output like:
// [
//
//		ResourceA4,
//	 ResourceA5,
//	 ResourceA1,
//	 ResourceA2,
//	 ResourceA3,
//	 ResourceA6,
//	 ResourceB4,
//	 ResourceB5,
//	 ResourceB2,
//		ResourceB1,
//	 ResourceB6,
//	 ResourceB3
//
// ]
//
// What matters in the output is that resources are ordered by the priority
// definition of the links, the order of items that have no direct or transitive
// relationship are irrelevant.
func OrderLinksForDeployment(chains []*links.ChainLink) []*links.ChainLink {
	flattened := flattenChains(chains, []*links.ChainLink{})
	sort.Slice(flattened, func(i, j int) bool {
		linkA := flattened[i]
		linkB := flattened[j]

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
		isParentWithPriority := len(core.Filter(directParentsOfLinkB, hasPriorityOver(linkB))) > 0
		isChildWithPriority := len(core.Filter(linkB.LinksTo, hasPriorityOver(linkB))) > 0
		linkAHasPriority := isParentWithPriority || isChildWithPriority

		return (linkAIsAncestor || linkAIsDescendant) && linkAHasPriority
	})
	return flattened
}

func isResourceAncestor(resourceName string) func(string, int) bool {
	return func(path string, index int) bool {
		return strings.Contains(path, fmt.Sprintf("/%s", resourceName))
	}
}

func getDirectParentsForPaths(paths []string, link *links.ChainLink) []*links.ChainLink {
	return core.Filter(link.LinkedFrom, isLastInAtLeastOnePath(paths))
}

func isLastInAtLeastOnePath(paths []string) func(*links.ChainLink, int) bool {
	return func(candidateParentLink *links.ChainLink, index int) bool {
		return len(core.Filter(paths, isLastInPath(candidateParentLink))) > 0
	}
}

func isLastInPath(link *links.ChainLink) func(string, int) bool {
	return func(path string, index int) bool {
		return strings.HasSuffix(path, fmt.Sprintf("/%s", link.ResourceName))
	}
}

func hasPriorityOver(otherLink *links.ChainLink) func(*links.ChainLink, int) bool {
	return func(candidatePriorityLink *links.ChainLink, index int) bool {
		linkImplementation, hasLinkImplementation := candidatePriorityLink.LinkImplementations[otherLink.ResourceName]
		if !hasLinkImplementation {
			// The relationship could be either way.
			linkImplementation, hasLinkImplementation = otherLink.LinkImplementations[candidatePriorityLink.ResourceName]
		}

		if !hasLinkImplementation {
			// Might be a good idea to refactor this so we can return an error
			// somehow as something will be wrong somewhere in the code
			// if there is no link implementation.
			return false
		}

		priorityResourceType := linkImplementation.PriorityResourceType()
		isHardLink := linkImplementation.Type() == provider.LinkTypeHard
		return priorityResourceType == candidatePriorityLink.Resource.Type && isHardLink
	}
}

func flattenChains(chains []*links.ChainLink, flattenedAccum []*links.ChainLink) []*links.ChainLink {
	flattened := append([]*links.ChainLink{}, flattenedAccum...)
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
