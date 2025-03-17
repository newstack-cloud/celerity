package aws

import (
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/plugin-framework/sdk/providerv1"
)

// NewProvider creates a new instance of the AWS provider
// that contains the supported resources, links and custom variable types
// used when a Celerity application is deployed to AWS.
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
