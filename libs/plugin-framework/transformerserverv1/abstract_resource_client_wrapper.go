package transformerserverv1

import (
	context "context"

	"github.com/two-hundred/celerity/libs/blueprint/serialisation"
	"github.com/two-hundred/celerity/libs/blueprint/transform"
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
	sharedtypesv1 "github.com/two-hundred/celerity/libs/plugin-framework/sharedtypesv1"
)

type abstractResourceTransformerClientWrapper struct {
	client               TransformerClient
	abstractResourceType string
	hostID               string
}

func (t *abstractResourceTransformerClientWrapper) CustomValidate(
	ctx context.Context,
	input *transform.AbstractResourceValidateInput,
) (*transform.AbstractResourceValidateOutput, error) {
	schemaResourcePB, err := serialisation.ToResourcePB(input.SchemaResource)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionTransformerCustomValidateAbstractResource,
		)
	}

	transformerCtx, err := toPBTransformerContext(input.TransformerContext)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionTransformerCustomValidateAbstractResource,
		)
	}

	response, err := t.client.CustomValidateAbstractResource(
		ctx,
		&CustomValidateAbstractResourceRequest{
			AbstractResourceType: &sharedtypesv1.ResourceType{
				Type: t.abstractResourceType,
			},
			HostId:         t.hostID,
			SchemaResource: schemaResourcePB,
			Context:        transformerCtx,
		},
	)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionTransformerCustomValidateAbstractResource,
		)
	}

	switch result := response.Response.(type) {
	case *CustomValidateAbstractResourceResponse_CompleteResponse:
		return &transform.AbstractResourceValidateOutput{
			Diagnostics: sharedtypesv1.ToCoreDiagnostics(result.CompleteResponse.GetDiagnostics()),
		}, nil
	case *CustomValidateAbstractResourceResponse_ErrorResponse:
		return nil, errorsv1.CreateErrorFromResponse(
			result.ErrorResponse,
			errorsv1.PluginActionTransformerCustomValidateAbstractResource,
		)
	}

	return nil, errorsv1.CreateGeneralError(
		errorsv1.ErrUnexpectedResponseType(
			errorsv1.PluginActionTransformerCustomValidateAbstractResource,
		),
		errorsv1.PluginActionTransformerCustomValidateAbstractResource,
	)
}

func (t *abstractResourceTransformerClientWrapper) GetSpecDefinition(
	ctx context.Context,
	input *transform.AbstractResourceGetSpecDefinitionInput,
) (*transform.AbstractResourceGetSpecDefinitionOutput, error) {
	return nil, nil
}

func (t *abstractResourceTransformerClientWrapper) CanLinkTo(
	ctx context.Context,
	input *transform.AbstractResourceCanLinkToInput,
) (*transform.AbstractResourceCanLinkToOutput, error) {
	return nil, nil
}

func (t *abstractResourceTransformerClientWrapper) IsCommonTerminal(
	ctx context.Context,
	input *transform.AbstractResourceIsCommonTerminalInput,
) (*transform.AbstractResourceIsCommonTerminalOutput, error) {
	return nil, nil
}

func (t *abstractResourceTransformerClientWrapper) GetType(
	ctx context.Context,
	input *transform.AbstractResourceGetTypeInput,
) (*transform.AbstractResourceGetTypeOutput, error) {
	return nil, nil
}

func (t *abstractResourceTransformerClientWrapper) GetTypeDescription(
	ctx context.Context,
	input *transform.AbstractResourceGetTypeDescriptionInput,
) (*transform.AbstractResourceGetTypeDescriptionOutput, error) {
	return nil, nil
}

func (t *abstractResourceTransformerClientWrapper) GetExamples(
	ctx context.Context,
	input *transform.AbstractResourceGetExamplesInput,
) (*transform.AbstractResourceGetExamplesOutput, error) {
	return nil, nil
}
