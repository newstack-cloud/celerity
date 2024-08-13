package core

import (
	"errors"

	"gopkg.in/yaml.v3"
)

// Error represents an error due to an issue
// with a core data type (mapping or scalar) in a blueprint.
type Error struct {
	ReasonCode ErrorCoreReasonCode
	Err        error
	// The line in the source blueprint file
	// where the error occurred.
	// This will be nil if the error is not related
	// to a specific line in the blueprint file
	// or the source format is JSON.
	SourceLine *int
	// The column on a line in the source blueprint file
	// where the error occurred.
	// This will be nil if the error is not related
	// to a specific line/column in the blueprint file
	// or the source format is JSON.
	SourceColumn *int
}

func (e *Error) Error() string {
	return e.Err.Error()
}

type ErrorCoreReasonCode string

const (
	// ErrorCoreReasonCodeInvalidMappingNode is provided
	// when a mapping node can not be parsed from a serialised blueprint.
	ErrorCoreReasonCodeInvalidMappingNode ErrorCoreReasonCode = "invalid_mapping_node"
	// ErrorCoreReasonCodeMissingMappingNode is provided
	// when a mapping node is missing a value in a serialised blueprint.
	ErrorCoreReasonCodeMissingMappingNode ErrorCoreReasonCode = "missing_mapping_node"
	// ErrorCoreReasonCodeMustBeScalar is an error that is returned
	// when a value that is expected to be a blueprint scalar is not a scalar value.
	ErrorCoreReasonCodeMustBeScalar ErrorCoreReasonCode = "must_be_scalar"
)

func errInvalidMappingNode(value *yaml.Node) error {
	innerError := errors.New("a blueprint mapping node must be a valid scalar, mapping or sequence")
	if value == nil {
		return &Error{
			ReasonCode: ErrorCoreReasonCodeInvalidMappingNode,
			Err:        innerError,
		}
	}

	return &Error{
		ReasonCode:   ErrorCoreReasonCodeInvalidMappingNode,
		Err:          innerError,
		SourceLine:   &value.Line,
		SourceColumn: &value.Column,
	}
}

func errMissingMappingNode(value *yaml.Node) error {
	innerError := errors.New("a blueprint mapping node must have a valid value set")
	if value == nil {
		return &Error{
			ReasonCode: ErrorCoreReasonCodeMissingMappingNode,
			Err:        innerError,
		}
	}

	return &Error{
		ReasonCode:   ErrorCoreReasonCodeMissingMappingNode,
		Err:          innerError,
		SourceLine:   &value.Line,
		SourceColumn: &value.Column,
	}
}

func errMustBeScalar(value *yaml.Node) error {
	innerError := errors.New("a blueprint scalar value must be a scalar (string, int, bool or float)")
	if value == nil {
		return &Error{
			ReasonCode: ErrorCoreReasonCodeMustBeScalar,
			Err:        innerError,
		}
	}

	return &Error{
		ReasonCode:   ErrorCoreReasonCodeMustBeScalar,
		Err:          innerError,
		SourceLine:   &value.Line,
		SourceColumn: &value.Column,
	}
}
