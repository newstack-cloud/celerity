package includes

import (
	"fmt"

	"github.com/two-hundred/celerity/libs/blueprint/errors"
)

const (
	// ErrorReasonCodeInvalidPath is an error that is returned when the path
	// provided to the file system child resolver is invalid.
	ErrorReasonCodeInvalidPath errors.ErrorReasonCode = "invalid_path"
	// ErrorReasonCodeBlueprintNotFound is an error that is returned when the
	// blueprint file is not found in a child resolver.
	ErrorReasonCodeBlueprintNotFound errors.ErrorReasonCode = "blueprint_not_found"
	// ErrorReasonCodePermissions is an error that is returned when the
	// file system child resolver encounters a permission error.
	ErrorReasonCodePermissions errors.ErrorReasonCode = "permission_error"
)

func errInvalidPath(includeName string) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeInvalidPath,
		Err: fmt.Errorf(
			"[include.%s]: invalid path found, path value must be a string for the file system "+
				"child resolver, the provided value is either empty or not a string",
			includeName,
		),
	}
}

func errBlueprintNotFound(includeName string, path string) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeBlueprintNotFound,
		Err: fmt.Errorf(
			"[include.%s]: blueprint not found at path: %s",
			includeName,
			path,
		),
	}
}

func errPermissions(includeName string, path string, err error) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodePermissions,
		Err: fmt.Errorf(
			"[include.%s]: permission error encountered while reading blueprint at path: %s: %w",
			includeName,
			path,
			err,
		),
	}
}
