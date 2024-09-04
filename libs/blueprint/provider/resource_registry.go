package provider

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/errors"
)

// ResourceRegistry provides a way to retrieve resource plugins
// across multiple providers for tasks such as resource spec validation.
type ResourceRegistry interface {
	// GetSpecDefinition returns the definition of a resource spec
	// in the registry that includes allowed parameters and return types.
	GetSpecDefinition(
		ctx context.Context,
		resourceType string,
		input *ResourceGetSpecDefinitionInput,
	) (*ResourceGetSpecDefinitionOutput, error)
	// HasResourceType checks if a resource type is available in the registry.
	HasResourceType(ctx context.Context, resourceType string) (bool, error)
}

type resourceRegistryFromProviders struct {
	providers     map[string]Provider
	resourceCache map[string]Resource
}

// NewResourceRegistry creates a new ResourceRegistry from a map of providers,
// matching against providers based on the resource type prefix.
func NewResourceRegistry(providers map[string]Provider) ResourceRegistry {
	return &resourceRegistryFromProviders{
		providers:     providers,
		resourceCache: map[string]Resource{},
	}
}

func (r *resourceRegistryFromProviders) GetSpecDefinition(
	ctx context.Context,
	resourceType string,
	input *ResourceGetSpecDefinitionInput,
) (*ResourceGetSpecDefinitionOutput, error) {
	resourceImpl, err := r.getResourceType(ctx, resourceType)
	if err != nil {
		return nil, err
	}

	return resourceImpl.GetSpecDefinition(ctx, input)
}

func (r *resourceRegistryFromProviders) HasResourceType(ctx context.Context, resourceType string) (bool, error) {
	resourceImpl, err := r.getResourceType(ctx, resourceType)
	if err != nil {
		if loadErr, isLoadErr := err.(*errors.LoadError); isLoadErr {
			if loadErr.ReasonCode == ErrorReasonCodeProviderResourceTypeNotFound {
				return false, nil
			}
		}
		return false, err
	}
	return resourceImpl != nil, nil
}

func (r *resourceRegistryFromProviders) getResourceType(ctx context.Context, resourceType string) (Resource, error) {
	resource, cached := r.resourceCache[resourceType]
	if cached {
		return resource, nil
	}

	providerNamespace := ExtractProviderFromResourceType(resourceType)
	provider, ok := r.providers[providerNamespace]
	if !ok {
		return nil, errResourceTypeProviderNotFound(providerNamespace, resourceType)
	}
	resourceImpl, err := provider.Resource(ctx, resourceType)
	if err != nil {
		return nil, errProviderResourceTypeNotFound(resourceType, providerNamespace)
	}
	r.resourceCache[resourceType] = resourceImpl

	return resourceImpl, nil
}
