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
	// ErrorReasonCodeProviderResourceTypeNotFound is provided when the
	// reason for a blueprint spec load error is due to
	// the resource provider missing an implementation for a
	// specific resource type.
	ErrorReasonCodeProviderResourceTypeNotFound errors.ErrorReasonCode = "resource_type_not_found"
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
)

func errResourceTypeProviderNotFound(
	providerNamespace string,
	resourceType string,
) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeItemTypeProviderNotFound,
		Err: fmt.Errorf(
			"run failed as the provider with namespace %q was not found for resource type %q",
			providerNamespace,
			resourceType,
		),
	}
}

func errProviderResourceTypeNotFound(
	resourceType string,
	providerNamespace string,
) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeProviderResourceTypeNotFound,
		Err: fmt.Errorf(
			"run failed as the provider with namespace %q does not have an implementation for resource type %q",
			providerNamespace,
			resourceType,
		),
	}
}

func errDataSourceTypeProviderNotFound(
	providerNamespace string,
	resourceType string,
) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeItemTypeProviderNotFound,
		Err: fmt.Errorf(
			"run failed as the provider with namespace %q was not found for data source type %q",
			providerNamespace,
			resourceType,
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

// ErrUnknownResourceSpecType is returned when the schema definition for a resource type
// contains an unknown resource spec schema type.
func ErrUnknownResourceSpecType(
	specType ResourceSpecSchemaType,
	resourceType string,
) error {
	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidResourceSpecDefinition,
		Err: fmt.Errorf(
			"validation failed due to an unknown resource spec schema type %q "+
				"used in the schema definition for resource type %q",
			specType,
			resourceType,
		),
	}
}
