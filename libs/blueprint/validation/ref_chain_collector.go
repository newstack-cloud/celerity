package validation

import (
	"fmt"
	"strings"
)

// ReferenceChain provides a node in a chain of references that contains the name
// of the current element in the chain.
type ReferenceChain struct {
	// ElementName is the unique name in the spec for the current
	// element in the chain.
	ElementName string
	// Element holds the information about the element at the blueprint spec schema-level.
	Element interface{}
	// References holds the references that the current element makes to other elements
	// in the blueprint.
	References []*ReferenceChain
	// ReferencedBy holds the references that other elements make to the current element.
	ReferencedBy []*ReferenceChain
	// Paths holds all the different "routes" to get to the current element in a set of chains.
	// These are known as materialised paths in the context of tree data structures.
	// Having this information here allows us to efficiently find out if
	// there is a relationship between two elements at any depth in the chain.
	Paths []string
}

// RefChainCollector collects references in a set of chains (tree structures)
// and allows for checking for cycles in references once the full set of chains
// have been built for the current blueprint.
// This can also be used to for change staging/deployment stages to determine the
// order in which changes should be applied to elements in a blueprint.
type RefChainCollector interface {
	// Collect adds a new reference to the reference chain(s) to be used for cycle detection
	// and other use cases.
	Collect(elementName string, element interface{}, referencedBy string) error
	// FindCircularReferences returns a list of reference chain nodes for which there are
	// cycles.
	FindCircularReferences() []*ReferenceChain
}

type refChainCollectorImpl struct {
	chains []*ReferenceChain
	refMap map[string]*ReferenceChain
}

// NewRefChainCollector creates a new instance of the default
// reference chain collector implementation.
func NewRefChainCollector() RefChainCollector {
	return &refChainCollectorImpl{
		chains: []*ReferenceChain{},
		refMap: map[string]*ReferenceChain{},
	}
}

func (s *refChainCollectorImpl) Collect(elementName string, element interface{}, referencedBy string) error {
	chain, err := s.createOrUpdateChain(elementName, element, referencedBy)
	if err != nil {
		return err
	}

	s.chains = append(s.chains, chain)
	s.refMap[elementName] = chain

	return nil
}

func (s *refChainCollectorImpl) FindCircularReferences() []*ReferenceChain {
	circularRefs := []*ReferenceChain{}
	findCycles(s.chains, &circularRefs)
	return circularRefs
}

func (s *refChainCollectorImpl) createOrUpdateChain(elementName string, element interface{}, referencedBy string) (*ReferenceChain, error) {

	elementChain, elementChainExists := s.refMap[elementName]
	if !elementChainExists {
		elementChain = &ReferenceChain{
			ElementName: elementName,
			Element:     element,
		}
	}

	if parent, parentExists := s.refMap[referencedBy]; referencedBy != "" && parentExists {
		elementChain.ReferencedBy = append(elementChain.ReferencedBy, parent)
		addParentPaths(elementChain, parent)
		parent.References = append(parent.References, elementChain)
	} else if referencedBy != "" && !parentExists {
		return nil, fmt.Errorf("referenced by element %q does not exist", referencedBy)
	}

	return elementChain, nil
}

func addParentPaths(elementChain *ReferenceChain, parent *ReferenceChain) {
	for _, path := range parent.Paths {
		elementChain.Paths = append(elementChain.Paths, fmt.Sprintf("%s/%s", path, elementChain.ElementName))
	}
	if len(parent.Paths) == 0 {
		elementChain.Paths = append(elementChain.Paths, fmt.Sprintf("%s/%s", parent.ElementName, elementChain.ElementName))
	}
}

func findCycles(chains []*ReferenceChain, chainsWithCycle *[]*ReferenceChain) {
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

func hasCyclicPath(chain *ReferenceChain) bool {
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

func hasChain(chains []*ReferenceChain, chain *ReferenceChain) bool {
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
