package pluginservicev1

import (
	context "context"

	"github.com/two-hundred/celerity/libs/blueprint/function"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/plugin-framework/convertv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
	sharedtypesv1 "github.com/two-hundred/celerity/libs/plugin-framework/sharedtypesv1"
)

// ResourceDeployServiceFromClient creates a new instance of a ResourceDeployService
// that uses the provided ServiceClient to interact with the deploy engine.
// This allows plugin implementations to interact with the deploy engine
// using the blueprint framework interfaces abstracting away the communication
// protocol from plugin developers.
func ResourceDeployServiceFromClient(
	client ServiceClient,
) provider.ResourceDeployService {
	return &resourceDeployServiceClientWrapper{
		client: client,
	}
}

type resourceDeployServiceClientWrapper struct {
	client ServiceClient
}

func (r *resourceDeployServiceClientWrapper) Deploy(
	ctx context.Context,
	resourceType string,
	input *provider.ResourceDeployServiceInput,
) (*provider.ResourceDeployOutput, error) {
	deployReq, err := convertv1.ToPBDeployResourceRequest(resourceType, input.DeployInput)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionServiceDeployResource,
		)
	}

	response, err := r.client.DeployResource(
		ctx,
		&DeployResourceServiceRequest{
			DeployRequest:   deployReq,
			WaitUntilStable: input.WaitUntilStable,
		},
	)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionServiceDeployResource,
		)
	}

	switch result := response.Response.(type) {
	case *sharedtypesv1.DeployResourceResponse_CompleteResponse:
		return r.toResourceDeployOutput(result.CompleteResponse)
	case *sharedtypesv1.DeployResourceResponse_ErrorResponse:
		return nil, errorsv1.CreateErrorFromResponse(
			result.ErrorResponse,
			errorsv1.PluginActionServiceDeployResource,
		)
	}

	return nil, errorsv1.CreateGeneralError(
		errorsv1.ErrUnexpectedResponseType(
			errorsv1.PluginActionServiceDeployResource,
		),
		errorsv1.PluginActionServiceDeployResource,
	)
}

func (r *resourceDeployServiceClientWrapper) toResourceDeployOutput(
	response *sharedtypesv1.DeployResourceCompleteResponse,
) (*provider.ResourceDeployOutput, error) {
	computedFieldValues, err := convertv1.FromPBMappingNodeMap(
		response.ComputedFieldValues,
	)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionServiceDeployResource,
		)
	}

	return &provider.ResourceDeployOutput{
		ComputedFieldValues: computedFieldValues,
	}, nil
}

func (r *resourceDeployServiceClientWrapper) Destroy(
	ctx context.Context,
	resourceType string,
	input *provider.ResourceDestroyInput,
) error {
	destroyReq, err := convertv1.ToPBDestroyResourceRequest(resourceType, input)
	if err != nil {
		return errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionServiceDestroyResource,
		)
	}

	response, err := r.client.DestroyResource(
		ctx,
		destroyReq,
	)
	if err != nil {
		return errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionServiceDestroyResource,
		)
	}

	switch result := response.Response.(type) {
	case *sharedtypesv1.DestroyResourceResponse_Result:
		if result.Result != nil && result.Result.Destroyed {
			return nil
		}
		return errorsv1.CreateGeneralError(
			errorsv1.ErrResourceNotDestroyed(
				resourceType,
				errorsv1.PluginActionServiceDestroyResource,
			),
			errorsv1.PluginActionServiceDestroyResource,
		)
	case *sharedtypesv1.DestroyResourceResponse_ErrorResponse:
		return errorsv1.CreateErrorFromResponse(
			result.ErrorResponse,
			errorsv1.PluginActionServiceDestroyResource,
		)
	}

	return errorsv1.CreateGeneralError(
		errorsv1.ErrUnexpectedResponseType(
			errorsv1.PluginActionServiceDestroyResource,
		),
		errorsv1.PluginActionServiceDestroyResource,
	)
}

// FunctionRegistryFromClient creates a new instance of a FunctionRegistry
// that uses the provided ServiceClient to interact with the deploy engine.
// This allows plugin implementations to interact with the deploy engine
// using the blueprint framework interfaces abstracting away the communication
// protocol from plugin developers.
func FunctionRegistryFromClient(
	client ServiceClient,
	hostID string,
) provider.FunctionRegistry {
	return &functionRegistryClientWrapper{
		client:    client,
		callStack: function.NewStack(),
		hostID:    hostID,
	}
}

type functionRegistryClientWrapper struct {
	client    ServiceClient
	callStack function.Stack
	hostID    string
}

func (f *functionRegistryClientWrapper) ForCallContext(
	stack function.Stack,
) provider.FunctionRegistry {
	return &functionRegistryClientWrapper{
		client:    f.client,
		callStack: stack,
		hostID:    f.hostID,
	}
}

func (f *functionRegistryClientWrapper) Call(
	ctx context.Context,
	functionName string,
	input *provider.FunctionCallInput,
) (*provider.FunctionCallOutput, error) {
	callReq, err := convertv1.ToPBFunctionCallRequest(ctx, functionName, input)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionServiceCallFunction,
		)
	}

	response, err := f.client.CallFunction(
		ctx,
		callReq,
	)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionServiceCallFunction,
		)
	}

	switch result := response.Response.(type) {
	case *sharedtypesv1.FunctionCallResponse_FunctionResult:
		return f.toFunctionCallOutput(result.FunctionResult)
	case *sharedtypesv1.FunctionCallResponse_ErrorResponse:
		return nil, errorsv1.CreateErrorFromResponse(
			result.ErrorResponse,
			errorsv1.PluginActionServiceCallFunction,
		)
	}

	return nil, errorsv1.CreateGeneralError(
		errorsv1.ErrUnexpectedResponseType(
			errorsv1.PluginActionServiceCallFunction,
		),
		errorsv1.PluginActionServiceCallFunction,
	)
}

func (f *functionRegistryClientWrapper) toFunctionCallOutput(
	response *sharedtypesv1.FunctionCallResult,
) (*provider.FunctionCallOutput, error) {
	funcCallOutput, err := convertv1.FromPBFunctionCallResult(response)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionServiceCallFunction,
		)
	}

	return funcCallOutput, nil
}

func (f *functionRegistryClientWrapper) GetDefinition(
	ctx context.Context,
	functionName string,
	input *provider.FunctionGetDefinitionInput,
) (*provider.FunctionGetDefinitionOutput, error) {
	definitionReq, err := convertv1.ToPBFunctionDefinitionRequest(
		functionName,
		input,
		f.hostID,
	)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionServiceGetFunctionDefinition,
		)
	}

	response, err := f.client.GetFunctionDefinition(
		ctx,
		definitionReq,
	)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionServiceGetFunctionDefinition,
		)
	}

	switch result := response.Response.(type) {
	case *sharedtypesv1.FunctionDefinitionResponse_FunctionDefinition:
		return convertv1.FromPBFunctionDefinitionResponse(
			result.FunctionDefinition,
			errorsv1.PluginActionServiceGetFunctionDefinition,
		)
	case *sharedtypesv1.FunctionDefinitionResponse_ErrorResponse:
		return nil, errorsv1.CreateErrorFromResponse(
			result.ErrorResponse,
			errorsv1.PluginActionServiceGetFunctionDefinition,
		)
	}

	return nil, errorsv1.CreateGeneralError(
		errorsv1.ErrUnexpectedResponseType(
			errorsv1.PluginActionServiceGetFunctionDefinition,
		),
		errorsv1.PluginActionServiceGetFunctionDefinition,
	)
}

func (f *functionRegistryClientWrapper) HasFunction(
	ctx context.Context,
	functionName string,
) (bool, error) {
	return false, nil
}

func (f *functionRegistryClientWrapper) ListFunctions(
	ctx context.Context,
) ([]string, error) {
	return nil, nil
}
