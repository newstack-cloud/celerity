package providerserverv1

import (
	context "context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/serialisation"
	"github.com/two-hundred/celerity/libs/plugin-framework/convertv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
	"google.golang.org/grpc"
)

type linkResourceUpdateFunc func(
	ctx context.Context,
	input *UpdateLinkResourceRequest,
	opts ...grpc.CallOption,
) (*UpdateLinkResourceResponse, error)

type linkProviderClientWrapper struct {
	client        ProviderClient
	resourceTypeA string
	resourceTypeB string
	hostID        string
}

func (l *linkProviderClientWrapper) StageChanges(
	ctx context.Context,
	input *provider.LinkStageChangesInput,
) (*provider.LinkStageChangesOutput, error) {
	changesRequest, err := l.buildStageChangesRequest(input)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionProviderStageLinkChanges,
		)
	}

	response, err := l.client.StageLinkChanges(ctx, changesRequest)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionProviderStageLinkChanges,
		)
	}

	switch result := response.Response.(type) {
	case *StageLinkChangesResponse_CompleteResponse:
		linkChanges, err := convertv1.FromPBLinkChanges(result.CompleteResponse.Changes)
		if err != nil {
			return nil, errorsv1.CreateGeneralError(
				err,
				errorsv1.PluginActionProviderStageLinkChanges,
			)
		}

		return &provider.LinkStageChangesOutput{
			Changes: &linkChanges,
		}, nil
	case *StageLinkChangesResponse_ErrorResponse:
		return nil, errorsv1.CreateErrorFromResponse(
			result.ErrorResponse,
			errorsv1.PluginActionProviderStageLinkChanges,
		)
	}

	return nil, errorsv1.CreateGeneralError(
		errorsv1.ErrUnexpectedResponseType(errorsv1.PluginActionProviderStageLinkChanges),
		errorsv1.PluginActionProviderStageLinkChanges,
	)
}

func (l *linkProviderClientWrapper) buildStageChangesRequest(
	input *provider.LinkStageChangesInput,
) (*StageLinkChangesRequest, error) {
	resourceAChangesPB, err := convertv1.ToPBChanges(input.ResourceAChanges)
	if err != nil {
		return nil, err
	}

	resourceBChangesPB, err := convertv1.ToPBChanges(input.ResourceBChanges)
	if err != nil {
		return nil, err
	}

	currentLinkStatePB, err := toPBLinkState(input.CurrentLinkState)
	if err != nil {
		return nil, err
	}

	linkCtx, err := toPBLinkContext(input.LinkContext)
	if err != nil {
		return nil, err
	}

	return &StageLinkChangesRequest{
		LinkType: &LinkType{
			Type: core.LinkType(
				l.resourceTypeA,
				l.resourceTypeB,
			),
		},
		HostId:           l.hostID,
		ResourceAChanges: resourceAChangesPB,
		ResourceBChanges: resourceBChangesPB,
		CurrentLinkState: currentLinkStatePB,
		Context:          linkCtx,
	}, nil
}

func (l *linkProviderClientWrapper) UpdateResourceA(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return l.updateResource(
		ctx,
		input,
		l.client.UpdateLinkResourceA,
		errorsv1.PluginActionProviderUpdateLinkResourceA,
	)
}

func (l *linkProviderClientWrapper) UpdateResourceB(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return l.updateResource(
		ctx,
		input,
		l.client.UpdateLinkResourceB,
		errorsv1.PluginActionProviderUpdateLinkResourceB,
	)
}

func (l *linkProviderClientWrapper) updateResource(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
	updateFunc linkResourceUpdateFunc,
	action errorsv1.PluginAction,
) (*provider.LinkUpdateResourceOutput, error) {
	updateLinkResourceReq, err := l.buildUpdateResourceRequest(input)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			action,
		)
	}

	response, err := updateFunc(ctx, updateLinkResourceReq)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			action,
		)
	}

	switch result := response.Response.(type) {
	case *UpdateLinkResourceResponse_CompleteResponse:
		linkData, err := serialisation.FromMappingNodePB(
			result.CompleteResponse.LinkData,
			/* optional */ true,
		)
		if err != nil {
			return nil, errorsv1.CreateGeneralError(
				err,
				action,
			)
		}

		return &provider.LinkUpdateResourceOutput{
			LinkData: linkData,
		}, nil
	case *UpdateLinkResourceResponse_ErrorResponse:
		return nil, errorsv1.CreateErrorFromResponse(
			result.ErrorResponse,
			action,
		)
	}

	return nil, errorsv1.CreateGeneralError(
		errorsv1.ErrUnexpectedResponseType(action),
		action,
	)
}

func (l *linkProviderClientWrapper) buildUpdateResourceRequest(
	input *provider.LinkUpdateResourceInput,
) (*UpdateLinkResourceRequest, error) {
	linkChangesPB, err := toPBLinkChanges(input.Changes)
	if err != nil {
		return nil, err
	}

	linkCtx, err := toPBLinkContext(input.LinkContext)
	if err != nil {
		return nil, err
	}

	resourceInfoPB, err := convertv1.ToPBResourceInfo(input.ResourceInfo)
	if err != nil {
		return nil, err
	}

	otherResourceInfoPB, err := convertv1.ToPBResourceInfo(input.OtherResourceInfo)
	if err != nil {
		return nil, err
	}

	return &UpdateLinkResourceRequest{
		LinkType: &LinkType{
			Type: core.LinkType(
				l.resourceTypeA,
				l.resourceTypeB,
			),
		},
		HostId:            l.hostID,
		Changes:           linkChangesPB,
		ResourceInfo:      resourceInfoPB,
		OtherResourceInfo: otherResourceInfoPB,
		UpdateType:        LinkUpdateType(input.LinkUpdateType),
		Context:           linkCtx,
	}, nil
}

func (l *linkProviderClientWrapper) UpdateIntermediaryResources(
	ctx context.Context,
	input *provider.LinkUpdateIntermediaryResourcesInput,
) (*provider.LinkUpdateIntermediaryResourcesOutput, error) {
	request, err := l.buildUpdateIntermediaryResourcesRequest(input)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionProviderUpdateLinkIntermediaryResources,
		)
	}

	response, err := l.client.UpdateLinkIntermediaryResources(ctx, request)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionProviderUpdateLinkIntermediaryResources,
		)
	}

	switch result := response.Response.(type) {
	case *UpdateLinkIntermediaryResourcesResponse_CompleteResponse:
		output, err := fromPBLinkIntermediaryResourcesCompleteResponse(
			result.CompleteResponse,
		)
		if err != nil {
			return nil, errorsv1.CreateGeneralError(
				err,
				errorsv1.PluginActionProviderUpdateLinkIntermediaryResources,
			)
		}

		return output, nil
	case *UpdateLinkIntermediaryResourcesResponse_ErrorResponse:
		return nil, errorsv1.CreateErrorFromResponse(
			result.ErrorResponse,
			errorsv1.PluginActionProviderUpdateLinkIntermediaryResources,
		)
	}

	return nil, errorsv1.CreateGeneralError(
		errorsv1.ErrUnexpectedResponseType(
			errorsv1.PluginActionProviderUpdateLinkIntermediaryResources,
		),
		errorsv1.PluginActionProviderUpdateLinkIntermediaryResources,
	)
}

func (l *linkProviderClientWrapper) buildUpdateIntermediaryResourcesRequest(
	input *provider.LinkUpdateIntermediaryResourcesInput,
) (*UpdateLinkIntermediaryResourcesRequest, error) {
	linkCtx, err := toPBLinkContext(input.LinkContext)
	if err != nil {
		return nil, err
	}

	resourceAInfoPB, err := convertv1.ToPBResourceInfo(input.ResourceAInfo)
	if err != nil {
		return nil, err
	}

	resourceBInfoPB, err := convertv1.ToPBResourceInfo(input.ResourceBInfo)
	if err != nil {
		return nil, err
	}

	linkChangesPB, err := toPBLinkChanges(input.Changes)
	if err != nil {
		return nil, err
	}

	return &UpdateLinkIntermediaryResourcesRequest{
		LinkType: &LinkType{
			Type: core.LinkType(
				l.resourceTypeA,
				l.resourceTypeB,
			),
		},
		HostId:        l.hostID,
		ResourceAInfo: resourceAInfoPB,
		ResourceBInfo: resourceBInfoPB,
		Changes:       linkChangesPB,
		UpdateType:    LinkUpdateType(input.LinkUpdateType),
		Context:       linkCtx,
	}, nil
}

func (l *linkProviderClientWrapper) GetPriorityResource(
	ctx context.Context,
	input *provider.LinkGetPriorityResourceInput,
) (*provider.LinkGetPriorityResourceOutput, error) {
	return nil, nil
}

func (l *linkProviderClientWrapper) GetType(
	ctx context.Context,
	input *provider.LinkGetTypeInput,
) (*provider.LinkGetTypeOutput, error) {
	return nil, nil
}

func (l *linkProviderClientWrapper) GetTypeDescription(
	ctx context.Context,
	input *provider.LinkGetTypeDescriptionInput,
) (*provider.LinkGetTypeDescriptionOutput, error) {
	return nil, nil
}

func (l *linkProviderClientWrapper) GetAnnotationDefinitions(
	ctx context.Context,
	input *provider.LinkGetAnnotationDefinitionsInput,
) (*provider.LinkGetAnnotationDefinitionsOutput, error) {
	return nil, nil
}

func (l *linkProviderClientWrapper) GetKind(
	ctx context.Context,
	input *provider.LinkGetKindInput,
) (*provider.LinkGetKindOutput, error) {
	return nil, nil
}
