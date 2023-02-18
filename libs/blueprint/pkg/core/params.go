package core

// BlueprintParams represents a service that
// contains a mixture config for a resource provider,
// general context variables and variables for the blueprint.
type BlueprintParams interface {
	// ProviderConfig retrieves the config for the provider
	// with the given namespace in the form a concrete struct.
	// It's up to the caller to validate the provider config at runtime.
	ProviderConfig(namespace string) map[string]*ScalarValue
	// ContextVariable retrieves a context-wide variable
	// for, this differs from values extracted from context.Context
	// as these context variables are specific to the components
	// that implement the interfaces of the blueprint library.
	ContextVariable(name string) *ScalarValue
	// BlueprintVariable retrieves a variable value
	// specific to a blueprint that will ultimately substitute
	// variable placeholders in a blueprint.
	BlueprintVariable(name string) *ScalarValue
}
