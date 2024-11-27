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
	// ErrorReasonCodeResourceTemplateLinkLengthMismatch
	// is provided when the reason for an error
	// during deployment or change staging is due to
	// a mismatch in the length of the resolved items
	// for linked resource templates.
	ErrorReasonCodeResourceTemplateLinkLengthMismatch errors.ErrorReasonCode = "resource_template_link_length_mismatch"
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

func errResourceTemplateLinkLengthMismatch(
	linkFrom string,
	linkTo string,
) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeResourceTemplateLinkLengthMismatch,
		Err: fmt.Errorf(
			"resource template %s has a link to resource template %s with a different input length, links between resource templates can only be made "+
				"when the resolved items list from the `each` property of both templates is of the same length",
			linkFrom,
			linkTo,
		),
	}
}
