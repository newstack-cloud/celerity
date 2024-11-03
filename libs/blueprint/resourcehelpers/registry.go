package resourcehelpers

import (
	"context"
	"sync"

	"github.com/two-hundred/celerity/libs/blueprint/core"
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

	// GetTypeDescription returns the description of a resource type
	// in the registry.
	GetTypeDescription(
		ctx context.Context,
		resourceType string,
		input *provider.ResourceGetTypeDescriptionInput,
	) (*provider.ResourceGetTypeDescriptionOutput, error)

	// HasResourceType checks if a resource type is available in the registry.
	HasResourceType(ctx context.Context, resourceType string) (bool, error)

	// ListResourceTypes returns a list of all resource types available in the registry.
	ListResourceTypes(ctx context.Context) ([]string, error)

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
	resourceCache         *core.Cache[provider.Resource]
	abstractResourceCache *core.Cache[transform.AbstractResource]
	resourceTypes         []string
	mu                    sync.Mutex
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
		resourceCache:         core.NewCache[provider.Resource](),
		abstractResourceCache: core.NewCache[transform.AbstractResource](),
		resourceTypes:         []string{},
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

func (r *registryFromProviders) GetTypeDescription(
	ctx context.Context,
	resourceType string,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	resourceImpl, err := r.getResourceType(ctx, resourceType)
	if err != nil {
		abstractResourceImpl, abstractErr := r.getAbstractResourceType(ctx, resourceType)
		if abstractErr != nil {
			return nil, errMultipleRunErrors([]error{err, abstractErr})
		}

		output, err := abstractResourceImpl.GetTypeDescription(
			ctx,
			&transform.AbstractResourceGetTypeDescriptionInput{
				Params: input.Params,
			},
		)
		if err != nil {
			return nil, err
		}

		return &provider.ResourceGetTypeDescriptionOutput{
			MarkdownDescription:  output.MarkdownDescription,
			PlainTextDescription: output.PlainTextDescription,
		}, nil
	}

	return resourceImpl.GetTypeDescription(ctx, input)
}

func (r *registryFromProviders) ListResourceTypes(ctx context.Context) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.resourceTypes) > 0 {
		return r.resourceTypes, nil
	}

	resourceTypes := []string{}
	for _, provider := range r.providers {
		types, err := provider.ListResourceTypes(ctx)
		if err != nil {
			return nil, err
		}

		resourceTypes = append(resourceTypes, types...)
	}

	for _, transformer := range r.transformers {
		abstractResourceTypes, err := transformer.ListAbstractResourceTypes(ctx)
		if err != nil {
			return nil, err
		}

		resourceTypes = append(resourceTypes, abstractResourceTypes...)
	}

	r.resourceTypes = resourceTypes

	return resourceTypes, nil
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
	resource, cached := r.resourceCache.Get(resourceType)
	if cached {
		return resource, nil
	}

	providerNamespace := provider.ExtractProviderFromItemType(resourceType)
	provider, ok := r.providers[providerNamespace]
	if !ok {
		return nil, errResourceTypeProviderNotFound(providerNamespace, resourceType)
	}
	resourceImpl, err := provider.Resource(ctx, resourceType)
	if err != nil || resourceImpl == nil {
		return nil, errProviderResourceTypeNotFound(resourceType, providerNamespace)
	}
	r.resourceCache.Set(resourceType, resourceImpl)

	return resourceImpl, nil
}

func (r *registryFromProviders) getAbstractResourceType(ctx context.Context, resourceType string) (transform.AbstractResource, error) {
	resource, cached := r.abstractResourceCache.Get(resourceType)
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

	r.abstractResourceCache.Set(resourceType, abstractResource)

	return abstractResource, nil
}
