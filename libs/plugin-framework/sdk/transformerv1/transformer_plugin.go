package transformerv1

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/transform"
	"github.com/two-hundred/celerity/libs/plugin-framework/convertv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/pluginservicev1"
	"github.com/two-hundred/celerity/libs/plugin-framework/sdk/pluginutils"
	"github.com/two-hundred/celerity/libs/plugin-framework/sharedtypesv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/transformerserverv1"
)

// NewTransformerPlugin creates a new instance of a transformer plugin
// from a blueprint framework transform.SpecTransforemr implementation.
// This produces a gRPC server plugin that the deploy engine host
// can interact with.
// The `TransformerPluginDefinition` utility type can be passed in to
// create a transformer plugin server as it implements the `transform.SpecTransformer`
// interface.
//
// The host info container is used to retrieve the ID of the host
// that the plugin was registered with.
//
// The service client is used to communicate with other plugins
// that are registered with the deploy engine host.
func NewTransformerPlugin(
	bpTransformer transform.SpecTransformer,
	hostInfoContainer pluginutils.HostInfoContainer,
	serviceClient pluginservicev1.ServiceClient,
) transformerserverv1.TransformerServer {
	return &blueprintTransformerPluginImpl{
		bpTransformer:     bpTransformer,
		hostInfoContainer: hostInfoContainer,
		serviceClient:     serviceClient,
	}
}

type blueprintTransformerPluginImpl struct {
	transformerserverv1.UnimplementedTransformerServer
	bpTransformer     transform.SpecTransformer
	hostInfoContainer pluginutils.HostInfoContainer
	serviceClient     pluginservicev1.ServiceClient
}

func (p *blueprintTransformerPluginImpl) GetTransformName(
	ctx context.Context,
	req *transformerserverv1.TransformerRequest,
) (*transformerserverv1.TransformNameResponse, error) {
	err := p.checkHostID(req.HostId)
	if err != nil {
		return toTransformNameErrorResponse(err), nil
	}

	transformName, err := p.bpTransformer.GetTransformName(ctx)
	if err != nil {
		return toTransformNameErrorResponse(err), nil
	}

	return toPBTransformNameResponse(transformName), nil
}

func (p *blueprintTransformerPluginImpl) GetConfigDefinition(
	ctx context.Context,
	req *transformerserverv1.TransformerRequest,
) (*sharedtypesv1.ConfigDefinitionResponse, error) {
	err := p.checkHostID(req.HostId)
	if err != nil {
		return convertv1.ToPBConfigDefinitionErrorResponse(err), nil
	}

	configDefinition, err := p.bpTransformer.ConfigDefinition(ctx)
	if err != nil {
		return convertv1.ToPBConfigDefinitionErrorResponse(err), nil
	}

	configDefinitionPB, err := convertv1.ToPBConfigDefinitionResponse(
		configDefinition,
	)
	if err != nil {
		return convertv1.ToPBConfigDefinitionErrorResponse(err), nil
	}

	return configDefinitionPB, nil
}

func (p *blueprintTransformerPluginImpl) ListAbstractResourceTypes(
	ctx context.Context,
	req *transformerserverv1.TransformerRequest,
) (*transformerserverv1.AbstractResourceTypesResponse, error) {
	return nil, nil
}

func (p *blueprintTransformerPluginImpl) CustomValidateAbstractResource(
	ctx context.Context,
	req *transformerserverv1.CustomValidateAbstractResourceRequest,
) (*transformerserverv1.CustomValidateAbstractResourceResponse, error) {
	return nil, nil
}

func (p *blueprintTransformerPluginImpl) GetAbstractResourceSpecDefinition(
	ctx context.Context,
	req *transformerserverv1.AbstractResourceRequest,
) (*transformerserverv1.AbstractResourceSpecDefinitionResponse, error) {
	return nil, nil
}

func (p *blueprintTransformerPluginImpl) CanAbstractResourceLinkTo(
	ctx context.Context,
	req *transformerserverv1.AbstractResourceRequest,
) (*transformerserverv1.CanAbstractResourceLinkToResponse, error) {
	return nil, nil
}

func (p *blueprintTransformerPluginImpl) IsAbstractResourceCommonTerminal(
	ctx context.Context,
	req *transformerserverv1.AbstractResourceRequest,
) (*transformerserverv1.IsAbstractResourceCommonTerminalResponse, error) {
	return nil, nil
}

func (p *blueprintTransformerPluginImpl) GetAbstractResourceType(
	ctx context.Context,
	req *transformerserverv1.AbstractResourceRequest,
) (*sharedtypesv1.ResourceTypeResponse, error) {
	return nil, nil
}

func (p *blueprintTransformerPluginImpl) GetAbstractResourceTypeDescription(
	ctx context.Context,
	req *transformerserverv1.AbstractResourceRequest,
) (*sharedtypesv1.TypeDescriptionResponse, error) {
	return nil, nil
}

func (p *blueprintTransformerPluginImpl) GetAbstractResourceExamples(
	ctx context.Context,
	req *transformerserverv1.AbstractResourceRequest,
) (*sharedtypesv1.ExamplesResponse, error) {
	return nil, nil
}

func (p *blueprintTransformerPluginImpl) checkHostID(hostID string) error {
	if hostID != p.hostInfoContainer.GetID() {
		return errorsv1.ErrInvalidHostID(hostID)
	}

	return nil
}
