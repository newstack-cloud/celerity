package container

import (
	"fmt"

	"github.com/two-hundred/celerity/libs/blueprint/errors"
)

const (
	// ErrorReasonCodeMissingChildBlueprintPath
	// is provided when the reason for an error
	// during deployment or change staging is due to
	// a missing path to a child blueprint in an include.
	ErrorReasonCodeMissingChildBlueprintPath errors.ErrorReasonCode = "missing_child_blueprint_path"
	// ErrorReasonCodeEmptyChildBlueprintPath
	// is provided when the reason for an error
	// during deployment or change staging is due to
	// an empty path to a child blueprint in an include.
	ErrorReasonCodeEmptyChildBlueprintPath errors.ErrorReasonCode = "empty_child_blueprint_path"
)

func errMissingChildBlueprintPath(includeName string) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeMissingChildBlueprintPath,
		Err:        fmt.Errorf("[include.%s]: child blueprint path is missing for include", includeName),
	}
}

func errEmptyChildBlueprintPath(includeName string) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeEmptyChildBlueprintPath,
		Err:        fmt.Errorf("[include.%s]: child blueprint path is empty for include", includeName),
	}
}
