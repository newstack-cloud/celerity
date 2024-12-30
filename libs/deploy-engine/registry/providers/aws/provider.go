package aws

import (
	"context"
	"fmt"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/deploy-engine/plugin/sdk/providerv1"
)

type providerImpl struct {
	definition providerv1.ProviderPluginDefinition
}

// NewProvider creates a new instance of the AWS provider
// that contains the supported resources, links and custom variable types
// used when a Celerity application is deployed to AWS.
func NewProvider() provider.Provider {
	return &providerImpl{
		definition: providerv1.ProviderPluginDefinition{
			Namespace: "aws",
			Resources: map[string]provider.Resource{
				// "aws/lambda/function": &LambdaFunction{
				// 	resourceTypeSchema: map[string]*schema.Schema{},
				// },
			},
			Links:               make(map[string]provider.Link),
			CustomVariableTypes: make(map[string]provider.CustomVariableType),
		},
	}
}

func (p *providerImpl) Namespace(ctx context.Context) (string, error) {
	return p.definition.Namespace, nil
}

func (p *providerImpl) Resource(ctx context.Context, resourceType string) (provider.Resource, error) {
	resource, hasResource := p.definition.Resources[resourceType]
	if !hasResource {
		// todo: wrap in helper errors
		return nil, fmt.Errorf("resource type %s not found", resourceType)
	}
	return resource, nil
}

func (p *providerImpl) DataSource(ctx context.Context, dataSourceType string) (provider.DataSource, error) {
	dataSource, hasDataSource := p.definition.DataSources[dataSourceType]
	if !hasDataSource {
		// todo: wrap in helper errors
		return nil, fmt.Errorf("resource type %s not found", dataSourceType)
	}
	return dataSource, nil
}

func (p *providerImpl) Link(ctx context.Context, resourceTypeA string, resourceTypeB string) (provider.Link, error) {
	link, hasLink := p.definition.Links[fmt.Sprintf("%s::%s", resourceTypeA, resourceTypeB)]
	if !hasLink {
		// todo: wrap in helper errors
		return nil, fmt.Errorf("link between %s and %s not found", resourceTypeA, resourceTypeB)
	}
	return link, nil
}

func (p *providerImpl) CustomVariableType(ctx context.Context, customVariableType string) (provider.CustomVariableType, error) {
	varType, hasVarType := p.definition.CustomVariableTypes[customVariableType]
	if !hasVarType {
		// todo: wrap in helper errors
		return nil, fmt.Errorf("custom variable type %s not found", customVariableType)
	}
	return varType, nil
}

func (p *providerImpl) ListResourceTypes(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (p *providerImpl) ListDataSourceTypes(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (p *providerImpl) ListCustomVariableTypes(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (p *providerImpl) ListFunctions(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (p *providerImpl) Function(ctx context.Context, functionName string) (provider.Function, error) {
	return nil, nil
}

func (p *providerImpl) RetryPolicy(ctx context.Context) (*provider.RetryPolicy, error) {
	return nil, nil
}
