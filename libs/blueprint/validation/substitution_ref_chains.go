package validation

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
type RefChainCollector struct {
	chains []*ReferenceChain
}

// NewRefChainCollector creates a new instance of the reference chain collector.
func NewRefChainCollector() *RefChainCollector {
	return &RefChainCollector{
		chains: []*ReferenceChain{},
	}
}

// Collect adds a new reference to the service chain(s) to be used for cycle detection.
func (s *RefChainCollector) Collect(referencedBy string, elementName string, element interface{}) {
}
