package errors

import "fmt"

type ErrorReasonCode string

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
	errorsLabel := deriveErrorsLabel(childErrCount)
	return fmt.Sprintf("blueprint load error (%d child %s): %s", childErrCount, errorsLabel, e.Err.Error())
}

func deriveErrorsLabel(errorCount int) string {
	if errorCount == 1 {
		return "error"
	}

	return "errors"
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
