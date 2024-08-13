package provider

// Provider is the interface for an implementation of a provider
// of a set of resource and data source types that can be used in a blueprint.
// An example of a provider could be a cloud provider such as AWS
// or Google Cloud.
//
// When we have links between resources of different providers, a new provider
// implementation should be implemented to act as a bridge between the two providers
// the linked resources belong to.
type Provider interface {
	// Resource retrieves a resource plugin to handle a resource in a blueprint for
	// a given resource type.
	Resource(resourceType string) Resource
	// DataSource retrieves a data source plugin to handle a data source in a blueprint
	// for a given data source type.
	DataSource(dataSourceType string) DataSource
	// Link retrieves a link plugin to handle a link between two resource types
	// in a blueprint.
	Link(resourceTypeA string, resourceTypeB string) Link
	// CustomVariableType retrieves a custom variable type plugin to handle validating
	// convenience variable types with a (usually large) fixed set of possible values.
	// These custom variable types should not be used for dynamically sourced values
	// external to a blueprint, data sources exist for that purpose.
	CustomVariableType(customVariableType string) CustomVariableType
}
