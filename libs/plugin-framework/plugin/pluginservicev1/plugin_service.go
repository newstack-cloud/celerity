package pluginservicev1

import (
	context "context"
	"fmt"
	"time"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/plugin-framework/plugin/convertv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/plugin/errorsv1"
	sharedtypesv1 "github.com/two-hundred/celerity/libs/plugin-framework/plugin/sharedtypesv1"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	// DefaultPluginToPluginCallTimeout is the default timeout
	// in seconds for plugin to plugin calls.
	// This includes invoking functions, deploying resources and more.
	DefaultPluginToPluginCallTimeout = 120
)

type pluginServiceServer struct {
	UnimplementedServiceServer
	manager                   Manager
	functionRegistry          provider.FunctionRegistry
	resourceDeployService     provider.ResourceDeployService
	hostID                    string
	pluginToPluginCallTimeout int
}

// ServiceServerOption is a function that configures a service server.
type ServiceServerOption func(*pluginServiceServer)

// WithPluginToPluginCallTimeout is a service server option that sets the timeout
// in seconds for plugin to plugin calls.
// This covers the case where a provider or transformer plugin uses the plugin service
// to invoke functions or deploy resources as these actions will call plugins.
//
// When not provided, the default timeout is 120 seconds.
func WithPluginToPluginCallTimeout(timeout int) ServiceServerOption {
	return func(s *pluginServiceServer) {
		s.pluginToPluginCallTimeout = timeout
	}
}

// NewServiceServer creates a new gRPC server for the plugin service
// that manages registration and deregistration of plugins along with
// allowing a subset of plugin functionality to make calls to other plugins.
func NewServiceServer(
	pluginManager Manager,
	functionRegistry provider.FunctionRegistry,
	resourceDeployService provider.ResourceDeployService,
	hostID string,
	opts ...ServiceServerOption,
) ServiceServer {
	server := &pluginServiceServer{
		manager:                   pluginManager,
		functionRegistry:          functionRegistry,
		resourceDeployService:     resourceDeployService,
		hostID:                    hostID,
		pluginToPluginCallTimeout: DefaultPluginToPluginCallTimeout,
	}

	for _, opt := range opts {
		opt(server)
	}

	return server
}

func (s *pluginServiceServer) Register(
	ctx context.Context,
	req *PluginRegistrationRequest,
) (*PluginRegistrationResponse, error) {
	err := s.manager.RegisterPlugin(
		&PluginInstanceInfo{
			PluginType:      req.PluginType,
			ProtocolVersion: req.ProtocolVersion,
			ID:              req.PluginId,
			InstanceID:      req.InstanceId,
			TCPPort:         int(req.Port),
			UnixSocketPath:  req.UnixSocket,
		},
	)
	if err != nil {
		return &PluginRegistrationResponse{
			Success: false,
			Message: fmt.Sprintf(
				"failed to register plugin due to error: %s",
				err.Error(),
			),
			HostId: s.hostID,
		}, nil
	}

	return &PluginRegistrationResponse{
		Success: true,
		Message: "plugin registered successfully",
		HostId:  s.hostID,
	}, nil
}

func (s *pluginServiceServer) Deregister(
	ctx context.Context,
	req *PluginDeregistrationRequest,
) (*PluginDeregistrationResponse, error) {
	if req.HostId != s.hostID {
		return &PluginDeregistrationResponse{
			Success: false,
			Message: fmt.Sprintf(
				"failed to deregister plugin due to error: host id mismatch, expected %q, got %q",
				s.hostID,
				req.HostId,
			),
		}, nil
	}

	err := s.manager.DeregisterPlugin(
		req.PluginType,
		req.InstanceId,
	)
	if err != nil {
		return &PluginDeregistrationResponse{
			Success: false,
			Message: fmt.Sprintf(
				"failed to deregister plugin due to error: %s",
				err.Error(),
			),
		}, nil
	}

	return &PluginDeregistrationResponse{
		Success: true,
		Message: "plugin deregistered successfully",
	}, nil
}

func (s *pluginServiceServer) CallFunction(
	ctx context.Context,
	req *sharedtypesv1.FunctionCallRequest,
) (*sharedtypesv1.FunctionCallResponse, error) {
	input, err := convertv1.FromPBFunctionCallRequest(req, s.functionRegistry)
	if err != nil {
		return convertv1.ToPBFunctionCallErrorResponse(err), nil
	}

	ctxWithTimeout, cancel := context.WithTimeout(
		ctx,
		time.Duration(s.pluginToPluginCallTimeout)*time.Second,
	)
	defer cancel()

	output, err := s.functionRegistry.Call(
		ctxWithTimeout,
		req.FunctionName,
		input,
	)
	if err != nil {
		return convertv1.ToPBFunctionCallErrorResponse(err), nil
	}

	response, err := convertv1.ToPBFunctionCallResponse(output)
	if err != nil {
		return convertv1.ToPBFunctionCallErrorResponse(err), nil
	}

	return response, nil
}

func (s *pluginServiceServer) GetFunctionDefinition(
	ctx context.Context,
	req *sharedtypesv1.FunctionDefinitionRequest,
) (*sharedtypesv1.FunctionDefinitionResponse, error) {
	input, err := convertv1.FromPBFunctionDefinitionRequest(req)
	if err != nil {
		return convertv1.ToPBFunctionDefinitionErrorResponse(err), nil
	}

	ctxWithTimeout, cancel := context.WithTimeout(
		ctx,
		time.Duration(s.pluginToPluginCallTimeout)*time.Second,
	)
	defer cancel()

	output, err := s.functionRegistry.GetDefinition(
		ctxWithTimeout,
		req.FunctionName,
		input,
	)
	if err != nil {
		return convertv1.ToPBFunctionDefinitionErrorResponse(err), nil
	}

	response, err := convertv1.ToPBFunctionDefinitionResponse(output.Definition)
	if err != nil {
		return convertv1.ToPBFunctionDefinitionErrorResponse(err), nil
	}

	return response, nil
}

func (s *pluginServiceServer) HasFunction(
	ctx context.Context,
	req *HasFunctionRequest,
) (*HasFunctionResponse, error) {
	ctxWithTimeout, cancel := context.WithTimeout(
		ctx,
		time.Duration(s.pluginToPluginCallTimeout)*time.Second,
	)
	defer cancel()

	hasFunction, err := s.functionRegistry.HasFunction(ctxWithTimeout, req.FunctionName)
	if err != nil {
		return toHasFunctionErrorRespponse(err), nil
	}

	return &HasFunctionResponse{
		Response: &HasFunctionResponse_FunctionCheckResult{
			FunctionCheckResult: &FunctionCheckResult{
				HasFunction: hasFunction,
			},
		},
	}, nil
}

func (s *pluginServiceServer) ListFunctions(
	ctx context.Context,
	_ *emptypb.Empty,
) (*ListFunctionsResponse, error) {
	ctxWithTimeout, cancel := context.WithTimeout(
		ctx,
		time.Duration(s.pluginToPluginCallTimeout)*time.Second,
	)
	defer cancel()

	functions, err := s.functionRegistry.ListFunctions(ctxWithTimeout)
	if err != nil {
		return toListFunctionsErrorResponse(err), nil
	}

	return &ListFunctionsResponse{
		Response: &ListFunctionsResponse_FunctionList{
			FunctionList: &FunctionList{
				Functions: functions,
			},
		},
	}, nil
}

func (s *pluginServiceServer) DeployResource(
	ctx context.Context,
	req *DeployResourceServiceRequest,
) (*sharedtypesv1.DeployResourceResponse, error) {
	input, err := convertv1.FromPBDeployResourceRequest(req.DeployRequest)
	if err != nil {
		return convertv1.ToPBDeployResourceErrorResponse(err), nil
	}

	ctxWithTimeout, cancel := context.WithTimeout(
		ctx,
		time.Duration(s.pluginToPluginCallTimeout)*time.Second,
	)
	defer cancel()

	output, err := s.resourceDeployService.Deploy(
		ctxWithTimeout,
		convertv1.ResourceTypeToString(req.DeployRequest.ResourceType),
		&provider.ResourceDeployServiceInput{
			WaitUntilStable: req.WaitUntilStable,
			DeployInput:     input,
		},
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

func (s *pluginServiceServer) DestroyResource(
	ctx context.Context,
	req *sharedtypesv1.DestroyResourceRequest,
) (*sharedtypesv1.DestroyResourceResponse, error) {
	input, err := convertv1.FromPBDestroyResourceRequest(req)
	if err != nil {
		return convertv1.ToPBDestroyResourceErrorResponse(err), nil
	}

	ctxWithTimeout, cancel := context.WithTimeout(
		ctx,
		time.Duration(s.pluginToPluginCallTimeout)*time.Second,
	)
	defer cancel()

	err = s.resourceDeployService.Destroy(
		ctxWithTimeout,
		convertv1.ResourceTypeToString(req.ResourceType),
		input,
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

func toHasFunctionErrorRespponse(err error) *HasFunctionResponse {
	return &HasFunctionResponse{
		Response: &HasFunctionResponse_ErrorResponse{
			ErrorResponse: errorsv1.CreateResponseFromError(err),
		},
	}
}

func toListFunctionsErrorResponse(err error) *ListFunctionsResponse {
	return &ListFunctionsResponse{
		Response: &ListFunctionsResponse_ErrorResponse{
			ErrorResponse: errorsv1.CreateResponseFromError(err),
		},
	}
}
