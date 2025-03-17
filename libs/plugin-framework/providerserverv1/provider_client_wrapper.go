package providerserverv1

import (
	context "context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// WrapProviderClient wraps a provider plugin v1 ProviderClient
// in a blueprint framework Provider to allow the deploy engine
// to interact with the provider in a way that is compatible
// with the blueprint framework and is agnostic to the underlying
// communication protocol.
func WrapProviderClient(client ProviderClient) provider.Provider {
	return &providerClientWrapper{
		client: client,
	}
}

type providerClientWrapper struct {
	client ProviderClient
}

func (p *providerClientWrapper) Namespace(ctx context.Context) (string, error) {
	response, err := p.client.GetNamespace(context.Background(), &emptypb.Empty{})
	if err != nil {
		return "", errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionProviderGetNamespace,
		)
	}

	switch result := response.Response.(type) {
	case *NamespaceResponse_Namespace:
		return result.Namespace.GetNamespace(), nil
	case *NamespaceResponse_ErrorResponse:
		return "", errorsv1.CreateErrorFromResponse(
			result.ErrorResponse,
			errorsv1.PluginActionProviderGetNamespace,
		)
	}

	return "", errorsv1.CreateGeneralError(
		errorsv1.ErrUnexpectedResponseType(
			errorsv1.PluginActionProviderGetNamespace,
		),
		errorsv1.PluginActionProviderGetNamespace,
	)
}

func (p *providerClientWrapper) ConfigDefinition(ctx context.Context) (*core.ConfigDefinition, error) {
	return nil, nil
}

func (p *providerClientWrapper) Resource(ctx context.Context, resourceType string) (provider.Resource, error) {
	return &resourceProviderClientWrapper{
		client:       p.client,
		resourceType: resourceType,
	}, nil
}

func (p *providerClientWrapper) DataSource(ctx context.Context, dataSourceType string) (provider.DataSource, error) {
	return nil, nil
}

func (p *providerClientWrapper) Link(ctx context.Context, resourceTypeA string, resourceTypeB string) (provider.Link, error) {
	return nil, nil
}

func (p *providerClientWrapper) CustomVariableType(ctx context.Context, customVariableType string) (provider.CustomVariableType, error) {
	return nil, nil
}

func (p *providerClientWrapper) ListFunctions(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (p *providerClientWrapper) ListResourceTypes(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (p *providerClientWrapper) ListDataSourceTypes(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (p *providerClientWrapper) ListCustomVariableTypes(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (p *providerClientWrapper) Function(ctx context.Context, functionName string) (provider.Function, error) {
	return nil, nil
}

func (p *providerClientWrapper) RetryPolicy(ctx context.Context) (*provider.RetryPolicy, error) {
	return nil, nil
}
