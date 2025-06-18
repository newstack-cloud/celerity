package pluginutils

import (
	"context"

	"github.com/newstack-cloud/celerity/libs/blueprint/core"
	"github.com/newstack-cloud/celerity/libs/blueprint/provider"
)

// ServiceFactory is a function type that creates an instance of a service
// that will usually be a client for the provider service API.
type ServiceFactory[ServiceConfig any, Service any] func(
	serviceConfig ServiceConfig,
	providerContext provider.Context,
) Service

// ServiceConfigStore is an interface that defines a method to retrieve
// service-specific configuration for a service factory to create an instance of the service.
// The `ServiceConfig` type parameter should be the config required by client libraries
// for the service provider, such as `*aws.Config` for AWS services.
//
// It is a good practise to use a store that caches service configuration
// for a session to reuse the same configuration between calls to the same plugin
// that are a part of the same deployment process/session.
// Provider-specific configuration will almost always be derived from the provider
// context, so implementing config stores with the `FromProviderContext` method
// is a good approach that will also allow you to use the resource test tools
// that are provided in the `plugintestutils` package.
type ServiceConfigStore[ServiceConfig any] interface {
	// FromProviderContext derives service-specific configuration
	// from the provider context for the current request to the provider plugin.
	FromProviderContext(
		ctx context.Context,
		providerContext provider.Context,
		// A map of additional metadata that can contain values specific
		// to the current request that can be used to configure the service.
		meta map[string]*core.MappingNode,
	) (ServiceConfig, error)
}
