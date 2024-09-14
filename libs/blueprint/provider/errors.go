package provider

import (
	"fmt"

	"github.com/two-hundred/celerity/libs/blueprint/errors"
)

const (
	// ErrorReasonCodeItemTypeProviderNotFound is provided when the
	// reason for a blueprint spec load error is due to
	// there being no provider for a specific resource or data source
	// type.
	ErrorReasonCodeItemTypeProviderNotFound errors.ErrorReasonCode = "provider_not_found"
	// ErrorReasonCodeProviderDataSourceTypeNotFound is provided when the
	// reason for a blueprint spec load error is due to
	// the data source provider missing an implementation for a
	// specific data source type.
	ErrorReasonCodeProviderDataSourceTypeNotFound errors.ErrorReasonCode = "data_source_type_not_found"
	// ErrorReasonCodeFunctionNotFound is provided when the
	// reason for a blueprint spec load error is due to
	// the function not being found in any of the configured providers.
	ErrorReasonCodeFunctionNotFound errors.ErrorReasonCode = "function_not_found"
	// ErrorReasonCodeProviderFunctionNotFound is provided when the
	// reason for a blueprint spec load error is due to
	// the function not being found in a specific provider.
	ErrorReasonCodeProviderFunctionNotFound errors.ErrorReasonCode = "provider_function_not_found"
	// ErrorReasonCodeFunctionAlreadyProvided is provided when the
	// reason for a blueprint spec load error is due to
	// the same function being provided by multiple providers.
	ErrorReasonCodeFunctionAlreadyProvided errors.ErrorReasonCode = "function_already_provided"
	// ErrorReasonCodeInvalidResourceSpecDefinition is provided when the
	// reason for a blueprint spec load error is due to
	// an unknown resource spec schema type being used in the schema definition.
	ErrorReasonCodeInvalidResourceSpecDefinition errors.ErrorReasonCode = "invalid_resource_spec_def"
	// ErrorReasonCodeCustomVariableTypeNotFound is provided when the
	// reason for a blueprint spec load error is due to
	// the custom variable type not being found in a specific provider.
	ErrorReasonCodeProviderCustomVariableTypeNotFound errors.ErrorReasonCode = "custom_variable_type_not_found"
)

func errDataSourceTypeProviderNotFound(
	providerNamespace string,
	dataSourceType string,
) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeItemTypeProviderNotFound,
		Err: fmt.Errorf(
			"run failed as the provider with namespace %q was not found for data source type %q",
			providerNamespace,
			dataSourceType,
		),
	}
}

func errProviderDataSourceTypeNotFound(
	dataSourceType string,
	providerNamespace string,
) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeProviderDataSourceTypeNotFound,
		Err: fmt.Errorf(
			"run failed as the provider with namespace %q does not have an implementation for data source type %q",
			providerNamespace,
			dataSourceType,
		),
	}
}

func errCustomVariableTypeProviderNotFound(
	providerNamespace string,
	customVariableType string,
) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeItemTypeProviderNotFound,
		Err: fmt.Errorf(
			"run failed as the provider with namespace %q was not found for custom variable type %q",
			providerNamespace,
			customVariableType,
		),
	}
}

func errProviderCustomVariableTypeNotFound(
	customVariableType string,
	providerNamespace string,
) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeProviderCustomVariableTypeNotFound,
		Err: fmt.Errorf(
			"run failed as the provider with namespace %q does not have an implementation for custom variable type %q",
			providerNamespace,
			customVariableType,
		),
	}
}

func errFunctionNotFound(
	functionName string,
) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeFunctionNotFound,
		Err: fmt.Errorf(
			"run failed as the function %q was not found in any of the configured providers",
			functionName,
		),
	}
}

func errFunctionNotFoundInProvider(
	functionName string,
	providerNamespace string,
) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeProviderFunctionNotFound,
		Err: fmt.Errorf(
			"run failed as the function %q implementation was not found in the provider with namespace %q,"+
				" despite the provider listing it as a function it implements",
			functionName,
			providerNamespace,
		),
	}
}

func errFunctionAlreadyProvided(
	functionName string,
	providedBy string,
	provider string,
) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeFunctionAlreadyProvided,
		Err: fmt.Errorf(
			"run failed as the function %q is already provided by the %q provider,"+
				" but is also implemented by the %q provider",
			functionName,
			providedBy,
			provider,
		),
	}
}

// ErrUnknownResourceDefSchemaType is returned when the schema definition for a resource type
// contains an unknown resource definition schema type.
func ErrUnknownResourceDefSchemaType(
	specType ResourceDefinitionsSchemaType,
	resourceType string,
) error {
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResourceSpecDefinition,
		Err: fmt.Errorf(
			"validation failed due to an unknown resource definitions schema type %q "+
				"used in the schema definition for resource type %q",
			specType,
			resourceType,
		),
	}
}
