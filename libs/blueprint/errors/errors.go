package errors

import (
	"fmt"
	"strings"
)

type ErrorReasonCode string

type LoadError struct {
	ReasonCode  ErrorReasonCode
	Err         error
	ChildErrors []error
	Line        *int
	Column      *int
}

func (e *LoadError) Error() string {
	childErrCount := len(e.ChildErrors)
	if childErrCount == 0 {
		return fmt.Sprintf("blueprint load error: %s", e.Err.Error())
	}
	errorsLabel := deriveErrorsLabel(childErrCount)
	return fmt.Sprintf("blueprint load error (%d child %s): %s", childErrCount, errorsLabel, e.Err.Error())
}

func deriveErrorsLabel(errorCount int) string {
	if errorCount == 1 {
		return "error"
	}

	return "errors"
}

type RunError struct {
	ReasonCode  ErrorReasonCode
	Err         error
	ChildErrors []error
	// ChildBlueprintPath is the path to the child blueprint that caused the error.
	// This should be in the following format:
	// "include.<childName>::include.<grandChildName>::..."
	// Rendered as "include.<childName> -> include.<grandChildName> -> ..."
	//
	// This is useful for distinguishing between errors that occur in the parent blueprint
	// and errors that occur in a child blueprint.
	ChildBlueprintPath string
}

func (e *RunError) Error() string {
	childBlueprintPathInfo := renderChildBlueprintPathInfo(e.ChildBlueprintPath)
	childErrCount := len(e.ChildErrors)
	if childErrCount == 0 {
		return fmt.Sprintf("run error%s: %s", childBlueprintPathInfo, e.Err.Error())
	}
	errorsLabel := deriveErrorsLabel(childErrCount)

	return fmt.Sprintf(
		"run error (%d child %s)%s: %s",
		childErrCount,
		errorsLabel,
		childBlueprintPathInfo,
		e.Err.Error(),
	)
}

func renderChildBlueprintPathInfo(childBlueprintPath string) string {
	if childBlueprintPath == "" {
		return ""
	}

	includes := strings.Split(childBlueprintPath, "::")
	displayPath := strings.Join(includes, " -> ")

	return fmt.Sprintf(" (child blueprint path: %s)", displayPath)
}

type SerialiseError struct {
	ReasonCode  ErrorReasonCode
	Err         error
	ChildErrors []error
}

func (e *SerialiseError) Error() string {
	childErrCount := len(e.ChildErrors)
	if childErrCount == 0 {
		return fmt.Sprintf("blueprint serialise error: %s", e.Err.Error())
	}
	errorsLabel := deriveErrorsLabel(childErrCount)
	return fmt.Sprintf("blueprint serialise error (%d child %s): %s", childErrCount, errorsLabel, e.Err.Error())
}

type ExpandedSerialiseError struct {
	ReasonCode  ErrorReasonCode
	Err         error
	ChildErrors []error
}

func (e *ExpandedSerialiseError) Error() string {
	childErrCount := len(e.ChildErrors)
	if childErrCount == 0 {
		return fmt.Sprintf("expanded blueprint serialise error: %s", e.Err.Error())
	}
	errorsLabel := deriveErrorsLabel(childErrCount)
	return fmt.Sprintf("expanded blueprint serialise error (%d child %s): %s", childErrCount, errorsLabel, e.Err.Error())
}
