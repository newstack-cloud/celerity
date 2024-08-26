package function

import (
	"fmt"
	"strings"
)

// FuncCallErrorCode is an enumeration of error codes that can be used to
// identify the type of error that occurred during the execution of a
// substitution function call.
type FuncCallErrorCode int

const (
	// FuncCallErrorCodeUnknown is an error code that indicates an unknown
	// error occurred during the execution of a substitution function call.
	// This is the default as 0 is the default value when an error code
	// is not set.
	FuncCallErrorCodeUnknown FuncCallErrorCode = iota

	// FuncCallErrorCodeInvalidArgumentType is an error code that indicates
	// that an argument passed to a function is of an invalid type.
	FuncCallErrorCodeInvalidArgumentType

	// FuncCallErrorCodeInvalidArgsOffset is an error code that indicates
	// that an invalid args offset was defined for a partially applied
	// function.
	FuncCallErrorCodeInvalidArgsOffset

	// FuncCallErrorCodeFunctionCall is an error code that indicates that an
	// error occurred during the execution of a function call.
	FuncCallErrorCodeFunctionCall

	// FuncCallErrorCodeInvalidInput is an error code that indicates that an
	// error occurred during the execution of a function call due to invalid
	// arguments. (e.g. an invalid JSON string passed into "jsondecode" or "fromjson")
	FuncCallErrorCodeInvalidInput

	// FuncCallErrorCodeInvalidReturnType is an error code that indicates
	// that the return type of a function is invalid.
	FuncCallErrorCodeInvalidReturnType

	// FuncCallErrorCodeSystem is an error code that indicates an error
	// with the function call system.
	FuncCallErrorCodeSystem

	// FuncCallErrorCodeFunctionNotFound is an error code that indicates
	// that the function to be called was not found in the registry.
	FuncCallErrorCodeFunctionNotFound
)

// FuncCallError is an error type that represents an error that occurred
// during the execution of a substitution function call.
// This must be used to wrap errors that occur during the execution of a
// substitution function call to allow the system to pass structured errors
// across process boundaries when inter-process plugins are used by a tool
// built on top of the framework.
type FuncCallError struct {
	Code      FuncCallErrorCode
	Message   string
	CallStack []*Call
}

func (f *FuncCallError) Error() string {
	errorCodeLabel := getErrorCodeLabel(f.Code)
	return fmt.Sprintf(
		"[%s]: %s\nCall stack:\n%s",
		errorCodeLabel,
		f.Message,
		formatCallStack(f.CallStack),
	)
}

// NewFuncCallError creates a new instance of a FuncCallError,
// this should be used to wrap errors that occur during the execution
// of a substitution function call and in the function execution system.
func NewFuncCallError(message string, code FuncCallErrorCode, callStack []*Call) error {
	return &FuncCallError{Message: message, Code: code, CallStack: callStack}
}

func formatCallStack(callStack []*Call) string {
	var sb strings.Builder
	for _, call := range callStack {
		lineInfo := ""
		if call.Location != nil {
			lineInfo = fmt.Sprintf(
				" (line %d, col %d)",
				call.Location.Line,
				call.Location.Column,
			)
		}
		fileInfo := ""
		if call.FilePath != "" {
			fileInfo = fmt.Sprintf("%s:\n", call.FilePath)
		}
		sb.WriteString(fmt.Sprintf("%s  %s%s\n", fileInfo, call.FunctionName, lineInfo))
	}
	return sb.String()
}

func getErrorCodeLabel(code FuncCallErrorCode) string {
	switch code {
	case FuncCallErrorCodeInvalidArgumentType:
		return "InvalidArgumentType"
	case FuncCallErrorCodeInvalidArgsOffset:
		return "InvalidArgsOffset"
	case FuncCallErrorCodeFunctionCall:
		return "FunctionCall"
	case FuncCallErrorCodeSystem:
		return "System"
	case FuncCallErrorCodeFunctionNotFound:
		return "FunctionNotFound"
	case FuncCallErrorCodeInvalidInput:
		return "InvalidInput"
	case FuncCallErrorCodeInvalidReturnType:
		return "InvalidReturnType"
	default:
		return "Unknown"
	}
}
