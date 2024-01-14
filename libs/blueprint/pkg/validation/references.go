package validation

import (
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/substitutions"
	"github.com/two-hundred/celerity/libs/common/pkg/core"
)

// ValidateReference validates a reference in a blueprint,
// a reference can be to a variable, resource, child blueprint or data source.
// This validation does not validate that the reference can be resolved,
// as this validation will normally be carried out at an early stage before information
// is available about what resources, variables, data sources or child blueprints are available.
func ValidateReference(reference string, context string, hasAccessTo []Referenceable) error {
	if strings.HasPrefix(reference, "variables.") {
		return validateVariableReference(reference, context, hasAccessTo)
	}

	if strings.HasPrefix(reference, "datasources.") {
		return validateDataSourceReference(reference, context, hasAccessTo)
	}

	if strings.HasPrefix(reference, "children.") {
		return validateChildBlueprintReference(reference, context, hasAccessTo)
	}

	// Resource references are used for all other cases as they can be made
	// with or without the "resources." prefix.
	return validateResourceReference(reference, context, hasAccessTo)
}

func validateVariableReference(reference string, context string, hasAccessTo []Referenceable) error {
	if !core.SliceContainsComparable(hasAccessTo, ReferenceableVariable) {
		return errReferenceContextAccess(reference, context, ReferenceableVariable)
	}

	if !substitutions.VariableReferencePattern.MatchString(reference) {
		return errInvalidReferencePattern(reference, context, ReferenceableVariable)
	}

	return nil
}

func validateDataSourceReference(reference string, context string, hasAccessTo []Referenceable) error {
	if !core.SliceContainsComparable(hasAccessTo, ReferenceableDataSource) {
		return errReferenceContextAccess(reference, context, ReferenceableDataSource)
	}

	if !substitutions.DataSourceReferencePattern.MatchString(reference) {
		return errInvalidReferencePattern(reference, context, ReferenceableDataSource)
	}

	return nil
}

func validateChildBlueprintReference(reference string, context string, hasAccessTo []Referenceable) error {
	if !core.SliceContainsComparable(hasAccessTo, ReferenceableChild) {
		return errReferenceContextAccess(reference, context, ReferenceableChild)
	}

	if !substitutions.ChildReferencePattern.MatchString(reference) {
		return errInvalidReferencePattern(reference, context, ReferenceableChild)
	}

	return nil
}

func validateResourceReference(reference string, context string, hasAccessTo []Referenceable) error {
	if !core.SliceContainsComparable(hasAccessTo, ReferenceableResource) {
		return errReferenceContextAccess(reference, context, ReferenceableResource)
	}

	if !substitutions.ResourceReferencePattern.MatchString(reference) {
		return errInvalidReferencePattern(reference, context, ReferenceableResource)
	}

	return nil
}

// Referencable is a type that can be referenced in a blueprint.
type Referenceable string

const (
	// ReferenceableResource signifies that a resource
	// can be referenced for a given context in a blueprint.
	ReferenceableResource Referenceable = "resource"
	// ReferenceableVariable signifies that a variable
	// can be referenced for a given context in a blueprint.
	ReferenceableVariable Referenceable = "variable"
	// ReferenceableDataSource signifies that a data source
	// can be referenced for a given context in a blueprint.
	ReferenceableDataSource Referenceable = "datasource"
	// ReferenceableChild signifies that a child blueprint
	// can be referenced for a given context in a blueprint.
	ReferenceableChild Referenceable = "child"
)

func referenceableLabel(referenceable Referenceable) string {
	switch referenceable {
	case ReferenceableResource:
		return "resource"
	case ReferenceableVariable:
		return "variable"
	case ReferenceableDataSource:
		return "data source"
	case ReferenceableChild:
		return "child blueprint"
	default:
		return "unknown"
	}
}
