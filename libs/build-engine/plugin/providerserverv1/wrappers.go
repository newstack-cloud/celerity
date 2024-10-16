package providerserverv1

import (
	context "context"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// WrapProviderClient wraps a provider plugin v1 ProviderClient
// in a blueprint framework Provider to allow the build engine
// to interact with the provider in a way that is compatible
// with the blueprint framework and unaware that the provider
// is backed by a gRPC server plugin.
func WrapProviderClient(client ProviderClient) provider.Provider {
	return &providerClientWrapper{
		client: client,
	}
}

type providerClientWrapper struct {
	client ProviderClient
}

func (p *providerClientWrapper) Namespace(ctx context.Context) (string, error) {
	namespace, err := p.client.GetNamespace(context.Background(), &emptypb.Empty{})
	if err != nil {
		return "", err
	}

	return namespace.Namespace, nil
}

func (p *providerClientWrapper) Resource(ctx context.Context, resourceType string) (provider.Resource, error) {
	return nil, nil
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
