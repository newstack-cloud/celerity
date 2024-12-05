package core

import "fmt"

// Error represents an error due to an issue
// with a path to access a value from a MappingNode.
type MappingPathError struct {
	ReasonCode ErrorCoreReasonCode
	Path       string
	Err        error
}

const (
	// ErrInvalidMappingPath is used when a path to access a value
	// in a MappingNode is invalid.
	ErrInvalidMappingPath ErrorCoreReasonCode = "invalid_mapping_path"
)

func (e *MappingPathError) Error() string {
	errorSuffix := ""
	if e.Err != nil {
		errorSuffix = fmt.Sprintf(": %s", e.Err)
	}
	return fmt.Sprintf("invalid mapping node accessor path %q%s", e.Path, errorSuffix)
}

// InvalidMappingPathError is a helper function that creates a new error
// for when a path to access a value in a MappingNode is invalid.
func errInvalidMappingPath(path string, err error) error {
	return &MappingPathError{
		ReasonCode: ErrInvalidMappingPath,
		Path:       path,
		Err:        err,
	}
}
