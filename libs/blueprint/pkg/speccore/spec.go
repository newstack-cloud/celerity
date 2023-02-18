package speccore

import "github.com/two-hundred/celerity/libs/blueprint/pkg/schema"

// BlueprintSpec provides an interface for a service that holds
// a parsed blueprint schema and concrete resource specs.
// Concrete resource specs are concrete structs that hold a strongly
// typed representation of all the "spec" mapping parameters
// that the resource provider supports.
// This interface is provided to decouple containers and loaders
// to make every component of the blueprint mechanism composable.
type BlueprintSpec interface {
	// ResourceConcreteSpec retrieves the concrecte type of
	// the "spec" mapping for the given resource.
	ResourceConcreteSpec(resourceName string) interface{}
	// ResourceSchema provides a convenient way to get the
	// schema for a resource without having to first get
	// the blueprint spec.
	ResourceSchema(resourceName string) *schema.Resource
	// Schema retrieves the schema for a loaded
	// blueprint.
	Schema() *schema.Blueprint
}

// ResourceSchemaSpec holds an unmarshalled resource schema
// and the concrete spec for the resource.
type ResourceSchemaSpec[ResourceSpecType any] struct {
	Schema *schema.Resource
	Spec   ResourceSpecType
}
