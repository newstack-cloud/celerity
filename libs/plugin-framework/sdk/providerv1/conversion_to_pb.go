package providerv1

import (
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/serialisation"
	commoncore "github.com/two-hundred/celerity/libs/common/core"
	"github.com/two-hundred/celerity/libs/plugin-framework/convertv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/providerserverv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/sharedtypesv1"
)

func toProviderNamespaceResponse(namespace string) *providerserverv1.NamespaceResponse {
	return &providerserverv1.NamespaceResponse{
		Response: &providerserverv1.NamespaceResponse_Namespace{
			Namespace: &providerserverv1.Namespace{
				Namespace: namespace,
			},
		},
	}
}

func toProviderNamespaceErrorResponse(err error) *providerserverv1.NamespaceResponse {
	return &providerserverv1.NamespaceResponse{
		Response: &providerserverv1.NamespaceResponse_ErrorResponse{
			ErrorResponse: errorsv1.CreateResponseFromError(err),
		},
	}
}

func toResourceTypesResponse(resourceTypes []string) *providerserverv1.ResourceTypesResponse {
	return &providerserverv1.ResourceTypesResponse{
		Response: &providerserverv1.ResourceTypesResponse_ResourceTypes{
			ResourceTypes: &providerserverv1.ResourceTypes{
				ResourceTypes: toPBResourceTypes(resourceTypes),
			},
		},
	}
}

func toResourceTypesErrorResponse(err error) *providerserverv1.ResourceTypesResponse {
	return &providerserverv1.ResourceTypesResponse{
		Response: &providerserverv1.ResourceTypesResponse_ErrorResponse{
			ErrorResponse: errorsv1.CreateResponseFromError(err),
		},
	}
}

func toDataSourceTypesResponse(dataSourceTypes []string) *providerserverv1.DataSourceTypesResponse {
	return &providerserverv1.DataSourceTypesResponse{
		Response: &providerserverv1.DataSourceTypesResponse_DataSourceTypes{
			DataSourceTypes: &providerserverv1.DataSourceTypes{
				DataSourceTypes: commoncore.Map(
					dataSourceTypes,
					func(dataSourceType string, _ int) *providerserverv1.DataSourceType {
						return &providerserverv1.DataSourceType{
							Type: dataSourceType,
						}
					},
				),
			},
		},
	}
}

func toDataSourceTypesErrorResponse(err error) *providerserverv1.DataSourceTypesResponse {
	return &providerserverv1.DataSourceTypesResponse{
		Response: &providerserverv1.DataSourceTypesResponse_ErrorResponse{
			ErrorResponse: errorsv1.CreateResponseFromError(err),
		},
	}
}

func toCustomVariableTypesResponse(customVariableTypes []string) *providerserverv1.CustomVariableTypesResponse {
	return &providerserverv1.CustomVariableTypesResponse{
		Response: &providerserverv1.CustomVariableTypesResponse_CustomVariableTypes{
			CustomVariableTypes: &providerserverv1.CustomVariableTypes{
				CustomVariableTypes: commoncore.Map(
					customVariableTypes,
					func(customVariableType string, _ int) *providerserverv1.CustomVariableType {
						return &providerserverv1.CustomVariableType{
							Type: customVariableType,
						}
					},
				),
			},
		},
	}
}

func toCustomVariableTypesErrorResponse(err error) *providerserverv1.CustomVariableTypesResponse {
	return &providerserverv1.CustomVariableTypesResponse{
		Response: &providerserverv1.CustomVariableTypesResponse_ErrorResponse{
			ErrorResponse: errorsv1.CreateResponseFromError(err),
		},
	}
}

func toFunctionsResponse(functions []string) *providerserverv1.FunctionListResponse {
	return &providerserverv1.FunctionListResponse{
		Response: &providerserverv1.FunctionListResponse_FunctionList{
			FunctionList: &providerserverv1.FunctionList{
				Functions: functions,
			},
		},
	}
}

func toFunctionsErrorResponse(err error) *providerserverv1.FunctionListResponse {
	return &providerserverv1.FunctionListResponse{
		Response: &providerserverv1.FunctionListResponse_ErrorResponse{
			ErrorResponse: errorsv1.CreateResponseFromError(err),
		},
	}
}

func toRetryPolicyResponse(policy *provider.RetryPolicy) *providerserverv1.RetryPolicyResponse {
	if policy == nil {
		return &providerserverv1.RetryPolicyResponse{
			Response: &providerserverv1.RetryPolicyResponse_RetryPolicy{
				RetryPolicy: nil,
			},
		}
	}

	return &providerserverv1.RetryPolicyResponse{
		Response: &providerserverv1.RetryPolicyResponse_RetryPolicy{
			RetryPolicy: &providerserverv1.RetryPolicy{
				MaxRetries:      int32(policy.MaxRetries),
				FirstRetryDelay: policy.FirstRetryDelay,
				MaxDelay:        policy.MaxDelay,
				BackoffFactor:   policy.BackoffFactor,
				Jitter:          policy.Jitter,
			},
		},
	}
}

func toRetryPolicyErrorResponse(err error) *providerserverv1.RetryPolicyResponse {
	return &providerserverv1.RetryPolicyResponse{
		Response: &providerserverv1.RetryPolicyResponse_ErrorResponse{
			ErrorResponse: errorsv1.CreateResponseFromError(err),
		},
	}
}

func toCustomValidateResourceResponse(
	output *provider.ResourceValidateOutput,
) *providerserverv1.CustomValidateResourceResponse {
	if output == nil {
		return &providerserverv1.CustomValidateResourceResponse{
			Response: &providerserverv1.CustomValidateResourceResponse_ErrorResponse{
				ErrorResponse: sharedtypesv1.NoResponsePBError(),
			},
		}
	}

	return &providerserverv1.CustomValidateResourceResponse{
		Response: &providerserverv1.CustomValidateResourceResponse_CompleteResponse{
			CompleteResponse: &providerserverv1.CustomValidateResourceCompleteResponse{
				Diagnostics: sharedtypesv1.ToPBDiagnostics(output.Diagnostics),
			},
		},
	}
}

func toCustomValidateErrorResponse(
	err error,
) *providerserverv1.CustomValidateResourceResponse {
	return &providerserverv1.CustomValidateResourceResponse{
		Response: &providerserverv1.CustomValidateResourceResponse_ErrorResponse{
			ErrorResponse: errorsv1.CreateResponseFromError(err),
		},
	}
}

func toPBResourceSpecDefinitionResponse(
	output *provider.ResourceGetSpecDefinitionOutput,
) (*providerserverv1.ResourceSpecDefinitionResponse, error) {
	if output == nil {
		return &providerserverv1.ResourceSpecDefinitionResponse{
			Response: &providerserverv1.ResourceSpecDefinitionResponse_ErrorResponse{
				ErrorResponse: sharedtypesv1.NoResponsePBError(),
			},
		}, nil
	}

	schema, err := convertv1.ToPBResourceDefinitionsSchema(output.SpecDefinition.Schema)
	if err != nil {
		return nil, err
	}

	return &providerserverv1.ResourceSpecDefinitionResponse{
		Response: &providerserverv1.ResourceSpecDefinitionResponse_SpecDefinition{
			SpecDefinition: &sharedtypesv1.ResourceSpecDefinition{
				Schema:  schema,
				IdField: output.SpecDefinition.IDField,
			},
		},
	}, nil
}

func toResourceSpecDefinitionErrorResponse(
	err error,
) *providerserverv1.ResourceSpecDefinitionResponse {
	return &providerserverv1.ResourceSpecDefinitionResponse{
		Response: &providerserverv1.ResourceSpecDefinitionResponse_ErrorResponse{
			ErrorResponse: errorsv1.CreateResponseFromError(err),
		},
	}
}

func toCanResourceLinkToErrorResponse(
	err error,
) *providerserverv1.CanResourceLinkToResponse {
	return &providerserverv1.CanResourceLinkToResponse{
		Response: &providerserverv1.CanResourceLinkToResponse_ErrorResponse{
			ErrorResponse: errorsv1.CreateResponseFromError(err),
		},
	}
}

func toCanResourceLinkToResponse(
	output *provider.ResourceCanLinkToOutput,
) *providerserverv1.CanResourceLinkToResponse {
	if output == nil {
		return &providerserverv1.CanResourceLinkToResponse{
			Response: &providerserverv1.CanResourceLinkToResponse_ErrorResponse{
				ErrorResponse: sharedtypesv1.NoResponsePBError(),
			},
		}
	}

	return &providerserverv1.CanResourceLinkToResponse{
		Response: &providerserverv1.CanResourceLinkToResponse_ResourceTypes{
			ResourceTypes: &sharedtypesv1.CanLinkTo{
				ResourceTypes: toPBResourceTypes(output.CanLinkTo),
			},
		},
	}
}

func toResourceStabilisedDepsResponse(
	output *provider.ResourceStabilisedDependenciesOutput,
) *providerserverv1.ResourceStabilisedDepsResponse {
	if output == nil {
		return &providerserverv1.ResourceStabilisedDepsResponse{
			Response: &providerserverv1.ResourceStabilisedDepsResponse_ErrorResponse{
				ErrorResponse: sharedtypesv1.NoResponsePBError(),
			},
		}
	}

	return &providerserverv1.ResourceStabilisedDepsResponse{
		Response: &providerserverv1.ResourceStabilisedDepsResponse_StabilisedDependencies{
			StabilisedDependencies: &providerserverv1.StabilisedDependencies{
				ResourceTypes: toPBResourceTypes(output.StabilisedDependencies),
			},
		},
	}
}

func toResourceStabilisedDepsErrorResponse(
	err error,
) *providerserverv1.ResourceStabilisedDepsResponse {
	return &providerserverv1.ResourceStabilisedDepsResponse{
		Response: &providerserverv1.ResourceStabilisedDepsResponse_ErrorResponse{
			ErrorResponse: errorsv1.CreateResponseFromError(err),
		},
	}
}

func toIsResourceCommonTerminalResponse(
	output *provider.ResourceIsCommonTerminalOutput,
) *providerserverv1.IsResourceCommonTerminalResponse {
	if output == nil {
		return &providerserverv1.IsResourceCommonTerminalResponse{
			Response: &providerserverv1.IsResourceCommonTerminalResponse_ErrorResponse{
				ErrorResponse: sharedtypesv1.NoResponsePBError(),
			},
		}
	}

	return &providerserverv1.IsResourceCommonTerminalResponse{
		Response: &providerserverv1.IsResourceCommonTerminalResponse_Data{
			Data: &sharedtypesv1.ResourceCommonTerminalInfo{
				IsCommonTerminal: output.IsCommonTerminal,
			},
		},
	}
}

func toIsResourceCommonTerminalErrorResponse(
	err error,
) *providerserverv1.IsResourceCommonTerminalResponse {
	return &providerserverv1.IsResourceCommonTerminalResponse{
		Response: &providerserverv1.IsResourceCommonTerminalResponse_ErrorResponse{
			ErrorResponse: errorsv1.CreateResponseFromError(err),
		},
	}
}

func toResourceTypeDescriptionResponse(
	output *provider.ResourceGetTypeDescriptionOutput,
) *sharedtypesv1.TypeDescriptionResponse {
	if output == nil {
		return &sharedtypesv1.TypeDescriptionResponse{
			Response: &sharedtypesv1.TypeDescriptionResponse_ErrorResponse{
				ErrorResponse: sharedtypesv1.NoResponsePBError(),
			},
		}
	}

	return &sharedtypesv1.TypeDescriptionResponse{
		Response: &sharedtypesv1.TypeDescriptionResponse_Description{
			Description: &sharedtypesv1.TypeDescription{
				PlainTextDescription: output.PlainTextDescription,
				MarkdownDescription:  output.MarkdownDescription,
			},
		},
	}
}

func toResourceTypeDescriptionErrorResponse(
	err error,
) *sharedtypesv1.TypeDescriptionResponse {
	return &sharedtypesv1.TypeDescriptionResponse{
		Response: &sharedtypesv1.TypeDescriptionResponse_ErrorResponse{
			ErrorResponse: errorsv1.CreateResponseFromError(err),
		},
	}
}

func toResourceExternalStateResponse(
	output *provider.ResourceGetExternalStateOutput,
) (*providerserverv1.GetResourceExternalStateResponse, error) {
	if output == nil {
		return &providerserverv1.GetResourceExternalStateResponse{
			Response: &providerserverv1.GetResourceExternalStateResponse_ErrorResponse{
				ErrorResponse: sharedtypesv1.NoResponsePBError(),
			},
		}, nil
	}

	externalState, err := serialisation.ToMappingNodePB(
		output.ResourceSpecState,
		/* optional */ true,
	)
	if err != nil {
		return nil, err
	}

	return &providerserverv1.GetResourceExternalStateResponse{
		Response: &providerserverv1.GetResourceExternalStateResponse_ResourceSpecState{
			ResourceSpecState: externalState,
		},
	}, nil
}

func toResourceExternalStateErrorResponse(
	err error,
) *providerserverv1.GetResourceExternalStateResponse {
	return &providerserverv1.GetResourceExternalStateResponse{
		Response: &providerserverv1.GetResourceExternalStateResponse_ErrorResponse{
			ErrorResponse: errorsv1.CreateResponseFromError(err),
		},
	}
}

func toPBResourceTypes(resourceTypes []string) []*sharedtypesv1.ResourceType {
	return commoncore.Map(
		resourceTypes,
		func(resourceType string, _ int) *sharedtypesv1.ResourceType {
			return &sharedtypesv1.ResourceType{
				Type: resourceType,
			}
		},
	)
}
