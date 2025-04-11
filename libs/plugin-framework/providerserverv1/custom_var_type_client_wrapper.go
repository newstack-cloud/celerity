package providerserverv1

import (
	context "context"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/plugin-framework/convertv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
)

type customVarTypeProviderClientWrapper struct {
	client             ProviderClient
	customVariableType string
	hostID             string
}

func (v *customVarTypeProviderClientWrapper) GetType(
	ctx context.Context,
	input *provider.CustomVariableTypeGetTypeInput,
) (*provider.CustomVariableTypeGetTypeOutput, error) {
	providerCtx, err := convertv1.ToPBProviderContext(input.ProviderContext)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionProviderGetCustomVariableType,
		)
	}

	response, err := v.client.GetCustomVariableType(
		ctx,
		&CustomVariableTypeRequest{
			CustomVariableType: &CustomVariableType{
				Type: v.customVariableType,
			},
			HostId:  v.hostID,
			Context: providerCtx,
		},
	)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionProviderGetCustomVariableType,
		)
	}

	switch result := response.Response.(type) {
	case *CustomVariableTypeResponse_CustomVarTypeInfo:
		return &provider.CustomVariableTypeGetTypeOutput{
			Type:  result.CustomVarTypeInfo.Type.Type,
			Label: result.CustomVarTypeInfo.Label,
		}, nil
	case *CustomVariableTypeResponse_ErrorResponse:
		return nil, errorsv1.CreateErrorFromResponse(
			result.ErrorResponse,
			errorsv1.PluginActionProviderGetCustomVariableType,
		)
	}

	return nil, errorsv1.CreateGeneralError(
		errorsv1.ErrUnexpectedResponseType(
			errorsv1.PluginActionProviderGetCustomVariableType,
		),
		errorsv1.PluginActionProviderGetCustomVariableType,
	)
}

func (v *customVarTypeProviderClientWrapper) GetDescription(
	ctx context.Context,
	input *provider.CustomVariableTypeGetDescriptionInput,
) (*provider.CustomVariableTypeGetDescriptionOutput, error) {
	return nil, nil
}

func (v *customVarTypeProviderClientWrapper) Options(
	ctx context.Context,
	input *provider.CustomVariableTypeOptionsInput,
) (*provider.CustomVariableTypeOptionsOutput, error) {
	return nil, nil
}

func (v *customVarTypeProviderClientWrapper) GetExamples(
	ctx context.Context,
	input *provider.CustomVariableTypeGetExamplesInput,
) (*provider.CustomVariableTypeGetExamplesOutput, error) {
	return nil, nil
}
