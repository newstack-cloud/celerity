package validation

import (
	"fmt"
	"strings"
)

// ReferenceChainNode provides a node in a chain of references that contains the name
// of the current element in the chain.
type ReferenceChainNode struct {
	// ElementName is the unique name in the spec for the current
	// element in the chain.
	ElementName string
	// Element holds the information about the element at the blueprint spec schema-level.
	Element interface{}
	// References holds the references that the current element makes to other elements
	// in the blueprint.
	References []*ReferenceChainNode
	// ReferencedBy holds the references that other elements make to the current element.
	ReferencedBy []*ReferenceChainNode
	// Paths holds all the different "routes" to get to the current element in a set of chains.
	// These are known as materialised paths in the context of tree data structures.
	// Having this information here allows us to efficiently find out if
	// there is a relationship between two elements at any depth in the chain.
	Paths []string
	// Tags provide a way to categorise a reference chain node.
	// For example, "link" could be used to indicate that the reference chain node
	// that represents a link derived from link selectors in the blueprint.
	Tags []string
}

// RefChainCollector collects references in a set of chains (tree structures)
// and allows for checking for cycles in references once the full set of chains
// have been built for the current blueprint.
// This can also be used to for change staging/deployment stages to determine the
// order in which changes should be applied to elements in a blueprint.
type RefChainCollector interface {
	// Collect adds a new reference to the reference chain(s) to be used for cycle detection
	// and other use cases.
	Collect(elementName string, element interface{}, referencedBy string, tags []string) error
	// Chain returns the reference chain node for the given element name.
	Chain(elementName string) *ReferenceChainNode
	// FindCircularReferences returns a list of reference chain nodes for which there are
	// cycles.
	FindCircularReferences() []*ReferenceChainNode
}

type refChainCollectorImpl struct {
	chains []*ReferenceChainNode
	refMap map[string]*ReferenceChainNode
}

// NewRefChainCollector creates a new instance of the default
// reference chain collector implementation.
func NewRefChainCollector() RefChainCollector {
	return &refChainCollectorImpl{
		chains: []*ReferenceChainNode{},
		refMap: map[string]*ReferenceChainNode{},
	}
}

func (s *refChainCollectorImpl) Collect(elementName string, element interface{}, referencedBy string, tags []string) error {
	chain, addedToExistingParent, err := s.createOrUpdateChain(elementName, element, referencedBy, tags)
	if err != nil {
		return err
	}

	if !addedToExistingParent {
		s.chains = append(s.chains, chain)
	}
	s.refMap[elementName] = chain

	return nil
}

func (s *refChainCollectorImpl) FindCircularReferences() []*ReferenceChainNode {
	circularRefs := []*ReferenceChainNode{}
	s.cleanupChains()
	findCycles(s.chains, &circularRefs)
	return circularRefs
}

func (s *refChainCollectorImpl) Chain(elementName string) *ReferenceChainNode {
	return s.refMap[elementName]
}

func (s *refChainCollectorImpl) cleanupChains() {
	newChains := []*ReferenceChainNode{}
	for _, chain := range s.chains {
		if chain.Element == nil {
			// Remove placeholder chains that were added for elements that were
			// referenced but not defined in the spec.
			delete(s.refMap, chain.ElementName)
		} else {
			newChains = append(newChains, chain)
		}
	}
	s.chains = newChains
}

func (s *refChainCollectorImpl) createOrUpdateChain(
	elementName string,
	element interface{},
	referencedBy string,
	tags []string,
) (*ReferenceChainNode, bool, error) {

	elementChain, elementChainExists := s.refMap[elementName]
	if !elementChainExists {
		elementChain = &ReferenceChainNode{
			ElementName: elementName,
			Element:     element,
			Tags:        tags,
		}
	}

	var parent *ReferenceChainNode
	addedToExistingParent := false
	if existingParent, parentExists := s.refMap[referencedBy]; referencedBy != "" && parentExists {
		parent = existingParent
		addedToExistingParent = true
	} else if referencedBy != "" && !parentExists {
		// Add a placeholder for the parent, parents with nil elements will be cleaned up
		// as a part of the cycle detection process when FindCircularReferences is called.
		parent = &ReferenceChainNode{
			ElementName: referencedBy,
		}
		s.chains = append(s.chains, parent)
		s.refMap[referencedBy] = parent
	}

	if parent != nil {
		elementChain.ReferencedBy = append(elementChain.ReferencedBy, parent)
		addParentPaths(elementChain, parent)
		if len(elementChain.References) > 0 {
			// Update the elements referenced by the current element to include the updated
			// parent paths.
			updatePathsForReferencedElements(elementChain)
		}
		parent.References = append(parent.References, elementChain)
	}

	elementChain.Tags = append(elementChain.Tags, tags...)

	return elementChain, addedToExistingParent, nil
}

func updatePathsForReferencedElements(elementChain *ReferenceChainNode) {
	for _, reference := range elementChain.References {
		addParentPaths(reference, elementChain)
	}
}

func addParentPaths(elementChain *ReferenceChainNode, parent *ReferenceChainNode) {
	for _, path := range parent.Paths {
		elementChain.Paths = append(elementChain.Paths, fmt.Sprintf("%s/%s", path, elementChain.ElementName))
	}
	if len(parent.Paths) == 0 {
		elementChain.Paths = append(elementChain.Paths, fmt.Sprintf("%s/%s", parent.ElementName, elementChain.ElementName))
	}
}

func findCycles(chains []*ReferenceChainNode, chainsWithCycle *[]*ReferenceChainNode) {
	for _, chain := range chains {
		// As soon as a cycle is found in a chain, we'll capture the node
		// and move on to the next independent chain.
		// This prevents repeat work where the same cycle will be found in the materialised
		// path for descendant nodes in the tree.
		if hasCyclicPath(chain) {
			if !hasChain(*chainsWithCycle, chain) {
				*chainsWithCycle = append(*chainsWithCycle, chain)
			}
		} else {
			findCycles(chain.References, chainsWithCycle)
		}
	}
}

func hasCyclicPath(chain *ReferenceChainNode) bool {
	foundPathWithCycle := false
	i := 0
	for !foundPathWithCycle && i < len(chain.Paths) {
		path := chain.Paths[i]
		parts := strings.Split(path, "/")
		foundPathWithCycle = appearsMultipleTimes(parts, chain.ElementName)
		i += 1
	}
	return foundPathWithCycle
}

func hasChain(chains []*ReferenceChainNode, chain *ReferenceChainNode) bool {
	hasChain := false
	i := 0
	for !hasChain && i < len(chains) {
		hasChain = chains[i].ElementName == chain.ElementName
		i += 1
	}
	return hasChain
}

func appearsMultipleTimes(parts []string, elementName string) bool {
	appearances := 0
	i := 0
	for appearances <= 1 && i < len(parts) {
		if parts[i] == elementName {
			appearances += 1
		}
		i += 1
	}
	return appearances > 1
}
