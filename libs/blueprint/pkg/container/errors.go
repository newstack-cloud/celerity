package container

import (
	"fmt"
	"strings"

	"github.com/freshwebio/celerity/libs/common/pkg/core"
)

type LoadError struct {
	ReasonCode  ErrorReasonCode
	Err         error
	ChildErrors []error
}

func (e *LoadError) Error() string {
	childErrCount := len(e.ChildErrors)
	if childErrCount == 0 {
		return fmt.Sprintf("blueprint load error: %s", e.Err.Error())
	}
	return fmt.Sprintf("blueprint load error (%d child errors): %s", childErrCount, e.Err.Error())
}

type ErrorReasonCode string

const (
	// ErrorReasonCodeInvalidSpecExtension is provided
	// when the reason for a blueprint spec load error
	// is due to an invalid specification file extension.
	ErrorReasonCodeInvalidSpecExtension ErrorReasonCode = "invalid_spec_ext"
	// ErrorReasonCodeInvalidResourceType is provided
	// when the reason for a blueprint spec load error
	// is due to an invalid resource type provided in one
	// of the resources in the spec.
	ErrorReasonCodeInvalidResourceType ErrorReasonCode = "invalid_resource_type"
	// ErrorReasonCodeMissingProvider is provided when the
	// reason for a blueprint spec load error is due to
	// a missing provider for one of the resources in
	// the spec.
	ErrorReasonCodeMissingProvider ErrorReasonCode = "missing_provider"
	// ErrorReasonCodeMissingResource is provided when the
	// reason for a blueprint spec load error is due to
	// the resource provider missing an implementation for the
	// resource type for one of the resources in the spec.
	ErrorReasonCodeMissingResource ErrorReasonCode = "missing_resource"
	// ErrorReasonCodeResourceValidationErrors is provided
	// when the reason for a blueprint spec load error is due to
	// a collection of errors for one or more resources in the spec.
	// This should be used for a wrapper error that holds more specific
	// errors which can be used for reporting useful information
	// about issues with the spec.
	ErrorReasonCodeResourceValidationErrors ErrorReasonCode = "resource_validation_errors"
	// ErrorReasonMissingTransformers is provided when the
	// reason for a blueprint spec load error is due to a spec referencing
	// transformers that aren't supported by the blueprint loader
	// used to parse the schema.
	ErrorReasonMissingTransformers ErrorReasonCode = "missing_transformers"
)

func errUnsupportedSpecFileExtension(filePath string) error {
	return &LoadError{
		ReasonCode: ErrorReasonCodeInvalidSpecExtension,
		Err:        fmt.Errorf("unsupported spec file extension in %s, only json and yaml are supported", filePath),
	}
}

func errInvalidResourceType(resourceType string) error {
	return &LoadError{
		ReasonCode: ErrorReasonCodeInvalidResourceType,
		Err:        fmt.Errorf("resource type format is invalid for %s, resource type must be of the form {provider}/*", resourceType),
	}
}

func errMissingProvider(providerKey string, resourceType string) error {
	return &LoadError{
		ReasonCode: ErrorReasonCodeMissingProvider,
		Err:        fmt.Errorf("missing provider %s for the resource type %s", providerKey, resourceType),
	}
}

func errMissingResource(providerKey string, resourceType string) error {
	return &LoadError{
		ReasonCode: ErrorReasonCodeMissingResource,
		Err:        fmt.Errorf("missing resource in provider %s for the resource type %s", providerKey, resourceType),
	}
}

func errResourceValidationError(errorMap map[string]error) error {
	errCount := len(errorMap)
	return &LoadError{
		ReasonCode:  ErrorReasonCodeResourceValidationErrors,
		Err:         fmt.Errorf("validation failed due to issues with %d resources in the spec", errCount),
		ChildErrors: core.MapToSlice(errorMap),
	}
}

func errTransformersMissingError(missingTransformers []string) error {
	return &LoadError{
		ReasonCode: ErrorReasonMissingTransformers,
		Err: fmt.Errorf(
			"the following transformers are missing in the blueprint loader: %s", strings.Join(missingTransformers, ", "),
		),
	}
}
