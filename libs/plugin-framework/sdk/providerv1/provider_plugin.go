package providerv1

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/plugin-framework/convertv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/providerserverv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/sharedtypesv1"
	"google.golang.org/protobuf/types/known/emptypb"
)

// NewProviderPlugin creates a new instance of a provider plugin
// from a blueprint framework provider.Provider implementation.
// This produces a gRPC server plugin that the deploy engine host
// can interact with.
// The `ProviderPluginDefinition` utility type can be passed in to
// create a provider plugin server as it implements the `provider.Provider`
// interface.
func NewProviderPlugin(bpProvider provider.Provider) providerserverv1.ProviderServer {
	return &blueprintProviderPluginImpl{
		bpProvider: bpProvider,
	}
}

type blueprintProviderPluginImpl struct {
	providerserverv1.UnimplementedProviderServer
	bpProvider provider.Provider
}

func (p *blueprintProviderPluginImpl) GetNamespace(
	ctx context.Context,
	_ *emptypb.Empty,
) (*providerserverv1.NamespaceResponse, error) {
	namespace, err := p.bpProvider.Namespace(ctx)
	if err != nil {
		return toProviderNamespaceErrorResponse(err), nil
	}

	return toProviderNamespaceResponse(namespace), nil
}

func (p *blueprintProviderPluginImpl) ListResourceTypes(
	ctx context.Context,
	_ *emptypb.Empty,
) (*providerserverv1.ResourceTypesResponse, error) {
	resourceTypes, err := p.bpProvider.ListResourceTypes(ctx)
	if err != nil {
		return toResourceTypesErrorResponse(err), nil
	}

	return toResourceTypesResponse(resourceTypes), nil
}

func (p *blueprintProviderPluginImpl) ListDataSourceTypes(
	ctx context.Context,
	_ *emptypb.Empty,
) (*providerserverv1.DataSourceTypesResponse, error) {
	dataSourceTypes, err := p.bpProvider.ListDataSourceTypes(ctx)
	if err != nil {
		return toDataSourceTypesErrorResponse(err), nil
	}

	return toDataSourceTypesResponse(dataSourceTypes), nil
}

func (p *blueprintProviderPluginImpl) ListCustomVariableTypes(
	ctx context.Context,
	_ *emptypb.Empty,
) (*providerserverv1.CustomVariableTypesResponse, error) {
	customVariableTypes, err := p.bpProvider.ListCustomVariableTypes(ctx)
	if err != nil {
		return toCustomVariableTypesErrorResponse(err), nil
	}

	return toCustomVariableTypesResponse(customVariableTypes), nil
}

func (p *blueprintProviderPluginImpl) ListFunctions(
	ctx context.Context,
	_ *emptypb.Empty,
) (*providerserverv1.FunctionListResponse, error) {
	functions, err := p.bpProvider.ListFunctions(ctx)
	if err != nil {
		return toFunctionsErrorResponse(err), nil
	}

	return toFunctionsResponse(functions), nil
}

func (p *blueprintProviderPluginImpl) GetRetryPolicy(
	ctx context.Context,
	_ *emptypb.Empty,
) (*providerserverv1.RetryPolicyResponse, error) {
	policy, err := p.bpProvider.RetryPolicy(ctx)
	if err != nil {
		return toRetryPolicyErrorResponse(err), nil
	}

	return toRetryPolicyResponse(policy), nil
}

func (p *blueprintProviderPluginImpl) CustomValidateResource(
	ctx context.Context,
	req *providerserverv1.CustomValidateResourceRequest,
) (*providerserverv1.CustomValidateResourceResponse, error) {
	resource, err := p.bpProvider.Resource(
		ctx,
		convertv1.ResourceTypeToString(req.ResourceType),
	)
	if err != nil {
		return toCustomValidateErrorResponse(err), nil
	}

	validationInput, err := fromPBCustomValidateResourceRequest(req)
	if err != nil {
		return toCustomValidateErrorResponse(err), nil
	}

	output, err := resource.CustomValidate(
		ctx,
		validationInput,
	)
	if err != nil {
		return toCustomValidateErrorResponse(err), nil
	}

	return toCustomValidateResourceResponse(output), nil
}

func (p *blueprintProviderPluginImpl) GetResourceSpecDefinition(
	ctx context.Context,
	req *providerserverv1.ResourceRequest,
) (*providerserverv1.ResourceSpecDefinitionResponse, error) {
	resource, err := p.bpProvider.Resource(
		ctx,
		convertv1.ResourceTypeToString(req.ResourceType),
	)
	if err != nil {
		return toResourceSpecDefinitionErrorResponse(err), nil
	}

	providerCtx, err := convertv1.FromPBProviderContext(req.Context)
	if err != nil {
		return toResourceSpecDefinitionErrorResponse(err), nil
	}

	output, err := resource.GetSpecDefinition(
		ctx,
		&provider.ResourceGetSpecDefinitionInput{
			ProviderContext: providerCtx,
		},
	)
	if err != nil {
		return toResourceSpecDefinitionErrorResponse(err), nil
	}

	response, err := toPBResourceSpecDefinitionResponse(output)
	if err != nil {
		return toResourceSpecDefinitionErrorResponse(err), nil
	}

	return response, nil
}

func (p *blueprintProviderPluginImpl) CanResourceLinkTo(
	ctx context.Context,
	req *providerserverv1.ResourceRequest,
) (*providerserverv1.CanResourceLinkToResponse, error) {
	resource, err := p.bpProvider.Resource(
		ctx,
		convertv1.ResourceTypeToString(req.ResourceType),
	)
	if err != nil {
		return toCanResourceLinkToErrorResponse(err), nil
	}

	providerCtx, err := convertv1.FromPBProviderContext(req.Context)
	if err != nil {
		return toCanResourceLinkToErrorResponse(err), nil
	}

	output, err := resource.CanLinkTo(
		ctx,
		&provider.ResourceCanLinkToInput{
			ProviderContext: providerCtx,
		},
	)
	if err != nil {
		return toCanResourceLinkToErrorResponse(err), nil
	}

	return toCanResourceLinkToResponse(output), nil
}

func (p *blueprintProviderPluginImpl) GetResourceStabilisedDeps(
	ctx context.Context,
	req *providerserverv1.ResourceRequest,
) (*providerserverv1.ResourceStabilisedDepsResponse, error) {
	resource, err := p.bpProvider.Resource(
		ctx,
		convertv1.ResourceTypeToString(req.ResourceType),
	)
	if err != nil {
		return toResourceStabilisedDepsErrorResponse(err), nil
	}

	providerCtx, err := convertv1.FromPBProviderContext(req.Context)
	if err != nil {
		return toResourceStabilisedDepsErrorResponse(err), nil
	}

	output, err := resource.GetStabilisedDependencies(
		ctx,
		&provider.ResourceStabilisedDependenciesInput{
			ProviderContext: providerCtx,
		},
	)
	if err != nil {
		return toResourceStabilisedDepsErrorResponse(err), nil
	}

	return toResourceStabilisedDepsResponse(output), nil
}

func (p *blueprintProviderPluginImpl) IsResourceCommonTerminal(
	ctx context.Context,
	req *providerserverv1.ResourceRequest,
) (*providerserverv1.IsResourceCommonTerminalResponse, error) {
	resource, err := p.bpProvider.Resource(
		ctx,
		convertv1.ResourceTypeToString(req.ResourceType),
	)
	if err != nil {
		return toIsResourceCommonTerminalErrorResponse(err), nil
	}

	providerCtx, err := convertv1.FromPBProviderContext(req.Context)
	if err != nil {
		return toIsResourceCommonTerminalErrorResponse(err), nil
	}

	output, err := resource.IsCommonTerminal(
		ctx,
		&provider.ResourceIsCommonTerminalInput{
			ProviderContext: providerCtx,
		},
	)
	if err != nil {
		return toIsResourceCommonTerminalErrorResponse(err), nil
	}

	return toIsResourceCommonTerminalResponse(output), nil
}

func (p *blueprintProviderPluginImpl) GetResourceTypeDescription(
	ctx context.Context,
	req *providerserverv1.ResourceRequest,
) (*sharedtypesv1.TypeDescriptionResponse, error) {
	resource, err := p.bpProvider.Resource(
		ctx,
		convertv1.ResourceTypeToString(req.ResourceType),
	)
	if err != nil {
		return toResourceTypeDescriptionErrorResponse(err), nil
	}

	providerCtx, err := convertv1.FromPBProviderContext(req.Context)
	if err != nil {
		return toResourceTypeDescriptionErrorResponse(err), nil
	}

	output, err := resource.GetTypeDescription(
		ctx,
		&provider.ResourceGetTypeDescriptionInput{
			ProviderContext: providerCtx,
		},
	)
	if err != nil {
		return toResourceTypeDescriptionErrorResponse(err), nil
	}

	return toResourceTypeDescriptionResponse(output), nil
}

func (p *blueprintProviderPluginImpl) DeployResource(
	ctx context.Context,
	req *sharedtypesv1.DeployResourceRequest,
) (*sharedtypesv1.DeployResourceResponse, error) {
	resource, err := p.bpProvider.Resource(
		ctx,
		convertv1.ResourceTypeToString(req.ResourceType),
	)
	if err != nil {
		return convertv1.ToPBDeployResourceErrorResponse(err), nil
	}

	resourceDeployInput, err := convertv1.FromPBDeployResourceRequest(req)
	if err != nil {
		return convertv1.ToPBDeployResourceErrorResponse(err), nil
	}

	output, err := resource.Deploy(
		ctx,
		resourceDeployInput,
	)
	if err != nil {
		return convertv1.ToPBDeployResourceErrorResponse(err), nil
	}

	response, err := convertv1.ToPBDeployResourceResponse(output)
	if err != nil {
		return convertv1.ToPBDeployResourceErrorResponse(err), nil
	}

	return response, nil
}

func (p *blueprintProviderPluginImpl) ResourceHasStabilised(
	ctx context.Context,
	req *sharedtypesv1.ResourceHasStabilisedRequest,
) (*sharedtypesv1.ResourceHasStabilisedResponse, error) {
	resource, err := p.bpProvider.Resource(
		ctx,
		convertv1.ResourceTypeToString(req.ResourceType),
	)
	if err != nil {
		return convertv1.ToPBResourceHasStabilisedErrorResponse(err), nil
	}

	resourceHasStabilisedInput, err := convertv1.FromPBResourceHasStabilisedRequest(req)
	if err != nil {
		return convertv1.ToPBResourceHasStabilisedErrorResponse(err), nil
	}

	output, err := resource.HasStabilised(
		ctx,
		resourceHasStabilisedInput,
	)
	if err != nil {
		return convertv1.ToPBResourceHasStabilisedErrorResponse(err), nil
	}

	return convertv1.ToPBResourceHasStabilisedResponse(output), nil
}

func (p *blueprintProviderPluginImpl) GetResourceExternalState(
	ctx context.Context,
	req *providerserverv1.GetResourceExternalStateRequest,
) (*providerserverv1.GetResourceExternalStateResponse, error) {
	resource, err := p.bpProvider.Resource(
		ctx,
		convertv1.ResourceTypeToString(req.ResourceType),
	)
	if err != nil {
		return toResourceExternalStateErrorResponse(err), nil
	}

	resourceExternalStateInput, err := fromPBGetResourceExternalStateRequest(req)
	if err != nil {
		return toResourceExternalStateErrorResponse(err), nil
	}

	output, err := resource.GetExternalState(
		ctx,
		resourceExternalStateInput,
	)
	if err != nil {
		return toResourceExternalStateErrorResponse(err), nil
	}

	response, err := toResourceExternalStateResponse(output)
	if err != nil {
		return toResourceExternalStateErrorResponse(err), nil
	}

	return response, nil
}

func (p *blueprintProviderPluginImpl) DestroyResource(
	ctx context.Context,
	req *sharedtypesv1.DestroyResourceRequest,
) (*sharedtypesv1.DestroyResourceResponse, error) {
	resource, err := p.bpProvider.Resource(
		ctx,
		convertv1.ResourceTypeToString(req.ResourceType),
	)
	if err != nil {
		return convertv1.ToPBDestroyResourceErrorResponse(err), nil
	}

	resourceDestroyInput, err := convertv1.FromPBDestroyResourceRequest(req)
	if err != nil {
		return convertv1.ToPBDestroyResourceErrorResponse(err), nil
	}

	err = resource.Destroy(
		ctx,
		resourceDestroyInput,
	)
	if err != nil {
		return convertv1.ToPBDestroyResourceErrorResponse(err), nil
	}

	return &sharedtypesv1.DestroyResourceResponse{
		Response: &sharedtypesv1.DestroyResourceResponse_Result{
			Result: &sharedtypesv1.DestroyResourceResult{
				Destroyed: true,
			},
		},
	}, nil
}
