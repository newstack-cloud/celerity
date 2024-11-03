// Params implementation for testing purposes.

package internal

import "github.com/two-hundred/celerity/libs/blueprint/core"

// Params provides an implementation of the blueprint
// core.BlueprintParams interface to supply parameters when
// loading blueprint source files.
type Params struct {
	providerConfig     map[string]map[string]*core.ScalarValue
	contextVariables   map[string]*core.ScalarValue
	blueprintVariables map[string]*core.ScalarValue
}

// NewParams creates a new Params instance with
// the supplied provider configuration, context variables
// and blueprint variables.
func NewParams(
	providerConfig map[string]map[string]*core.ScalarValue,
	contextVariables map[string]*core.ScalarValue,
	blueprintVariables map[string]*core.ScalarValue,
) *Params {
	return &Params{
		providerConfig:     providerConfig,
		contextVariables:   contextVariables,
		blueprintVariables: blueprintVariables,
	}
}

func (p *Params) ProviderConfig(namespace string) map[string]*core.ScalarValue {
	return p.providerConfig[namespace]
}

func (p *Params) ContextVariable(name string) *core.ScalarValue {
	return p.contextVariables[name]
}

func (p *Params) BlueprintVariable(name string) *core.ScalarValue {
	return p.blueprintVariables[name]
}

func (b *Params) WithBlueprintVariables(
	vars map[string]*core.ScalarValue,
	keepExisting bool,
) core.BlueprintParams {
	newBlueprintVariables := map[string]*core.ScalarValue{}
	if keepExisting {
		for k, v := range b.blueprintVariables {
			newBlueprintVariables[k] = v
		}
	}

	for k, v := range vars {
		newBlueprintVariables[k] = v
	}

	return &Params{
		providerConfig:     b.providerConfig,
		contextVariables:   b.contextVariables,
		blueprintVariables: newBlueprintVariables,
	}
}
