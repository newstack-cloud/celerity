package providerserverv1

import (
	context "context"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/plugin-framework/convertv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
	sharedtypesv1 "github.com/two-hundred/celerity/libs/plugin-framework/sharedtypesv1"
)

type functionProviderClientWrapper struct {
	client       ProviderClient
	functionName string
	hostID       string
}

func (f *functionProviderClientWrapper) GetDefinition(
	ctx context.Context,
	input *provider.FunctionGetDefinitionInput,
) (*provider.FunctionGetDefinitionOutput, error) {
	definitionReq, err := convertv1.ToPBFunctionDefinitionRequest(
		f.functionName,
		input,
		f.hostID,
	)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionProviderGetFunctionDefinition,
		)
	}

	response, err := f.client.GetFunctionDefinition(
		ctx,
		definitionReq,
	)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionProviderGetFunctionDefinition,
		)
	}

	switch result := response.Response.(type) {
	case *sharedtypesv1.FunctionDefinitionResponse_FunctionDefinition:
		return convertv1.FromPBFunctionDefinitionResponse(
			result.FunctionDefinition,
			errorsv1.PluginActionProviderGetFunctionDefinition,
		)
	case *sharedtypesv1.FunctionDefinitionResponse_ErrorResponse:
		return nil, errorsv1.CreateErrorFromResponse(
			result.ErrorResponse,
			errorsv1.PluginActionProviderGetFunctionDefinition,
		)
	}

	return nil, errorsv1.CreateGeneralError(
		errorsv1.ErrUnexpectedResponseType(
			errorsv1.PluginActionProviderGetFunctionDefinition,
		),
		errorsv1.PluginActionProviderGetFunctionDefinition,
	)
}

func (f *functionProviderClientWrapper) Call(
	ctx context.Context,
	input *provider.FunctionCallInput,
) (*provider.FunctionCallOutput, error) {
	return nil, nil
}
