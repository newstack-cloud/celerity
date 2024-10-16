package providerv1

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/build-engine/plugin/providerserverv1"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ProviderPluginDefinition is a template to be used when creating provider plugins.
// It provides a structure that allows you to define the resources, data sources,
// links and custom variable types that are supported by the provider plugin.
// This doesn't have to be used but is a useful way to define the plugin's capabilities,
// there are multiple convenience functions to create new plugins.
type ProviderPluginDefinition struct {
	Namespace           string
	Resources           map[string]provider.Resource
	DataSources         map[string]provider.DataSource
	Links               map[string]provider.Link
	CustomVariableTypes map[string]provider.CustomVariableType
}

// NewProviderPlugin creates a new instance of a provider plugin
// from a blueprint framework provider.Provider implementation.
// This produces a gRPC server plugin that the build engine host
// can use to interact with the provider.
func NewProviderPlugin(bpProvider provider.Provider) providerserverv1.ProviderServer {
	return &blueprintProviderPluginImpl{
		bpProvider: bpProvider,
	}
}

type blueprintProviderPluginImpl struct {
	providerserverv1.UnimplementedProviderServer
	bpProvider provider.Provider
}

func (p *blueprintProviderPluginImpl) GetNamespace(ctx context.Context, _ *emptypb.Empty) (*providerserverv1.Namespace, error) {
	namespace, err := p.bpProvider.Namespace(ctx)
	if err != nil {
		return nil, err
	}

	return &providerserverv1.Namespace{
		Namespace: namespace,
	}, nil
}

func (p *blueprintProviderPluginImpl) ValidateResource(ctx context.Context, req *providerserverv1.ValidateResourceRequest) (*providerserverv1.ValidateResourceResponse, error) {
	return nil, nil
}

func (p *blueprintProviderPluginImpl) CanLinkTo(ctx context.Context, req *providerserverv1.ResourceType) (*providerserverv1.CanLinkToResponse, error) {
	return nil, nil
}

// NewProviderPluginFromDefinition creates a new instance of a provider plugin
// from a ProviderPluginDefinition.
// This produces a gRPC server plugin that the build engine host
// can use to interact with the provider.
func NewProviderPluginFromDefinition(definition ProviderPluginDefinition) providerserverv1.ProviderServer {
	return &providerPluginFromDefinitionImpl{
		definition: definition,
	}
}

type providerPluginFromDefinitionImpl struct {
	providerserverv1.UnimplementedProviderServer
	definition ProviderPluginDefinition
}

func (p *providerPluginFromDefinitionImpl) GetNamespace(context.Context, *emptypb.Empty) (*providerserverv1.Namespace, error) {
	return &providerserverv1.Namespace{
		Namespace: p.definition.Namespace,
	}, nil
}

func (p *providerPluginFromDefinitionImpl) ValidateResource(
	ctx context.Context,
	req *providerserverv1.ValidateResourceRequest,
) (*providerserverv1.ValidateResourceResponse, error) {
	return nil, nil
}

func (p *providerPluginFromDefinitionImpl) CanLinkTo(
	ctx context.Context,
	req *providerserverv1.ResourceType,
) (*providerserverv1.CanLinkToResponse, error) {
	return nil, nil
}
