package providerserverv1

import (
	context "context"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/serialisation"
	"github.com/two-hundred/celerity/libs/plugin-framework/convertv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
	sharedtypesv1 "github.com/two-hundred/celerity/libs/plugin-framework/sharedtypesv1"
)

type dataSourceProviderClientWrapper struct {
	client         ProviderClient
	dataSourceType string
	hostID         string
}

func (d *dataSourceProviderClientWrapper) GetType(
	ctx context.Context,
	input *provider.DataSourceGetTypeInput,
) (*provider.DataSourceGetTypeOutput, error) {
	providerCtx, err := convertv1.ToPBProviderContext(input.ProviderContext)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionProviderGetDataSourceType,
		)
	}

	response, err := d.client.GetDataSourceType(
		ctx,
		&DataSourceRequest{
			DataSourceType: &DataSourceType{
				Type: d.dataSourceType,
			},
			HostId:  d.hostID,
			Context: providerCtx,
		},
	)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionProviderGetDataSourceType,
		)
	}

	switch result := response.Response.(type) {
	case *DataSourceTypeResponse_DataSourceTypeInfo:
		return &provider.DataSourceGetTypeOutput{
			Type:  result.DataSourceTypeInfo.Type.Type,
			Label: result.DataSourceTypeInfo.Label,
		}, nil
	case *DataSourceTypeResponse_ErrorResponse:
		return nil, errorsv1.CreateErrorFromResponse(
			result.ErrorResponse,
			errorsv1.PluginActionProviderGetDataSourceType,
		)
	}

	return nil, errorsv1.CreateGeneralError(
		errorsv1.ErrUnexpectedResponseType(
			errorsv1.PluginActionProviderGetDataSourceType,
		),
		errorsv1.PluginActionProviderGetDataSourceType,
	)
}

func (d *dataSourceProviderClientWrapper) GetTypeDescription(
	ctx context.Context,
	input *provider.DataSourceGetTypeDescriptionInput,
) (*provider.DataSourceGetTypeDescriptionOutput, error) {
	return nil, nil
}

func (d *dataSourceProviderClientWrapper) CustomValidate(
	ctx context.Context,
	input *provider.DataSourceValidateInput,
) (*provider.DataSourceValidateOutput, error) {
	schemaDataSourcePB, err := serialisation.ToDataSourcePB(input.SchemaDataSource)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionProviderCustomValidateDataSource,
		)
	}

	providerCtx, err := convertv1.ToPBProviderContext(input.ProviderContext)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionProviderCustomValidateDataSource,
		)
	}

	response, err := d.client.CustomValidateDataSource(
		ctx,
		&CustomValidateDataSourceRequest{
			DataSourceType: &DataSourceType{
				Type: d.dataSourceType,
			},
			HostId:           d.hostID,
			SchemaDataSource: schemaDataSourcePB,
			Context:          providerCtx,
		},
	)
	if err != nil {
		return nil, errorsv1.CreateGeneralError(
			err,
			errorsv1.PluginActionProviderCustomValidateDataSource,
		)
	}

	switch result := response.Response.(type) {
	case *CustomValidateDataSourceResponse_CompleteResponse:
		return &provider.DataSourceValidateOutput{
			Diagnostics: sharedtypesv1.ToCoreDiagnostics(
				result.CompleteResponse.GetDiagnostics(),
			),
		}, nil
	case *CustomValidateDataSourceResponse_ErrorResponse:
		return nil, errorsv1.CreateErrorFromResponse(
			result.ErrorResponse,
			errorsv1.PluginActionProviderCustomValidateDataSource,
		)
	}

	return nil, errorsv1.CreateGeneralError(
		errorsv1.ErrUnexpectedResponseType(
			errorsv1.PluginActionProviderCustomValidateDataSource,
		),
		errorsv1.PluginActionProviderCustomValidateDataSource,
	)
}

func (d *dataSourceProviderClientWrapper) GetSpecDefinition(
	ctx context.Context,
	input *provider.DataSourceGetSpecDefinitionInput,
) (*provider.DataSourceGetSpecDefinitionOutput, error) {
	return nil, nil
}

func (d *dataSourceProviderClientWrapper) GetFilterFields(
	ctx context.Context,
	input *provider.DataSourceGetFilterFieldsInput,
) (*provider.DataSourceGetFilterFieldsOutput, error) {
	return nil, nil
}

func (d *dataSourceProviderClientWrapper) Fetch(
	ctx context.Context,
	input *provider.DataSourceFetchInput,
) (*provider.DataSourceFetchOutput, error) {
	return nil, nil
}

func (d *dataSourceProviderClientWrapper) GetExamples(
	ctx context.Context,
	input *provider.DataSourceGetExamplesInput,
) (*provider.DataSourceGetExamplesOutput, error) {
	return nil, nil
}
