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
