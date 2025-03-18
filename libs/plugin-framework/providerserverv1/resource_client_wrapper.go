package providerserverv1

import (
	context "context"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/serialisation"
	"github.com/two-hundred/celerity/libs/plugin-framework/convertv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
	sharedtypesv1 "github.com/two-hundred/celerity/libs/plugin-framework/sharedtypesv1"
)

type resourceProviderClientWrapper struct {
	client       ProviderClient
	resourceType string
	hostID       string
}

func (r *resourceProviderClientWrapper) CustomValidate(
	ctx context.Context,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	schemaResourcePB, err := serialisation.ToResourcePB(input.SchemaResource)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionProviderCustomValidateResource,
		)
	}

	providerCtx, err := convertv1.ToPBProviderContext(input.ProviderContext)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionProviderCustomValidateResource,
		)
	}

	response, err := r.client.CustomValidateResource(
		ctx,
		&CustomValidateResourceRequest{
			ResourceType: &sharedtypesv1.ResourceType{
				Type: r.resourceType,
			},
			HostId:         r.hostID,
			SchemaResource: schemaResourcePB,
			Context:        providerCtx,
		},
	)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionProviderCustomValidateResource,
		)
	}

	switch result := response.Response.(type) {
	case *CustomValidateResourceResponse_CompleteResponse:
		return &provider.ResourceValidateOutput{
			Diagnostics: sharedtypesv1.ToCoreDiagnostics(result.CompleteResponse.GetDiagnostics()),
		}, nil
	case *CustomValidateResourceResponse_ErrorResponse:
		return nil, errorsv1.CreateErrorFromResponse(
			result.ErrorResponse,
			errorsv1.PluginActionProviderCustomValidateResource,
		)
	}

	return nil, errorsv1.CreateGeneralError(
		errorsv1.ErrUnexpectedResponseType(errorsv1.PluginActionProviderCustomValidateResource),
		errorsv1.PluginActionProviderCustomValidateResource,
	)
}

func (r *resourceProviderClientWrapper) GetSpecDefinition(
	ctx context.Context,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	return nil, nil
}

func (r *resourceProviderClientWrapper) CanLinkTo(
	ctx context.Context,
	input *provider.ResourceCanLinkToInput,
) (*provider.ResourceCanLinkToOutput, error) {
	return nil, nil
}

func (r *resourceProviderClientWrapper) GetStabilisedDependencies(
	ctx context.Context,
	input *provider.ResourceStabilisedDependenciesInput,
) (*provider.ResourceStabilisedDependenciesOutput, error) {
	return nil, nil
}

func (r *resourceProviderClientWrapper) IsCommonTerminal(
	ctx context.Context,
	input *provider.ResourceIsCommonTerminalInput,
) (*provider.ResourceIsCommonTerminalOutput, error) {
	return nil, nil
}

func (r *resourceProviderClientWrapper) GetType(
	ctx context.Context,
	input *provider.ResourceGetTypeInput,
) (*provider.ResourceGetTypeOutput, error) {
	return &provider.ResourceGetTypeOutput{
		Type: r.resourceType,
	}, nil
}

func (r *resourceProviderClientWrapper) GetTypeDescription(
	ctx context.Context,
	input *provider.ResourceGetTypeDescriptionInput,
) (*provider.ResourceGetTypeDescriptionOutput, error) {
	return nil, nil
}

func (r *resourceProviderClientWrapper) GetExamples(
	ctx context.Context,
	input *provider.ResourceGetExamplesInput,
) (*provider.ResourceGetExamplesOutput, error) {
	return nil, nil
}

func (r *resourceProviderClientWrapper) Deploy(
	ctx context.Context,
	input *provider.ResourceDeployInput,
) (*provider.ResourceDeployOutput, error) {
	return nil, nil
}

func (r *resourceProviderClientWrapper) HasStabilised(
	ctx context.Context,
	input *provider.ResourceHasStabilisedInput,
) (*provider.ResourceHasStabilisedOutput, error) {
	return nil, nil
}

func (r *resourceProviderClientWrapper) GetExternalState(
	ctx context.Context,
	input *provider.ResourceGetExternalStateInput,
) (*provider.ResourceGetExternalStateOutput, error) {
	return nil, nil
}

func (r *resourceProviderClientWrapper) Destroy(
	ctx context.Context,
	input *provider.ResourceDestroyInput,
) error {
	return nil
}
