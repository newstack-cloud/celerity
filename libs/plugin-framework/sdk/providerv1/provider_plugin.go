package providerv1

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/plugin-framework/convertv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/pluginservicev1"
	"github.com/two-hundred/celerity/libs/plugin-framework/providerserverv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/sdk/pluginutils"
	"github.com/two-hundred/celerity/libs/plugin-framework/sharedtypesv1"
)

// NewProviderPlugin creates a new instance of a provider plugin
// from a blueprint framework provider.Provider implementation.
// This produces a gRPC server plugin that the deploy engine host
// can interact with.
// The `ProviderPluginDefinition` utility type can be passed in to
// create a provider plugin server as it implements the `provider.Provider`
// interface.
//
// The host info container is used to retrieve the ID of the host
// that the plugin was registered with.
//
// The service client is used to communicate with other plugins
// that are registered with the deploy engine host.
func NewProviderPlugin(
	bpProvider provider.Provider,
	hostInfoContainer pluginutils.HostInfoContainer,
	serviceClient pluginservicev1.ServiceClient,
) providerserverv1.ProviderServer {
	return &blueprintProviderPluginImpl{
		bpProvider:        bpProvider,
		hostInfoContainer: hostInfoContainer,
		serviceClient:     serviceClient,
	}
}

type blueprintProviderPluginImpl struct {
	providerserverv1.UnimplementedProviderServer
	bpProvider        provider.Provider
	hostInfoContainer pluginutils.HostInfoContainer
	serviceClient     pluginservicev1.ServiceClient
}

func (p *blueprintProviderPluginImpl) GetNamespace(
	ctx context.Context,
	req *providerserverv1.ProviderRequest,
) (*providerserverv1.NamespaceResponse, error) {
	err := p.checkHostID(req.HostId)
	if err != nil {
		return toProviderNamespaceErrorResponse(err), nil
	}

	namespace, err := p.bpProvider.Namespace(ctx)
	if err != nil {
		return toProviderNamespaceErrorResponse(err), nil
	}

	return toProviderNamespaceResponse(namespace), nil
}

func (p *blueprintProviderPluginImpl) GetConfigDefinition(
	ctx context.Context,
	req *providerserverv1.ProviderRequest,
) (*sharedtypesv1.ConfigDefinitionResponse, error) {
	err := p.checkHostID(req.HostId)
	if err != nil {
		return convertv1.ToPBConfigDefinitionErrorResponse(err), nil
	}

	configDefinition, err := p.bpProvider.ConfigDefinition(ctx)
	if err != nil {
		return convertv1.ToPBConfigDefinitionErrorResponse(err), nil
	}

	return convertv1.ToPBConfigDefinitionResponse(configDefinition)
}

func (p *blueprintProviderPluginImpl) ListResourceTypes(
	ctx context.Context,
	req *providerserverv1.ProviderRequest,
) (*providerserverv1.ResourceTypesResponse, error) {
	err := p.checkHostID(req.HostId)
	if err != nil {
		return toResourceTypesErrorResponse(err), nil
	}

	resourceTypes, err := p.bpProvider.ListResourceTypes(ctx)
	if err != nil {
		return toResourceTypesErrorResponse(err), nil
	}

	return toResourceTypesResponse(resourceTypes), nil
}

func (p *blueprintProviderPluginImpl) ListDataSourceTypes(
	ctx context.Context,
	req *providerserverv1.ProviderRequest,
) (*providerserverv1.DataSourceTypesResponse, error) {
	err := p.checkHostID(req.HostId)
	if err != nil {
		return toDataSourceTypesErrorResponse(err), nil
	}

	dataSourceTypes, err := p.bpProvider.ListDataSourceTypes(ctx)
	if err != nil {
		return toDataSourceTypesErrorResponse(err), nil
	}

	return toDataSourceTypesResponse(dataSourceTypes), nil
}

func (p *blueprintProviderPluginImpl) ListCustomVariableTypes(
	ctx context.Context,
	req *providerserverv1.ProviderRequest,
) (*providerserverv1.CustomVariableTypesResponse, error) {
	err := p.checkHostID(req.HostId)
	if err != nil {
		return toCustomVariableTypesErrorResponse(err), nil
	}

	customVariableTypes, err := p.bpProvider.ListCustomVariableTypes(ctx)
	if err != nil {
		return toCustomVariableTypesErrorResponse(err), nil
	}

	return toCustomVariableTypesResponse(customVariableTypes), nil
}

func (p *blueprintProviderPluginImpl) ListFunctions(
	ctx context.Context,
	req *providerserverv1.ProviderRequest,
) (*providerserverv1.FunctionListResponse, error) {
	err := p.checkHostID(req.HostId)
	if err != nil {
		return toFunctionsErrorResponse(err), nil
	}

	functions, err := p.bpProvider.ListFunctions(ctx)
	if err != nil {
		return toFunctionsErrorResponse(err), nil
	}

	return toFunctionsResponse(functions), nil
}

func (p *blueprintProviderPluginImpl) GetRetryPolicy(
	ctx context.Context,
	req *providerserverv1.ProviderRequest,
) (*providerserverv1.RetryPolicyResponse, error) {
	err := p.checkHostID(req.HostId)
	if err != nil {
		return toRetryPolicyErrorResponse(err), nil
	}

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
	err := p.checkHostID(req.HostId)
	if err != nil {
		return toCustomValidateErrorResponse(err), nil
	}

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
	err := p.checkHostID(req.HostId)
	if err != nil {
		return toResourceSpecDefinitionErrorResponse(err), nil
	}

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
	err := p.checkHostID(req.HostId)
	if err != nil {
		return toCanResourceLinkToErrorResponse(err), nil
	}

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
	err := p.checkHostID(req.HostId)
	if err != nil {
		return toResourceStabilisedDepsErrorResponse(err), nil
	}

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
	err := p.checkHostID(req.HostId)
	if err != nil {
		return toIsResourceCommonTerminalErrorResponse(err), nil
	}

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

func (p *blueprintProviderPluginImpl) GetResourceType(
	ctx context.Context,
	req *providerserverv1.ResourceRequest,
) (*sharedtypesv1.ResourceTypeResponse, error) {
	err := p.checkHostID(req.HostId)
	if err != nil {
		return convertv1.ToPBResourceTypeErrorResponse(err), nil
	}

	resource, err := p.bpProvider.Resource(
		ctx,
		convertv1.ResourceTypeToString(req.ResourceType),
	)
	if err != nil {
		return convertv1.ToPBResourceTypeErrorResponse(err), nil
	}

	providerCtx, err := convertv1.FromPBProviderContext(req.Context)
	if err != nil {
		return convertv1.ToPBResourceTypeErrorResponse(err), nil
	}

	output, err := resource.GetType(
		ctx,
		&provider.ResourceGetTypeInput{
			ProviderContext: providerCtx,
		},
	)
	if err != nil {
		return convertv1.ToPBResourceTypeErrorResponse(err), nil
	}

	return convertv1.ToPBResourceTypeResponse(output), nil
}

func (p *blueprintProviderPluginImpl) GetResourceTypeDescription(
	ctx context.Context,
	req *providerserverv1.ResourceRequest,
) (*sharedtypesv1.TypeDescriptionResponse, error) {
	err := p.checkHostID(req.HostId)
	if err != nil {
		return convertv1.ToPBTypeDescriptionErrorResponse(err), nil
	}

	resource, err := p.bpProvider.Resource(
		ctx,
		convertv1.ResourceTypeToString(req.ResourceType),
	)
	if err != nil {
		return convertv1.ToPBTypeDescriptionErrorResponse(err), nil
	}

	providerCtx, err := convertv1.FromPBProviderContext(req.Context)
	if err != nil {
		return convertv1.ToPBTypeDescriptionErrorResponse(err), nil
	}

	output, err := resource.GetTypeDescription(
		ctx,
		&provider.ResourceGetTypeDescriptionInput{
			ProviderContext: providerCtx,
		},
	)
	if err != nil {
		return convertv1.ToPBTypeDescriptionErrorResponse(err), nil
	}

	return toResourceTypeDescriptionResponse(output), nil
}

func (p *blueprintProviderPluginImpl) GetResourceExamples(
	ctx context.Context,
	req *providerserverv1.ResourceRequest,
) (*sharedtypesv1.ExamplesResponse, error) {
	err := p.checkHostID(req.HostId)
	if err != nil {
		return convertv1.ToPBExamplesErrorResponse(err), nil
	}

	resource, err := p.bpProvider.Resource(
		ctx,
		convertv1.ResourceTypeToString(req.ResourceType),
	)
	if err != nil {
		return convertv1.ToPBExamplesErrorResponse(err), nil
	}

	providerCtx, err := convertv1.FromPBProviderContext(req.Context)
	if err != nil {
		return convertv1.ToPBExamplesErrorResponse(err), nil
	}

	output, err := resource.GetExamples(
		ctx,
		&provider.ResourceGetExamplesInput{
			ProviderContext: providerCtx,
		},
	)
	if err != nil {
		return convertv1.ToPBExamplesErrorResponse(err), nil
	}

	return toResourceExamplesResponse(output), nil
}

func (p *blueprintProviderPluginImpl) DeployResource(
	ctx context.Context,
	req *sharedtypesv1.DeployResourceRequest,
) (*sharedtypesv1.DeployResourceResponse, error) {
	err := p.checkHostID(req.HostId)
	if err != nil {
		return convertv1.ToPBDeployResourceErrorResponse(err), nil
	}

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
	err := p.checkHostID(req.HostId)
	if err != nil {
		return convertv1.ToPBResourceHasStabilisedErrorResponse(err), nil
	}

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
	err := p.checkHostID(req.HostId)
	if err != nil {
		return toResourceExternalStateErrorResponse(err), nil
	}

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
	err := p.checkHostID(req.HostId)
	if err != nil {
		return convertv1.ToPBDestroyResourceErrorResponse(err), nil
	}

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

func (p *blueprintProviderPluginImpl) StageLinkChanges(
	ctx context.Context,
	req *providerserverv1.StageLinkChangesRequest,
) (*providerserverv1.StageLinkChangesResponse, error) {
	err := p.checkHostID(req.HostId)
	if err != nil {
		return toStageLinkChangesErrorResponse(err), nil
	}

	linkTypeInfo, err := extractLinkTypeInfo(req.LinkType)
	if err != nil {
		return toStageLinkChangesErrorResponse(err), nil
	}

	link, err := p.bpProvider.Link(
		ctx,
		linkTypeInfo.resourceTypeA,
		linkTypeInfo.resourceTypeB,
	)
	if err != nil {
		return toStageLinkChangesErrorResponse(err), nil
	}

	stageChangesInput, err := fromPBStageLinkChangesRequest(req)
	if err != nil {
		return toStageLinkChangesErrorResponse(err), nil
	}

	output, err := link.StageChanges(
		ctx,
		stageChangesInput,
	)
	if err != nil {
		return toStageLinkChangesErrorResponse(err), nil
	}

	response, err := toPBStageLinkChangesResponse(output)
	if err != nil {
		return toStageLinkChangesErrorResponse(err), nil
	}

	return response, nil
}

func (p *blueprintProviderPluginImpl) UpdateLinkResourceA(
	ctx context.Context,
	req *providerserverv1.UpdateLinkResourceRequest,
) (*providerserverv1.UpdateLinkResourceResponse, error) {
	return p.updateLinkResource(
		ctx,
		req,
		provider.LinkPriorityResourceA,
	)
}

func (p *blueprintProviderPluginImpl) UpdateLinkResourceB(
	ctx context.Context,
	req *providerserverv1.UpdateLinkResourceRequest,
) (*providerserverv1.UpdateLinkResourceResponse, error) {
	return p.updateLinkResource(
		ctx,
		req,
		provider.LinkPriorityResourceB,
	)
}

func (p *blueprintProviderPluginImpl) updateLinkResource(
	ctx context.Context,
	req *providerserverv1.UpdateLinkResourceRequest,
	linkResource provider.LinkPriorityResource,
) (*providerserverv1.UpdateLinkResourceResponse, error) {
	err := p.checkHostID(req.HostId)
	if err != nil {
		return toUpdateLinkResourceErrorResponse(err), nil
	}

	linkTypeInfo, err := extractLinkTypeInfo(req.LinkType)
	if err != nil {
		return toUpdateLinkResourceErrorResponse(err), nil
	}

	link, err := p.bpProvider.Link(
		ctx,
		linkTypeInfo.resourceTypeA,
		linkTypeInfo.resourceTypeB,
	)
	if err != nil {
		return toUpdateLinkResourceErrorResponse(err), nil
	}

	updateLinkResourceInput, err := fromPBUpdateLinkResourceRequest(req)
	if err != nil {
		return toUpdateLinkResourceErrorResponse(err), nil
	}

	updateFunc := selectLinkUpdateResourceFunc(link, linkResource)
	output, err := updateFunc(
		ctx,
		updateLinkResourceInput,
	)
	if err != nil {
		return toUpdateLinkResourceErrorResponse(err), nil
	}

	response, err := toPBUpdateLinkResourceResponse(output)
	if err != nil {
		return toUpdateLinkResourceErrorResponse(err), nil
	}

	return response, nil
}

func (p *blueprintProviderPluginImpl) UpdateLinkIntermediaryResources(
	ctx context.Context,
	req *providerserverv1.UpdateLinkIntermediaryResourcesRequest,
) (*providerserverv1.UpdateLinkIntermediaryResourcesResponse, error) {
	err := p.checkHostID(req.HostId)
	if err != nil {
		return toUpdateLinkIntermediaryResourcesErrorResponse(err), nil
	}

	linkTypeInfo, err := extractLinkTypeInfo(req.LinkType)
	if err != nil {
		return toUpdateLinkIntermediaryResourcesErrorResponse(err), nil
	}

	link, err := p.bpProvider.Link(
		ctx,
		linkTypeInfo.resourceTypeA,
		linkTypeInfo.resourceTypeB,
	)
	if err != nil {
		return toUpdateLinkIntermediaryResourcesErrorResponse(err), nil
	}

	resourceDeployService := pluginservicev1.ResourceDeployServiceFromClient(
		p.serviceClient,
	)
	updateIntermediaryResourcesInput, err := fromPBLinkIntermediaryResourceRequest(
		req,
		resourceDeployService,
	)
	if err != nil {
		return toUpdateLinkIntermediaryResourcesErrorResponse(err), nil
	}

	output, err := link.UpdateIntermediaryResources(
		ctx,
		updateIntermediaryResourcesInput,
	)
	if err != nil {
		return toUpdateLinkIntermediaryResourcesErrorResponse(err), nil
	}

	response, err := toPBUpdateLinkIntermediaryResourcesResponse(output)
	if err != nil {
		return toUpdateLinkIntermediaryResourcesErrorResponse(err), nil
	}

	return response, nil
}

func (p *blueprintProviderPluginImpl) checkHostID(hostID string) error {
	if hostID != p.hostInfoContainer.GetID() {
		return errorsv1.ErrInvalidHostID(hostID)
	}

	return nil
}
