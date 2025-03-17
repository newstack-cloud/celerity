package providerv1

import (
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/serialisation"
	"github.com/two-hundred/celerity/libs/plugin-framework/convertv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/providerserverv1"
)

func fromPBCustomValidateResourceRequest(
	req *providerserverv1.CustomValidateResourceRequest,
) (*provider.ResourceValidateInput, error) {
	resource, err := serialisation.FromResourcePB(req.SchemaResource)
	if err != nil {
		return nil, err
	}

	providerCtx, err := convertv1.FromPBProviderContext(req.Context)
	if err != nil {
		return nil, err
	}

	return &provider.ResourceValidateInput{
		SchemaResource:  resource,
		ProviderContext: providerCtx,
	}, nil
}

func fromPBGetResourceExternalStateRequest(
	req *providerserverv1.GetResourceExternalStateRequest,
) (*provider.ResourceGetExternalStateInput, error) {
	providerCtx, err := convertv1.FromPBProviderContext(req.Context)
	if err != nil {
		return nil, err
	}

	currentResourceSpec, err := serialisation.FromMappingNodePB(
		req.CurrentResourceSpec,
		/* optional */ true,
	)
	if err != nil {
		return nil, err
	}

	currentResourceMetadata, err := convertv1.FromPBResourceMetadataState(req.CurrentResourceMetadata)
	if err != nil {
		return nil, err
	}

	return &provider.ResourceGetExternalStateInput{
		InstanceID:              req.InstanceId,
		ResourceID:              req.ResourceId,
		CurrentResourceSpec:     currentResourceSpec,
		CurrentResourceMetadata: currentResourceMetadata,
		ProviderContext:         providerCtx,
	}, nil
}
