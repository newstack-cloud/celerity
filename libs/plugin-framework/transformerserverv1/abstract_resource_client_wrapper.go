package transformerserverv1

import (
	context "context"

	"github.com/two-hundred/celerity/libs/blueprint/transform"
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
	return nil, nil
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
