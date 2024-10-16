package errors

import "fmt"

// BuildEngineError is an error type returned from a build engine
// instance for validation, build and deployment stages.
type BuildEngineError struct {
	Message string
}

func (e *BuildEngineError) Error() string {
	return fmt.Sprintf("build engine error: %s", e.Message)
}
