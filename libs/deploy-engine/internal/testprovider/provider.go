package testprovider

import (
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/deploy-engine/plugin/sdk/providerv1"
)

// NewProvider creates a new instance of the test AWS provider
// that contains the supported resources, links, custom variable types
// and functions for the stub AWS provider.
// This is purely for testing purposes and does not interact with AWS services
// or provide functionality that would reflect that of a real AWS provider
// implementation.
func NewProvider() provider.Provider {
	return &providerv1.ProviderPluginDefinition{
		ProviderNamespace: "aws",
		Resources:         map[string]provider.Resource{
			// "aws/lambda/function": &LambdaFunction{
			// 	resourceTypeSchema: map[string]*schema.Schema{},
			// },
		},
		Links:               make(map[string]provider.Link),
		CustomVariableTypes: make(map[string]provider.CustomVariableType),
	}
}
