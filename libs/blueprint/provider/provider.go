package provider

import "context"

// Provider is the interface for an implementation of a provider
// of a set of resource and data source types that can be used in a blueprint.
// An example of a provider could be a cloud provider such as AWS
// or Google Cloud.
//
// When we have links between resources of different providers, a new provider
// implementation should be implemented to act as a bridge between the two providers
// the linked resources belong to.
type Provider interface {
	// Namespace retrieves the resource type prefix that is used as the namespace
	// for all resource types in the provider.
	// (e.g. "aws" for AWS resources such as "aws/lambda/function")
	Namespace(ctx context.Context) (string, error)
	// Resource retrieves a resource plugin to handle a resource in a blueprint for
	// a given resource type.
	Resource(ctx context.Context, resourceType string) (Resource, error)
	// DataSource retrieves a data source plugin to handle a data source in a blueprint
	// for a given data source type.
	DataSource(ctx context.Context, dataSourceType string) (DataSource, error)
	// Link retrieves a link plugin to handle a link between two resource types
	// in a blueprint.
	Link(ctx context.Context, resourceTypeA string, resourceTypeB string) (Link, error)
	// CustomVariableType retrieves a custom variable type plugin to handle validating
	// convenience variable types with a (usually large) fixed set of possible values.
	// These custom variable types should not be used for dynamically sourced values
	// external to a blueprint, data sources exist for that purpose.
	CustomVariableType(ctx context.Context, customVariableType string) (CustomVariableType, error)
	// ListFunctions retrieves a list of all the function names that are provided by the
	// provider. This is primarily used to assign the correct provider to a function
	// as functions are globally named. When multiple providers provide the same function,
	// an error should be reported during initialisation.
	ListFunctions(ctx context.Context) ([]string, error)
	// Function retrieves a function plugin that provides custom pure functions for blueprint
	// substitutions "${..}".
	// Functions are global and which providers are to be used for which functions
	// should be configured during initialisation of an application using the framework.
	// The core functions that are defined in the blueprint specification can not be overridden
	// by a provider.
	Function(ctx context.Context, functionName string) (Function, error)
}
