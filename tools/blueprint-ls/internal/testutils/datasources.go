package testutils

import (
	"context"

	"github.com/newstack-cloud/celerity/libs/blueprint/core"
	"github.com/newstack-cloud/celerity/libs/blueprint/provider"
)

type VPCDataSource struct{}

func (d *VPCDataSource) GetSpecDefinition(
	ctx context.Context,
	input *provider.DataSourceGetSpecDefinitionInput,
) (*provider.DataSourceGetSpecDefinitionOutput, error) {
	return &provider.DataSourceGetSpecDefinitionOutput{
		SpecDefinition: &provider.DataSourceSpecDefinition{
			Fields: map[string]*provider.DataSourceSpecSchema{
				"instanceConfigId": {
					Type: provider.DataSourceSpecTypeString,
				},
			},
		},
	}, nil
}

func (d *VPCDataSource) Fetch(
	ctx context.Context,
	input *provider.DataSourceFetchInput,
) (*provider.DataSourceFetchOutput, error) {
	return &provider.DataSourceFetchOutput{
		Data: map[string]*core.MappingNode{},
	}, nil
}

func (d *VPCDataSource) GetType(
	ctx context.Context,
	input *provider.DataSourceGetTypeInput,
) (*provider.DataSourceGetTypeOutput, error) {
	return &provider.DataSourceGetTypeOutput{
		Type: "aws/vpc",
	}, nil
}

func (d *VPCDataSource) GetTypeDescription(
	ctx context.Context,
	input *provider.DataSourceGetTypeDescriptionInput,
) (*provider.DataSourceGetTypeDescriptionOutput, error) {
	return &provider.DataSourceGetTypeDescriptionOutput{
		MarkdownDescription:  "# VPC\n\n A Virtual Private Cloud (VPC) in AWS.",
		PlainTextDescription: "",
	}, nil
}

func (d *VPCDataSource) GetFilterFields(
	ctx context.Context,
	input *provider.DataSourceGetFilterFieldsInput,
) (*provider.DataSourceGetFilterFieldsOutput, error) {
	return &provider.DataSourceGetFilterFieldsOutput{
		Fields: []string{"instanceConfigId", "tags"},
	}, nil
}

func (d *VPCDataSource) CustomValidate(
	ctx context.Context,
	input *provider.DataSourceValidateInput,
) (*provider.DataSourceValidateOutput, error) {
	return &provider.DataSourceValidateOutput{}, nil
}

func (d *VPCDataSource) GetExamples(
	ctx context.Context,
	input *provider.DataSourceGetExamplesInput,
) (*provider.DataSourceGetExamplesOutput, error) {
	return &provider.DataSourceGetExamplesOutput{
		PlainTextExamples: []string{},
		MarkdownExamples:  []string{},
	}, nil
}
