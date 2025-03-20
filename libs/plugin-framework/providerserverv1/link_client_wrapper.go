package providerserverv1

import (
	context "context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/plugin-framework/convertv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
)

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
	return nil, nil
}

func (l *linkProviderClientWrapper) UpdateResourceB(
	ctx context.Context,
	input *provider.LinkUpdateResourceInput,
) (*provider.LinkUpdateResourceOutput, error) {
	return nil, nil
}

func (l *linkProviderClientWrapper) UpdateIntermediaryResources(
	ctx context.Context,
	input *provider.LinkUpdateIntermediaryResourcesInput,
) (*provider.LinkUpdateIntermediaryResourcesOutput, error) {
	return nil, nil
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
