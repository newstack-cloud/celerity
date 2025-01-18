package core

// BlueprintParams represents a service that
// contains a mixture config for a resource provider,
// general context variables and variables for the blueprint.
type BlueprintParams interface {
	// ProviderConfig retrieves the config for the provider
	// with the given namespace in the form a map of key-value pairs.
	// It's up to the caller to validate the provider config at runtime.
	ProviderConfig(namespace string) map[string]*ScalarValue
	// TransformerConfig retrieves the config for the transformer
	// with the given namespace in the form a map of key-value pairs.
	// It's up to the caller to validate the transformer config at runtime.
	TransformerConfig(namespace string) map[string]*ScalarValue
	// ContextVariable retrieves a context-wide variable
	// for the current environment, this differs from values extracted
	// from context.Context as these context variables are specific to the components
	// that implement the interfaces of the blueprint library.
	ContextVariable(name string) *ScalarValue
	// BlueprintVariable retrieves a variable value
	// specific to a blueprint that will ultimately substitute
	// variable placeholders in a blueprint.
	BlueprintVariable(name string) *ScalarValue
	// WithBlueprintVariables returns a new BlueprintParams
	// with the given variables added to the blueprint variables.
	// If keepExisting is true, the existing blueprint variables
	// will be kept, otherwise they will not be included in the new
	// BlueprintParams.
	//
	// This is useful for creating a new set of parameters for child
	// blueprints that need to inherit context variables and provider config
	// from the parent but require their own set of blueprint variables as
	// defined in an "include" block of the parent blueprint.
	WithBlueprintVariables(vars map[string]*ScalarValue, keepExisting bool) BlueprintParams
	// WithContextVariables returns a new BlueprintParams
	// with the given variables added to the context variables.
	// If keepExisting is true, the existing context variables
	// will be kept, otherwise they will not be included in the new
	// BlueprintParams.
	WithContextVariables(vars map[string]*ScalarValue, keepExisting bool) BlueprintParams
}

// ParamsImpl provides an implementation of the blueprint
// core.BlueprintParams interface to supply parameters when
// loading blueprint source files.
type ParamsImpl struct {
	ProviderConf       map[string]map[string]*ScalarValue
	TransformerConf    map[string]map[string]*ScalarValue
	ContextVariables   map[string]*ScalarValue
	BlueprintVariables map[string]*ScalarValue
}

// NewParams creates a new Params instance with
// the supplied provider configuration, context variables
// and blueprint variables.
func NewDefaultParams(
	providerConfig map[string]map[string]*ScalarValue,
	transformerConfig map[string]map[string]*ScalarValue,
	contextVariables map[string]*ScalarValue,
	blueprintVariables map[string]*ScalarValue,
) BlueprintParams {
	return &ParamsImpl{
		ProviderConf:       providerConfig,
		TransformerConf:    transformerConfig,
		ContextVariables:   contextVariables,
		BlueprintVariables: blueprintVariables,
	}
}

func (p *ParamsImpl) ProviderConfig(namespace string) map[string]*ScalarValue {
	return p.ProviderConf[namespace]
}

func (p *ParamsImpl) TransformerConfig(namespace string) map[string]*ScalarValue {
	return p.TransformerConf[namespace]
}

func (p *ParamsImpl) ContextVariable(name string) *ScalarValue {
	return p.ContextVariables[name]
}

func (p *ParamsImpl) BlueprintVariable(name string) *ScalarValue {
	return p.BlueprintVariables[name]
}

func (b *ParamsImpl) WithBlueprintVariables(
	vars map[string]*ScalarValue,
	keepExisting bool,
) BlueprintParams {
	newBlueprintVariables := map[string]*ScalarValue{}
	if keepExisting {
		for k, v := range b.BlueprintVariables {
			newBlueprintVariables[k] = v
		}
	}

	for k, v := range vars {
		newBlueprintVariables[k] = v
	}

	return &ParamsImpl{
		ProviderConf:       b.ProviderConf,
		ContextVariables:   b.ContextVariables,
		BlueprintVariables: newBlueprintVariables,
	}
}

func (b *ParamsImpl) WithContextVariables(
	vars map[string]*ScalarValue,
	keepExisting bool,
) BlueprintParams {
	newContextVariables := map[string]*ScalarValue{}
	if keepExisting {
		for k, v := range b.ContextVariables {
			newContextVariables[k] = v
		}
	}

	for k, v := range vars {
		newContextVariables[k] = v
	}

	return &ParamsImpl{
		ProviderConf:       b.ProviderConf,
		ContextVariables:   newContextVariables,
		BlueprintVariables: b.BlueprintVariables,
	}
}
