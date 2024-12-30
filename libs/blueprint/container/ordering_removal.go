package container

import (
	"slices"

	"github.com/two-hundred/celerity/libs/blueprint/state"
)

// OrderElementsForRemoval orders resources, children and links for removal based on dependencies
// in the current state of a blueprint instance.
//
// For the following dependency tree for a set of elements to be removed:
// (dependencies are the child nodes)
//
// ├── ResourceA1
// │	 ├── ResourceA2
// │	 │   ├── ChildA1
// │	 │   └── ResourceA5
// │	 └── ResourceA3
// │	 	 └── ResourceA6
// ├── LinkA1(ResourceA1, ResourceA2)
// │    ├── ResourceA1
// │    └── ResourceA2
//
// A valid order for removal would be:
// 1. LinkA1
// 2. ResourceA1
// 3. ResourceA2
// 4. ResourceA3
// 5. ResourceA5
// 6. ResourceA6
// 7. ChildA1
func OrderElementsForRemoval(
	elements *CollectedElements,
	currentState *state.InstanceState,
) []*ElementWithAllDeps {
	combinedList := []state.Element{}
	for _, resourceInfo := range elements.Resources {
		combinedList = append(combinedList, resourceInfo)
	}

	for _, childInfo := range elements.Children {
		combinedList = append(combinedList, childInfo)
	}

	for _, linkInfo := range elements.Links {
		combinedList = append(combinedList, linkInfo)
	}

	combinedListWithDeps := collectElementsWithDependencies(combinedList, currentState)

	// Instead of trying to use an efficient sort algorithm,
	// we have to compare each element with all other elements to determine
	// order based on dependencies,
	// This is because there are elements that are not connected to each other
	// and an efficient sort algorithm will not be able to correctly determine the order
	// if the two elements to compare are not connected.
	sortElementsByDependencies(combinedListWithDeps)

	return combinedListWithDeps
}

func sortElementsByDependencies(
	elements []*ElementWithAllDeps,
) {
	for i, elemA := range elements {
		for j, elemB := range elements {
			if i != j {
				if (hasDependency(elemA, elemB) && i > j) ||
					(hasDependency(elemB, elemA) && i < j) {
					// If a has a dependency on b and a is after b, swap.
					// If b has a dependency on a and b is after a, swap.
					elements[i], elements[j] = elements[j], elements[i]
				}
			}
		}
	}
}

func hasDependency(
	a, b *ElementWithAllDeps,
) bool {
	hasDep := slices.ContainsFunc(a.AllDependencies, func(current state.Element) bool {
		return current.ID() == b.Element.ID()
	})
	return hasDep
}

func collectElementsWithDependencies(
	elementList []state.Element,
	currentState *state.InstanceState,
) []*ElementWithAllDeps {
	elementsWithDeps := make([]*ElementWithAllDeps, len(elementList))
	for i, element := range elementList {
		elementsWithDeps[i] = &ElementWithAllDeps{
			Element:         element,
			AllDependencies: []state.Element{},
		}
	}

	for _, elementWithDeps := range elementsWithDeps {
		elementWithDeps.AllDependencies = collectElementDependencies(
			elementWithDeps.Element,
			elementsWithDeps,
			currentState,
		)
	}

	return elementsWithDeps
}

func collectElementDependencies(
	element state.Element,
	elementsWithDeps []*ElementWithAllDeps,
	currentState *state.InstanceState,
) []state.Element {
	dependencies := []state.Element{}

	if element.Kind() == state.ResourceElement {
		resourceState := getResourceStateByName(currentState, element.LogicalName())
		if resourceState != nil {
			childDependencies := collectElementChildDependenciesForResource(
				resourceState,
				elementsWithDeps,
				currentState,
			)
			dependencies = append(dependencies, childDependencies...)

			resourceDependencies := collectElementResourceDependenciesForResource(
				resourceState,
				elementsWithDeps,
				currentState,
			)
			dependencies = append(dependencies, resourceDependencies...)
		}
	}

	if element.Kind() == state.ChildElement {
		childDependencyInfo := getChildDependencies(currentState, element.LogicalName())
		if childDependencyInfo != nil {
			childDependencies := collectElementChildDependenciesForChild(
				childDependencyInfo,
				elementsWithDeps,
				currentState,
			)
			dependencies = append(dependencies, childDependencies...)

			resourceDependencies := collectElementResourceDependenciesForChild(
				childDependencyInfo,
				elementsWithDeps,
				currentState,
			)
			dependencies = append(dependencies, resourceDependencies...)
		}
	}

	if element.Kind() == state.LinkElement {
		linkDependencyInfo := extractLinkDirectDependencies(element.LogicalName())
		if linkDependencyInfo != nil {
			resourceDependencies := collectElementResourceDependenciesForLink(
				linkDependencyInfo,
				elementsWithDeps,
				currentState,
			)
			dependencies = append(dependencies, resourceDependencies...)
		}
	}

	return dependencies
}

func collectElementChildDependenciesForResource(
	resourceState *state.ResourceState,
	elementsWithDeps []*ElementWithAllDeps,
	currentState *state.InstanceState,
) []state.Element {
	return collectElementDependenciesOfType(
		resourceState.DependsOnChildren,
		elementsWithDeps,
		currentState,
		state.ChildElement,
	)
}

func collectElementResourceDependenciesForResource(
	resourceState *state.ResourceState,
	elementsWithDeps []*ElementWithAllDeps,
	currentState *state.InstanceState,
) []state.Element {
	return collectElementDependenciesOfType(
		resourceState.DependsOnResources,
		elementsWithDeps,
		currentState,
		state.ResourceElement,
	)
}

func collectElementResourceDependenciesForChild(
	childDependencyInfo *state.ChildDependencyInfo,
	elementsWithDeps []*ElementWithAllDeps,
	currentState *state.InstanceState,
) []state.Element {
	return collectElementDependenciesOfType(
		childDependencyInfo.DependsOnResources,
		elementsWithDeps,
		currentState,
		state.ChildElement,
	)
}

func collectElementChildDependenciesForChild(
	childDependencyInfo *state.ChildDependencyInfo,
	elementsWithDeps []*ElementWithAllDeps,
	currentState *state.InstanceState,
) []state.Element {
	return collectElementDependenciesOfType(
		childDependencyInfo.DependsOnChildren,
		elementsWithDeps,
		currentState,
		state.ChildElement,
	)
}

func collectElementResourceDependenciesForLink(
	linkDependencyInfo *linkDependencyInfo,
	elementsWithDeps []*ElementWithAllDeps,
	currentState *state.InstanceState,
) []state.Element {
	return collectElementDependenciesOfType(
		[]string{
			linkDependencyInfo.resourceAName,
			linkDependencyInfo.resourceBName,
		},
		elementsWithDeps,
		currentState,
		state.ResourceElement,
	)
}

func collectElementDependenciesOfType(
	dependenciesOfType []string,
	elementsWithDeps []*ElementWithAllDeps,
	currentState *state.InstanceState,
	elementKind state.ElementKind,
) []state.Element {
	dependencies := []state.Element{}

	for _, dependency := range dependenciesOfType {
		elementWithDeps := findElement(elementsWithDeps, dependency, elementKind)
		if elementWithDeps != nil {
			if len(elementWithDeps.AllDependencies) == 0 {
				elementDeps := collectElementDependencies(
					elementWithDeps.Element,
					elementsWithDeps,
					currentState,
				)
				elementWithDeps.AllDependencies = elementDeps
			}

			dependencies = append(dependencies, elementWithDeps.Element)
			// Ensure all transitive dependencies are collected in order for the sorting
			// to work correctly.
			dependencies = addUniqueElements(dependencies, elementWithDeps.AllDependencies)
		}

	}

	return dependencies
}

func findElement(
	elementsWithDeps []*ElementWithAllDeps,
	id string,
	elementKind state.ElementKind,
) *ElementWithAllDeps {
	i := 0
	element := (*ElementWithAllDeps)(nil)
	for element == nil && i < len(elementsWithDeps) {
		elementWithDeps := elementsWithDeps[i]
		if elementWithDeps.Element.ID() == id &&
			elementWithDeps.Element.Kind() == elementKind {
			element = elementWithDeps
		}
		i += 1
	}

	return element
}

func addUniqueElements(
	elements []state.Element,
	elementsToAdd []state.Element,
) []state.Element {
	for _, element := range elementsToAdd {
		if !containsElement(elements, element) {
			elements = append(elements, element)
		}
	}

	return elements
}

func containsElement(
	elements []state.Element,
	element state.Element,
) bool {
	return slices.ContainsFunc(elements, func(current state.Element) bool {
		return current.LogicalName() == element.LogicalName() &&
			current.Kind() == element.Kind()
	})
}

// ElementsWithAllDeps stores a representation of an element in a blueprint
// with all of its dependencies, both direct and transitive.
// This is primarily used for collecting and ordering elements for removal.
type ElementWithAllDeps struct {
	Element state.Element
	// All collected dependencies for an element, both direct and transitive.
	AllDependencies []state.Element
}

type linkDependencyInfo struct {
	resourceAName string
	resourceBName string
}
