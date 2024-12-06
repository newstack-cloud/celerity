package container

import (
	"fmt"

	"github.com/two-hundred/celerity/libs/blueprint/core"
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
	// ErrorReasonCodeBlueprintCycleDetected
	// is provided when the reason for an error
	// during deployment or change staging is due to
	// a cyclic blueprint inclusion detected.
	ErrorReasonCodeBlueprintCycleDetected errors.ErrorReasonCode = "blueprint_cycle_detected"
	// ErrorReasonCodeMaxBlueprintDepthExceeded
	// is provided when the reason for an error
	// during deployment or change staging is due to
	// the maximum blueprint depth being exceeded.
	ErrorReasonCodeMaxBlueprintDepthExceeded errors.ErrorReasonCode = "max_blueprint_depth_exceeded"
	// ErrorReasonCodeChildBlueprintError
	// is provided when the reason for an error
	// during deployment or change staging is due to
	// an error in a child blueprint.
	// This is used to wrap errors that occur in child blueprints
	// that are not run errors.
	ErrorReasonCodeChildBlueprintError errors.ErrorReasonCode = "child_blueprint_error"
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

func errBlueprintCycleDetected(
	includeName string,
	instanceTreePath string,
	cyclicInstanceID string,
) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeBlueprintCycleDetected,
		Err: fmt.Errorf(
			"[include.%s]: cyclic blueprint inclusion detected, instance %q is an ancestor of the "+
				"current blueprint as shown in the instance tree path: %q",
			includeName,
			cyclicInstanceID,
			instanceTreePath,
		),
	}
}

func errMaxBlueprintDepthExceeded(
	instanceTreePath string,
	maxDepth int,
) error {
	return &errors.RunError{
		ReasonCode: ErrorReasonCodeMaxBlueprintDepthExceeded,
		Err: fmt.Errorf(
			"max blueprint depth exceeded, instance tree path: %q, "+
				"only %d levels of blueprint includes are allowed",
			instanceTreePath,
			maxDepth,
		),
	}
}

func wrapErrorForChildContext(
	err error,
	params core.BlueprintParams,
) error {
	includeTreePath := getIncludeTreePath(params, "")
	if includeTreePath == "" {
		return err
	}

	runErr, isRunErr := err.(*errors.RunError)
	if isRunErr {
		// Be sure not to wrap errors that already have a child blueprint path,
		// we want to surface the most precise location of the error.
		if runErr.ChildBlueprintPath != "" {
			return err
		}

		return &errors.RunError{
			ReasonCode:         runErr.ReasonCode,
			Err:                runErr.Err,
			ChildErrors:        runErr.ChildErrors,
			ChildBlueprintPath: includeTreePath,
		}
	}
	return &errors.RunError{
		ReasonCode:         ErrorReasonCodeChildBlueprintError,
		Err:                err,
		ChildBlueprintPath: includeTreePath,
	}
}
