package errors

import "fmt"

// DeployEngineError is an error type returned from a deploy engine
// instance for validation, build and deployment stages.
type DeployEngineError struct {
	Message string
}

func (e *DeployEngineError) Error() string {
	return fmt.Sprintf("deploy engine error: %s", e.Message)
}
