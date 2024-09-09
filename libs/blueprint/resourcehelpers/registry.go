package resourcehelpers

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/errors"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/transform"
)

// Registry provides a way to retrieve resource plugins
// across multiple providers and transformers for tasks such as resource spec validation.
type Registry interface {
	// GetSpecDefinition returns the definition of a resource spec
	// in the registry.
	GetSpecDefinition(
		ctx context.Context,
		resourceType string,
		input *provider.ResourceGetSpecDefinitionInput,
	) (*provider.ResourceGetSpecDefinitionOutput, error)
	// GetStateDefinition returns the definition of a resource's output state
	// in the registry.
	GetStateDefinition(
		ctx context.Context,
		resourceType string,
		input *provider.ResourceGetStateDefinitionInput,
	) (*provider.ResourceGetStateDefinitionOutput, error)
	// HasResourceType checks if a resource type is available in the registry.
	HasResourceType(ctx context.Context, resourceType string) (bool, error)
	// CustomValidate allows for custom validation of a resource of a given type.
	CustomValidate(
		ctx context.Context,
		resourceType string,
		input *provider.ResourceValidateInput,
	) (*provider.ResourceValidateOutput, error)
}

type registryFromProviders struct {
	providers             map[string]provider.Provider
	transformers          map[string]transform.SpecTransformer
	resourceCache         map[string]provider.Resource
	abstractResourceCache map[string]transform.AbstractResource
}

// NewRegistry creates a new resource registry from a map of providers,
// matching against providers based on the resource type prefix.
func NewRegistry(
	providers map[string]provider.Provider,
	transformers map[string]transform.SpecTransformer,
) Registry {
	return &registryFromProviders{
		providers:             providers,
		transformers:          transformers,
		resourceCache:         map[string]provider.Resource{},
		abstractResourceCache: map[string]transform.AbstractResource{},
	}
}

func (r *registryFromProviders) GetSpecDefinition(
	ctx context.Context,
	resourceType string,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	resourceImpl, err := r.getResourceType(ctx, resourceType)
	if err != nil {
		abstractResourceImpl, abstractErr := r.getAbstractResourceType(ctx, resourceType)
		if abstractErr != nil {
			return nil, errMultipleRunErrors([]error{err, abstractErr})
		}

		output, err := abstractResourceImpl.GetSpecDefinition(
			ctx,
			&transform.AbstractResourceGetSpecDefinitionInput{
				Params: input.Params,
			},
		)
		if err != nil {
			return nil, err
		}

		return &provider.ResourceGetSpecDefinitionOutput{
			SpecDefinition: output.SpecDefinition,
		}, nil
	}

	return resourceImpl.GetSpecDefinition(ctx, input)
}

func (r *registryFromProviders) GetStateDefinition(
	ctx context.Context,
	resourceType string,
	input *provider.ResourceGetStateDefinitionInput,
) (*provider.ResourceGetStateDefinitionOutput, error) {
	resourceImpl, err := r.getResourceType(ctx, resourceType)
	if err != nil {
		abstractResourceImpl, abstractErr := r.getAbstractResourceType(ctx, resourceType)
		if abstractErr != nil {
			return nil, errMultipleRunErrors([]error{err, abstractErr})
		}

		output, err := abstractResourceImpl.GetStateDefinition(
			ctx,
			&transform.AbstractResourceGetStateDefinitionInput{
				Params: input.Params,
			},
		)
		if err != nil {
			return nil, err
		}

		return &provider.ResourceGetStateDefinitionOutput{
			StateDefinition: output.StateDefinition,
		}, nil
	}

	return resourceImpl.GetStateDefinition(ctx, input)
}

func (r *registryFromProviders) HasResourceType(ctx context.Context, resourceType string) (bool, error) {
	hasResourceType, err := r.hasProviderResourceType(ctx, resourceType)
	if err != nil {
		return false, err
	}

	hasAbstractResourceType, err := r.hasAbstractResourceType(ctx, resourceType)
	if err != nil {
		return false, err
	}

	return hasResourceType || hasAbstractResourceType, nil
}

func (r *registryFromProviders) hasProviderResourceType(ctx context.Context, resourceType string) (bool, error) {
	resourceImpl, err := r.getResourceType(ctx, resourceType)
	if err != nil {
		if runErr, isRunErr := err.(*errors.RunError); isRunErr {
			if runErr.ReasonCode == ErrorReasonCodeProviderResourceTypeNotFound {
				return false, nil
			}
		}
		return false, err
	}
	return resourceImpl != nil, nil
}

func (r *registryFromProviders) hasAbstractResourceType(ctx context.Context, resourceType string) (bool, error) {
	abstractResourceImpl, err := r.getAbstractResourceType(ctx, resourceType)
	if err != nil {
		if runErr, isRunErr := err.(*errors.RunError); isRunErr {
			if runErr.ReasonCode == ErrorReasonCodeAbstractResourceTypeNotFound {
				return false, nil
			}
		}
		return false, err
	}
	return abstractResourceImpl != nil, nil
}

func (r *registryFromProviders) CustomValidate(
	ctx context.Context,
	resourceType string,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	resourceImpl, err := r.getResourceType(ctx, resourceType)
	if err != nil {
		abstractResourceImpl, abstractErr := r.getAbstractResourceType(ctx, resourceType)
		if abstractErr != nil {
			return nil, errMultipleRunErrors([]error{err, abstractErr})
		}

		output, err := abstractResourceImpl.CustomValidate(ctx, &transform.AbstractResourceValidateInput{
			SchemaResource: input.SchemaResource,
			Params:         input.Params,
		})
		if err != nil {
			return nil, err
		}
		return &provider.ResourceValidateOutput{
			Diagnostics: output.Diagnostics,
		}, nil
	}

	return resourceImpl.CustomValidate(ctx, input)
}

func (r *registryFromProviders) getResourceType(ctx context.Context, resourceType string) (provider.Resource, error) {
	resource, cached := r.resourceCache[resourceType]
	if cached {
		return resource, nil
	}

	providerNamespace := provider.ExtractProviderFromItemType(resourceType)
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

func (r *registryFromProviders) getAbstractResourceType(ctx context.Context, resourceType string) (transform.AbstractResource, error) {
	resource, cached := r.abstractResourceCache[resourceType]
	if cached {
		return resource, nil
	}

	var abstractResource transform.AbstractResource
	// Transformers do not have namespaces that correspond to resource type prefixes
	// so we need to iterate through all transformers to find the correct one.
	// This shouldn't be a problem as in practice, a small number of transformers
	// will be used at a time.
	for _, transformer := range r.transformers {
		var err error
		abstractResource, err = transformer.AbstractResource(ctx, resourceType)
		if err == nil && abstractResource != nil {
			break
		}
	}

	if abstractResource == nil {
		return nil, errAbstactResourceTypeNotFound(resourceType)
	}

	r.abstractResourceCache[resourceType] = abstractResource

	return abstractResource, nil
}
