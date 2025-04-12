package testutils

import (
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

// CreateTestProviderContext creates a provider context for testing
// with the given namespace.
func CreateTestProviderContext(namespace string) provider.Context {
	params := core.NewDefaultParams(
		map[string]map[string]*core.ScalarValue{},
		map[string]map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
	)
	return provider.NewProviderContextFromParams(namespace, params)
}

// CreateTestLinkContext creates a link context for testing.
func CreateTestLinkContext() provider.LinkContext {
	params := core.NewDefaultParams(
		map[string]map[string]*core.ScalarValue{},
		map[string]map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
	)
	return provider.NewLinkContextFromParams(params)
}

// CreateEmptyTestParams creates an empty set of parameters for testing,
// primarily used for testing plugin functions.
func CreateEmptyTestParams() core.BlueprintParams {
	return core.NewDefaultParams(
		map[string]map[string]*core.ScalarValue{},
		map[string]map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
	)
}

// CreateEmptyConcreteParams creates an empty set of concrete parameters
// for testing, primarily used for testing plugin functions.
func CreateEmptyConcreteParams() *core.ParamsImpl {
	return &core.ParamsImpl{
		ProviderConf:       map[string]map[string]*core.ScalarValue{},
		TransformerConf:    map[string]map[string]*core.ScalarValue{},
		ContextVariables:   map[string]*core.ScalarValue{},
		BlueprintVariables: map[string]*core.ScalarValue{},
	}
}
